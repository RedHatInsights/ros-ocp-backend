package api

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"gorm.io/datatypes"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
)

func CollectionResponse(collection []interface{}, req *http.Request, count, limit, offset int) *Collection {
	var first, previous, next, last string
	q := req.URL.Query()

	// set the "first" link with same limit+offset (what they requested)
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	params, _ := url.PathUnescape(q.Encode())
	first = fmt.Sprintf("%v?%v", req.URL.Path, params)

	// set the "last" link with limit+offset set for the next page
	q.Set("offset", strconv.Itoa(offset+limit))
	params, _ = url.PathUnescape(q.Encode())
	last = fmt.Sprintf("%v?%v", req.URL.Path, params)

	// set the "previous" link with limit-offset set for the previous page
	if offset > limit {
		q.Set("offset", strconv.Itoa(offset-limit))
		params, _ = url.PathUnescape(q.Encode())
		previous = fmt.Sprintf("%v?%v", req.URL.Path, params)
	}

	// set the "next" link with limit+offset set for the next page
	if offset+limit < count {
		q.Set("offset", strconv.Itoa(offset+limit))
		params, _ = url.PathUnescape(q.Encode())
		next = fmt.Sprintf("%v?%v", req.URL.Path, params)
	}

	// set offset based on limit size aka page size
	links := Links{
		First:    first,
		Previous: previous,
		Next:     next,
		Last:     last,
	}

	return &Collection{
		Data: collection,
		Meta: Metadata{
			Count:  count,
			Limit:  limit,
			Offset: offset,
		},
		Links: links,
	}
}

func MapQueryParameters(c echo.Context) (map[string]interface{}, error) {
	log := logging.GetLogger()
	queryParams := make(map[string]interface{})
	var startTimestamp, endTimestamp time.Time

	now := time.Now().UTC().Truncate(time.Second)
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	startDateStr := c.QueryParam("start_date")

	if startDateStr == "" {
		startTimestamp = firstOfMonth
	} else {
		var err error
		startTimestamp, err = time.Parse(timeLayout, startDateStr)
		if err != nil {
			log.Error("error parsing start_date:", err)
			return queryParams, err
		}
	}
	queryParams["recommendation_sets.monitoring_end_time >= ?"] = startTimestamp

	endDateStr := c.QueryParam("end_date")
	if endDateStr == "" {
		endTimestamp = now
	} else {
		var err error
		endTimestamp, err = time.Parse(timeLayout, endDateStr)
		if err != nil {
			log.Error("error parsing end_date:", err)
			return queryParams, err
		}
		// Inclusive user-provided end_date timestamp
		endTimestamp = endTimestamp.Add(24 * time.Hour)
	}
	queryParams["recommendation_sets.monitoring_end_time < ?"] = endTimestamp

	var errs []error
	if err := applyParamFilter(c, queryParams, "cluster", "", model.ClusterMaxLen, true); err != nil {
		errs = append(errs, err)
	}
	if err := applyParamFilter(c, queryParams, "project", "workloads.namespace", model.NamespaceMaxLen, false); err != nil {
		errs = append(errs, err)
	}
	if err := applyParamFilter(c, queryParams, "workload", "workloads.workload_name", model.ClusterMaxLen, true); err != nil {
		errs = append(errs, err)
	}
	workloadTypeVals := slices.Concat(
		c.QueryParams()["workload_type"],
		c.QueryParams()["filter[exact:workload_type]"],
		c.QueryParams()["exclude[workload_type]"],
	)
	if err := validateWorkloadTypeValues(workloadTypeVals); err != nil {
		errs = append(errs, err)
	} else if err := applyParamFilter(c, queryParams, "workload_type", "workloads.workload_type", model.NamespaceMaxLen, false, true); err != nil {
		errs = append(errs, err)
	}
	if err := applyParamFilter(c, queryParams, "container", "recommendation_sets.container_name", model.NamespaceMaxLen, false); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return queryParams, errors.Join(errs...)
	}

	return queryParams, nil
}

func ParseUnitParams(c echo.Context, defaultCPU, defaultMemory string) (map[string]string, bool, error) {
	unitChoices := make(map[string]string)

	cpuUnitParam := c.QueryParam("cpu-unit")
	cpuUnitOptions := map[string]bool{
		"millicores": true,
		"cores":      true,
	}

	if cpuUnitParam != "" {
		if !cpuUnitOptions[cpuUnitParam] {
			return nil, false, fmt.Errorf("invalid cpu unit")
		}
		unitChoices["cpu"] = cpuUnitParam
	} else {
		unitChoices["cpu"] = defaultCPU
	}

	memoryUnitParam := c.QueryParam("memory-unit")
	memoryUnitOptions := map[string]bool{
		"bytes": true,
		"MiB":   true,
		"GiB":   true,
	}

	if memoryUnitParam != "" {
		if !memoryUnitOptions[memoryUnitParam] {
			return nil, false, fmt.Errorf("invalid memory unit")
		}
		unitChoices["memory"] = memoryUnitParam
	} else {
		unitChoices["memory"] = defaultMemory
	}

	trueUnitsStr := c.QueryParam("true-units")
	var trueUnits bool
	if trueUnitsStr != "" {
		var err error
		trueUnits, err = strconv.ParseBool(trueUnitsStr)
		if err != nil {
			return nil, false, fmt.Errorf("invalid value for true-units")
		}
	}

	return unitChoices, !trueUnits, nil
}

// isCharSafeRFC1123 returns true for chars valid in RFC 1123 DNS labels/subdomains, plus underscore.
// allowDot: true for subdomains (cluster alias), false for single labels (namespace).
// Additionally, isCharSafeRFC1123 aims to provide necessary defense from SQL injection attacks.
// Ref - https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
func isCharSafeRFC1123(c rune, allowDot bool) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '-', c == '_':
		return true
	case allowDot && c == '.':
		return true
	default:
		return false
	}
}

func sanitizeParamValue(paramName, s string, paramMaxLen int, allowDot bool) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", namespaceAPIErrf(true, "empty value for %s", paramName)
	}
	if len(s) > paramMaxLen {
		return "", namespaceAPIErrf(true, "%s exceeds max length %d", paramName, paramMaxLen)
	}
	for _, c := range s {
		if !isCharSafeRFC1123(c, allowDot) {
			return "", namespaceAPIErrf(true, "invalid character in %s value", paramName)
		}
	}
	return s, nil
}

func parseClusterParams(value string, mode string) ([]string, []string, error) {
	if value == "" {
		return nil, nil, nil
	}
	modeClause := FilterModeClause[mode]
	if modeClause.Suffix == "" {
		return nil, nil, namespaceAPIErrf(false, "unknown cluster filter mode: %s", mode)
	}
	if _, err := uuid.Parse(value); err == nil {
		suffix := modeClause.Suffix
		// for cluster_uuid exact is set for includes
		if mode == FilterModeInclude {
			suffix = FilterModeClause[FilterModeExact].Suffix
		}
		return []string{"clusters.cluster_uuid" + suffix}, []string{value}, nil
	}
	s := value
	if modeClause.Wrap {
		s = "%" + s + "%"
	}
	return []string{"clusters.cluster_alias" + modeClause.Suffix}, []string{s}, nil
}

func buildModeClause(param, column, mode string, vals []string, maxLen int, allowDot bool) (map[string]any, error) {
	if len(vals) == 0 {
		return nil, nil
	}
	modeClause := FilterModeClause[mode]
	if modeClause.Suffix == "" {
		return nil, namespaceAPIErrf(false, "unknown filter mode: %s", mode)
	}

	allSQLClauses := make([]string, 0, len(vals))
	allParamVals := make([]string, 0, len(vals))
	for _, val := range vals {
		if val == "" {
			continue
		}
		switch param {
		case "cluster":
			sqlClauses, paramVals, err := parseClusterParams(val, mode)
			if err != nil {
				return nil, err
			}
			allSQLClauses = append(allSQLClauses, sqlClauses...)
			allParamVals = append(allParamVals, paramVals...)
		default:
			// handles all other string based query params
			s, err := sanitizeParamValue(param, val, maxLen, allowDot)
			if err != nil {
				return nil, err
			}
			if modeClause.Wrap {
				s = "%" + s + "%"
			}
			allParamVals = append(allParamVals, s)
			allSQLClauses = append(allSQLClauses, column+modeClause.Suffix)
		}
	}
	if len(allSQLClauses) == 0 {
		return nil, nil
	}
	joinedSQLClause := strings.Join(allSQLClauses, modeClause.Join)
	return map[string]any{joinedSQLClause: allParamVals}, nil
}

// parsing of string params based on mode -> include, exclude, exact.
func buildSQLClauseWithFilterType(param string, includeVals, exactVals, excludeVals []string, column string, maxLen int, allowDot bool) (map[string]any, error) {
	hasExclude, hasExact, hasInclude := len(excludeVals) > 0, len(exactVals) > 0, len(includeVals) > 0

	if !hasExclude && !hasExact {
		if !hasInclude {
			return nil, nil
		}
		// early exit as default is includes i.e. param=value
		return buildModeClause(param, column, FilterModeInclude, includeVals, maxLen, allowDot)
	}

	if hasExclude {
		for _, ev := range excludeVals {
			if slices.Contains(exactVals, ev) {
				return nil, namespaceAPIErrf(true, "exclude and exact cannot share values for %s", param)
			}
			if slices.Contains(includeVals, ev) {
				return nil, namespaceAPIErrf(true, "exclude and include cannot share values for %s", param)
			}
		}
	}

	clauseMap := make(map[string]any)
	if len(excludeVals) > 0 {
		clause, err := buildModeClause(param, column, FilterModeExclude, excludeVals, maxLen, allowDot)
		if err != nil {
			return nil, err
		}
		if clause != nil {
			maps.Copy(clauseMap, clause)
		}
	}
	if len(exactVals) > 0 {
		clause, err := buildModeClause(param, column, FilterModeExact, exactVals, maxLen, allowDot)
		if err != nil {
			return nil, err
		}
		if clause != nil {
			maps.Copy(clauseMap, clause)
		}
	}
	// exact is priority when present with includes for the same value
	var includeValsFiltered []string
	if hasExact && hasInclude {
		exactSet := make(map[string]bool)
		for _, v := range exactVals {
			exactSet[v] = true
		}
		for _, v := range includeVals {
			if !exactSet[v] {
				includeValsFiltered = append(includeValsFiltered, v)
			}
		}
	} else {
		includeValsFiltered = includeVals
	}
	if len(includeValsFiltered) > 0 {
		clause, err := buildModeClause(param, column, FilterModeInclude, includeValsFiltered, maxLen, allowDot)
		if err != nil {
			return nil, err
		}
		if clause != nil {
			maps.Copy(clauseMap, clause)
		}
	}
	return clauseMap, nil
}

func applyParamFilter(c echo.Context, queryParams map[string]any, param, column string, maxLen int, allowDot bool, treatIncludeAsExact ...bool) error {
	cfg := config.GetConfig()
	excludeKey := "exclude[" + param + "]"
	exactKey := "filter[exact:" + param + "]"
	useExactForInclude := len(treatIncludeAsExact) > 0 && treatIncludeAsExact[0]
	var includeVals, excludeVals, exactVals []string
	for _, v := range c.QueryParams()[param] {
		if v != "" {
			if useExactForInclude {
				exactVals = append(exactVals, v)
			} else {
				includeVals = append(includeVals, v)
			}
		}
	}

	if len(includeVals) > cfg.MaxCountPerQueryParam {
		return namespaceAPIErrf(true, "too many %s parameters, a maximum of %d is allowed", param, cfg.MaxCountPerQueryParam)
	}

	for _, v := range c.QueryParams()[excludeKey] {
		if v != "" {
			excludeVals = append(excludeVals, v)
		}
	}

	if len(excludeVals) > cfg.MaxCountPerQueryParam {
		return namespaceAPIErrf(true, "too many %s parameters, a maximum of %d is allowed", param, cfg.MaxCountPerQueryParam)
	}

	for _, v := range c.QueryParams()[exactKey] {
		if v != "" {
			exactVals = append(exactVals, v)
		}
	}

	if len(exactVals) > cfg.MaxCountPerQueryParam {
		return namespaceAPIErrf(true, "too many %s parameters, a maximum of %d is allowed", param, cfg.MaxCountPerQueryParam)
	}

	if len(includeVals) == 0 && len(excludeVals) == 0 && len(exactVals) == 0 {
		return nil
	}
	clauseMap, err := buildSQLClauseWithFilterType(param, includeVals, exactVals, excludeVals, column, maxLen, allowDot)
	if err != nil {
		return err
	}
	if clauseMap != nil {
		maps.Copy(queryParams, clauseMap)
	}
	return nil
}

func MapNamespaceQueryParameters(c echo.Context) (map[string]any, error) {
	log := logging.GetLogger()
	queryParams := make(map[string]any)
	var startTimestamp, endTimestamp time.Time

	now := time.Now().UTC().Truncate(time.Second)
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	startDateStr := c.QueryParam("start_date")
	if startDateStr == "" {
		startTimestamp = firstOfMonth
	} else {
		var err error
		startTimestamp, err = time.Parse(timeLayout, startDateStr)
		if err != nil {
			log.Error("error parsing start_date:", err)
			return queryParams, namespaceAPIErrf(true, "invalid start_date format, use YYYY-MM-DD")
		}
	}
	queryParams["namespace_recommendation_sets.monitoring_end_time >= ?"] = startTimestamp

	endDateStr := c.QueryParam("end_date")
	if endDateStr == "" {
		endTimestamp = now
	} else {
		var err error
		endTimestamp, err = time.Parse(timeLayout, endDateStr)
		if err != nil {
			log.Error("error parsing end_date:", err)
			return queryParams, namespaceAPIErrf(true, "invalid end_date format, use YYYY-MM-DD")
		}
		endTimestamp = endTimestamp.Add(24 * time.Hour)
	}
	queryParams["namespace_recommendation_sets.monitoring_end_time < ?"] = endTimestamp

	var errs []error
	if err := applyParamFilter(c, queryParams, "cluster", "", model.ClusterMaxLen, true); err != nil {
		errs = append(errs, err)
	}
	if err := applyParamFilter(c, queryParams, "project", "namespace_recommendation_sets.namespace_name", model.NamespaceMaxLen, false); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return queryParams, errors.Join(errs...)
	}

	return queryParams, nil
}

func get_user_permissions(c echo.Context) map[string][]string {
	var user_permissions map[string][]string
	switch t := c.Get("user.permissions").(type) {
	case map[string][]string:
		user_permissions = t
	default:
		user_permissions = map[string][]string{}
	}
	return user_permissions
}

func convertCPUUnit(cpuUnit string, cpuValue float64) float64 {
	var convertedValueCPU float64

	switch cpuUnit {
	case "millicores":
		convertedValueCPU = math.Round(cpuValue * 1000) // millicore values don't require decimal precision
	case "cores":
		convertedValueCPU = utils.TruncateToThreeDecimalPlaces(cpuValue)
	default:
		convertedValueCPU = cpuValue
	}

	return convertedValueCPU
}

func convertMemoryUnit(memoryUnit string, memoryValue float64) float64 {
	var convertedValueMemory float64

	switch memoryUnit {
	case "MiB":
		convertedValueMemory = utils.TruncateMemoryBytesToMiBTwoDecimals(memoryValue)
	case "GiB":
		convertedValueMemory = utils.TruncateMemoryBytesToGiBTwoDecimals(memoryValue)
	case "bytes":
		convertedValueMemory = memoryValue
	}

	return convertedValueMemory
}

func transformComponentUnits(unitsToTransform map[string]string, updateUnitsk8s bool, recommendationJSON map[string]interface{}) map[string]interface{} {
	/*
		Truncates CPU units(cores) to three decimal places
		Truncates Memory units(Mi) to two decimal places
		Hack: Truncates duration_in_hours to one decimal places
		TODO: Once Kruize returns identical values for duration_in_hours
		the ros-ocp should stop truncating the duration_in_hours
	*/

	truncateDurationInHours := func(intervalData map[string]interface{}) bool {
		durationInHours, ok := intervalData["duration_in_hours"].(float64)
		if ok {
			intervalData["duration_in_hours"] = math.Trunc(durationInHours*10) / 10
		}
		return ok
	}

	// Current section of recommendation
	current_config, ok := recommendationJSON["current"].(map[string]interface{})
	if !ok {
		log.Error("current not found in JSON")
	}

	for _, section := range []string{"limits", "requests"} {
		sectionObject, ok := current_config[section].(map[string]interface{})
		if ok {
			memoryObject, ok := sectionObject["memory"].(map[string]interface{})
			if ok {
				if memoryValue, ok := memoryObject["amount"].(float64); ok {
					memoryUnit := unitsToTransform["memory"]
					convertedMemoryValue := convertMemoryUnit(memoryUnit, memoryValue)
					memoryObject["amount"] = convertedMemoryValue
					if updateUnitsk8s {
						memoryObject["format"] = MemoryUnitk8s[memoryUnit]
					} else {
						memoryObject["format"] = memoryUnit
					}
				}
			}

			cpuObject, ok := sectionObject["cpu"].(map[string]interface{})
			if ok {
				if cpuValue, ok := cpuObject["amount"].(float64); ok {
					cpuUnit := unitsToTransform["cpu"]
					convertedCPUValue := convertCPUUnit(cpuUnit, cpuValue)
					cpuObject["amount"] = convertedCPUValue
					if updateUnitsk8s {
						cpuObject["format"] = CPUUnitk8s[cpuUnit]
					} else {
						cpuObject["format"] = cpuUnit
					}
				}
			}
		}
	}

	/*
		Recommendation data is available for three periods
		under cost and performance keys(engines)
		For each of these actual values will be present in
		below mentioned dataBlocks > request and limits
	*/

	// Recommendation section
	recommendation_terms, ok := recommendationJSON["recommendation_terms"].(map[string]interface{})
	if !ok {
		log.Error("recommendation data not found in JSON")
		return recommendationJSON
	}

	for _, period := range []string{"short_term", "medium_term", "long_term"} {
		intervalData, ok := recommendation_terms[period].(map[string]interface{})
		if !ok {
			continue
		}

		/* Hack
		// monitoring_start_time is currently not nullable on DB
		// Hence cannot be set to null while saving response from Kruize
		*/
		// remove nil equivalent monitoring_start_time in API response
		monitoring_start_time := intervalData["monitoring_start_time"]
		if monitoring_start_time == "0001-01-01T00:00:00Z" {
			delete(intervalData, "monitoring_start_time")
		}

		err := truncateDurationInHours(intervalData)
		if !err {
			log.Errorf("error truncating duration_in_hours in term %s\n", period)
		}

		if plotsObject, ok := intervalData["plots"].(map[string]interface{}); ok {
			if plotsDataObject, ok := plotsObject["plots_data"].(map[string]interface{}); ok {
				for _, value := range plotsDataObject {
					if datapointMap, ok := value.(map[string]interface{}); ok {
						if cpuUsage, ok := datapointMap["cpuUsage"].(map[string]interface{}); ok {
							cpuUnit := unitsToTransform["cpu"]
							if _, ok := cpuUsage["format"].(string); ok {
								if updateUnitsk8s {
									cpuUsage["format"] = CPUUnitk8s[cpuUnit]
								} else {
									cpuUsage["format"] = cpuUnit
								}
							}
							for _, key := range []string{"q1", "q3", "min", "max", "median"} {
								cpuValue, _ := cpuUsage[key].(float64)
								cpuUsage[key] = convertCPUUnit(cpuUnit, cpuValue)
							}
						}
						if memoryUsage, ok := datapointMap["memoryUsage"].(map[string]interface{}); ok {
							memoryUnit := unitsToTransform["memory"]
							if _, ok := memoryUsage["format"].(string); ok {
								if updateUnitsk8s {
									memoryUsage["format"] = MemoryUnitk8s[memoryUnit]
								} else {
									memoryUsage["format"] = memoryUnit
								}
							}
							for _, key := range []string{"q1", "q3", "min", "max", "median"} {
								memoryValue, _ := memoryUsage[key].(float64)
								memoryUsage[key] = convertMemoryUnit(memoryUnit, memoryValue)
							}
						}
					}
				}
			}
		}

		if intervalData["recommendation_engines"] != nil {
			for _, recommendationType := range []string{"cost", "performance"} {
				engineData, ok := intervalData["recommendation_engines"].(map[string]interface{})[recommendationType].(map[string]interface{})
				if !ok {
					continue
				}

				for _, dataBlock := range []string{"config", "variation"} {
					recommendationSection, ok := engineData[dataBlock].(map[string]interface{})
					if !ok {
						continue
					}

					for _, section := range []string{"limits", "requests"} {
						sectionObject, ok := recommendationSection[section].(map[string]interface{})
						if ok {
							memoryObject, ok := sectionObject["memory"].(map[string]interface{})
							if ok {
								if memoryValue, ok := memoryObject["amount"].(float64); ok {
									memoryUnit := unitsToTransform["memory"]
									convertedMemoryValue := convertMemoryUnit(memoryUnit, memoryValue)
									memoryObject["amount"] = convertedMemoryValue
									if updateUnitsk8s {
										memoryObject["format"] = MemoryUnitk8s[memoryUnit]
									} else {
										memoryObject["format"] = memoryUnit
									}
								}
							}

							cpuObject, ok := sectionObject["cpu"].(map[string]interface{})
							if ok {
								if cpuValue, ok := cpuObject["amount"].(float64); ok {
									cpuUnit := unitsToTransform["cpu"]
									convertedCPUValue := convertCPUUnit(cpuUnit, cpuValue)
									cpuObject["amount"] = convertedCPUValue
									if updateUnitsk8s {
										cpuObject["format"] = CPUUnitk8s[cpuUnit]
									} else {
										cpuObject["format"] = cpuUnit
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return recommendationJSON
}

func filterNotifications(recommendationID string, clusterUUID string, recommendationJSON map[string]interface{}) map[string]interface{} {
	var droppedNotifications []string

	deleteNotificationObject := func(recommendationSection map[string]interface{}) {
		notificationObject, ok := recommendationSection["notifications"].(map[string]interface{})
		if ok {
			for key := range notificationObject {
				_, found := NotificationsToShow[key]
				if !found {
					delete(recommendationSection, "notifications")
					droppedNotifications = append(droppedNotifications, key)
				}
			}
		}
	}

	// level 1 notifications are not stored in the database

	// level 2
	deleteNotificationObject(recommendationJSON)

	recommendationTerms, ok := recommendationJSON["recommendation_terms"].(map[string]interface{})
	if !ok {
		log.Error("recommendation data not found in JSON")
		return recommendationJSON
	}

	for _, term := range []string{"short_term", "medium_term", "long_term"} {
		levelThree, ok := recommendationTerms[term].(map[string]interface{})
		if ok {
			deleteNotificationObject(levelThree)
		}
		recommendationEngineObject, ok := levelThree["recommendation_engines"].(map[string]interface{})
		if ok {
			for _, engine := range []string{"cost", "performance"} {
				levelFour, ok := recommendationEngineObject[engine].(map[string]interface{})
				if ok {
					deleteNotificationObject(levelFour)
				}
			}
		}
	}
	droppedNotificationsString := strings.Join(droppedNotifications, ", ")
	log.Warnf("%s dropped from recommendation ID: %s; cluster ID: %s", droppedNotificationsString, recommendationID, clusterUUID)

	return recommendationJSON
}

func dropBoxPlotsObject(recommendationJSON map[string]interface{}) map[string]interface{} {
	recommendation_terms, ok := recommendationJSON["recommendation_terms"].(map[string]interface{})
	if !ok {
		log.Error("recommendation data not found in JSON")
		return recommendationJSON
	}

	for _, period := range []string{"short_term", "medium_term", "long_term"} {
		intervalData, ok := recommendation_terms[period].(map[string]interface{})
		if !ok {
			continue
		}
		delete(intervalData, "plots")
	}
	return recommendationJSON
}

// convertVariationToPercentage replaces variation amounts in the recommendation JSON with
// percentages relative to the corresponding current amounts. When skipRequests is true the
// "requests" section is left untouched (used when stored *_pct values have already been
// injected via injectStoredRequestVariationPct).
func convertVariationToPercentage(recommendationJSON map[string]interface{}, skipRequests bool) map[string]interface{} {
	var currentCpuLimits, currentMemoryLimits, currentCpuRequests, currentMemoryRequests float64

	current_config, ok := recommendationJSON["current"].(map[string]interface{})
	if !ok {
		log.Error("current not found in JSON")
	}

	for _, section := range []string{"limits", "requests"} {
		sectionObject, ok := current_config[section].(map[string]interface{})
		if ok {
			memoryObject, ok := sectionObject["memory"].(map[string]interface{})
			if ok {
				if memoryValue, ok := memoryObject["amount"].(float64); ok {
					switch section {
					case "limits":
						currentMemoryLimits = memoryValue
					case "requests":
						currentMemoryRequests = memoryValue
					}
				}
			}

			cpuObject, ok := sectionObject["cpu"].(map[string]interface{})
			if ok {
				if cpuValue, ok := cpuObject["amount"].(float64); ok {
					switch section {
					case "limits":
						currentCpuLimits = cpuValue
					case "requests":
						currentCpuRequests = cpuValue
					}
				}
			}
		}
	}

	recommendation_terms, ok := recommendationJSON["recommendation_terms"].(map[string]interface{})
	if !ok {
		log.Error("recommendation data not found in JSON")
		return recommendationJSON
	}

	for _, period := range []string{"short_term", "medium_term", "long_term"} {
		intervalData, ok := recommendation_terms[period].(map[string]interface{})
		if !ok {
			continue
		}

		if intervalData["recommendation_engines"] != nil {
			for _, recommendationType := range []string{"cost", "performance"} {
				engineData, ok := intervalData["recommendation_engines"].(map[string]interface{})[recommendationType].(map[string]interface{})
				if !ok {
					continue
				}

				for _, dataBlock := range []string{"variation"} {
					recommendationSection, ok := engineData[dataBlock].(map[string]interface{})
					if !ok {
						continue
					}

					sections := []string{"limits", "requests"}
					if skipRequests {
						sections = []string{"limits"}
					}

					for _, section := range sections {
						sectionObject, ok := recommendationSection[section].(map[string]interface{})
						if ok {
							memoryObject, ok := sectionObject["memory"].(map[string]interface{})
							if ok {
								if memoryValue, ok := memoryObject["amount"].(float64); ok {
									switch section {
									case "limits":
										percentageMemoryValue := utils.CalculatePercentage(memoryValue, currentMemoryLimits)
										memoryObject["amount"] = utils.TruncateToThreeDecimalPlaces(percentageMemoryValue)
									case "requests":
										percentageMemoryValue := utils.CalculatePercentage(memoryValue, currentMemoryRequests)
										memoryObject["amount"] = utils.TruncateToThreeDecimalPlaces(percentageMemoryValue)
									}
									memoryObject["format"] = "percent"
								}
							}

							cpuObject, ok := sectionObject["cpu"].(map[string]interface{})
							if ok {
								if cpuValue, ok := cpuObject["amount"].(float64); ok {
									switch section {
									case "limits":
										percentageCpuValue := utils.CalculatePercentage(cpuValue, currentCpuLimits)
										cpuObject["amount"] = utils.TruncateToThreeDecimalPlaces(percentageCpuValue)
									case "requests":
										percentageCpuValue := utils.CalculatePercentage(cpuValue, currentCpuRequests)
										cpuObject["amount"] = utils.TruncateToThreeDecimalPlaces(percentageCpuValue)
									}
									cpuObject["format"] = "percent"
								}
							}
						}
					}
				}
			}
		}
	}
	return recommendationJSON
}

// injectStoredRequestVariationPct writes the pre-computed *_pct DB column values directly into
// the variation.requests section of the recommendation JSON, replacing the raw amounts. This
// avoids recomputing percentages from the JSON blob for the requests section. The limits section
// is left unchanged and must still be processed by convertVariationToPercentage.
func injectStoredRequestVariationPct(data map[string]interface{}, pcts *model.StoredVariationPcts) map[string]interface{} {
	terms, ok := data["recommendation_terms"].(map[string]interface{})
	if !ok {
		return data
	}
	for _, period := range []string{"short_term", "medium_term", "long_term"} {
		intervalData, ok := terms[period].(map[string]interface{})
		if !ok {
			continue
		}
		engines, ok := intervalData["recommendation_engines"].(map[string]interface{})
		if !ok {
			continue
		}
		for _, engineName := range []string{"cost", "performance"} {
			engine, ok := engines[engineName].(map[string]interface{})
			if !ok {
				continue
			}
			variation, ok := engine["variation"].(map[string]interface{})
			if !ok {
				continue
			}
			requests, ok := variation["requests"].(map[string]interface{})
			if !ok {
				continue
			}
			cpuPct, memPct := pcts.Lookup(period, engineName)
			if cpu, ok := requests["cpu"].(map[string]interface{}); ok && cpuPct != nil {
				cpu["amount"] = *cpuPct
				cpu["format"] = "percent"
			}
			if mem, ok := requests["memory"].(map[string]interface{}); ok && memPct != nil {
				mem["amount"] = *memPct
				mem["format"] = "percent"
			}
		}
	}
	return data
}

// UpdateRecommendationJSON transforms raw recommendation JSON for API output: unit conversion,
// notification filtering, and variation-to-percentage conversion.
// When storedPcts is provided and has values, the requests variation percentages are taken
// directly from the stored DB columns instead of being recomputed from the JSON blob.
func UpdateRecommendationJSON(handlerName string, recommendationID string, clusterUUID string, unitsToTransform map[string]string, updateUnitsk8s bool, jsonData datatypes.JSON, storedPcts *model.StoredVariationPcts) map[string]interface{} {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(jsonData), &data)
	if err != nil {
		log.Error("unable to unmarshall recommendation json")
		return nil
	}

	// box-plots data is not required from list endpoints
	if handlerName == "recommendationset-list" || handlerName == "namespace-recommendationset-list" {
		data = dropBoxPlotsObject(data)
	}

	data = transformComponentUnits(unitsToTransform, updateUnitsk8s, data) // cpu: core values require truncation
	data = filterNotifications(recommendationID, clusterUUID, data)

	skipRequests := storedPcts != nil && storedPcts.HasValues()
	if skipRequests {
		data = injectStoredRequestVariationPct(data, storedPcts)
	}
	data = convertVariationToPercentage(data, skipRequests)
	return data
}

func formatPrecisionValuesToStr(val float64) string {
	// avoid un-necessary rounding by sprintf 104.939886 -> -104.940
	multiplier := 1000.0
	truncatedVal := math.Trunc(val*multiplier) / multiplier

	s := fmt.Sprintf("%.3f", truncatedVal)
	s = strings.TrimRight(s, "0")  // removes trailing zeros
	s = strings.TrimSuffix(s, ".") // removes trailing "." for instance 10.0
	return s
}

func GenerateCSVRows(recommendationSet model.RecommendationSetResult) ([][]string, error) {
	rows := [][]string{}
	variationFormat := "percent"
	var recommendationObj kruizePayload.RecommendationData
	err := json.Unmarshal([]byte(recommendationSet.Recommendations), &recommendationObj)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshall recommendation %s: %w", recommendationSet.ID, err)
	}

	type namedTerm struct {
		name string
		term kruizePayload.RecommendationTerm
	}
	orderedTerms := []namedTerm{
		{"short_term", recommendationObj.RecommendationTerms.Short_term},
		{"medium_term", recommendationObj.RecommendationTerms.Medium_term},
		{"long_term", recommendationObj.RecommendationTerms.Long_term},
	}

	type namedEngine struct {
		name   string
		engine kruizePayload.RecommendationEngineObject
	}

	for _, nt := range orderedTerms {
		termName := nt.name
		recommendationTerm := nt.term
		if recommendationTerm.RecommendationEngines == nil {
			continue
		}
		orderedEngines := []namedEngine{
			{"cost", recommendationTerm.RecommendationEngines.Cost},
			{"performance", recommendationTerm.RecommendationEngines.Performance},
		}
		for _, ne := range orderedEngines {
			recommendationType := ne.name
			recommendationEngine := ne.engine

			variationCPULimitPercentage := utils.VariationPercentOfRequestCPU(
				recommendationEngine.Variation.Limits.Cpu.Amount,
				recommendationObj.Current.Limits.Cpu.Amount,
			)

			variationMemoryLimitPercentage := utils.VariationPercentOfRequestMemoryBytesMiB(
				recommendationEngine.Variation.Limits.Memory.Amount,
				recommendationObj.Current.Limits.Memory.Amount,
			)

			variationCPURequestPercentage := utils.VariationPercentOfRequestCPU(
				recommendationEngine.Variation.Requests.Cpu.Amount,
				recommendationObj.Current.Requests.Cpu.Amount,
			)

			variationMemoryRequestPercentage := utils.VariationPercentOfRequestMemoryBytesMiB(
				recommendationEngine.Variation.Requests.Memory.Amount,
				recommendationObj.Current.Requests.Memory.Amount,
			)
			rows = append(rows, []string{
				recommendationSet.ID,
				recommendationSet.ClusterUUID,
				recommendationSet.ClusterAlias,
				recommendationSet.Container,
				recommendationSet.Project,
				recommendationSet.Workload,
				recommendationSet.WorkloadType,
				recommendationSet.LastReported,
				recommendationSet.SourceID,
				formatPrecisionValuesToStr(convertCPUUnit("cores", recommendationObj.Current.Limits.Cpu.Amount)),
				recommendationObj.Current.Limits.Cpu.Format,
				fmt.Sprint(recommendationObj.Current.Limits.Memory.Amount),
				recommendationObj.Current.Limits.Memory.Format,
				formatPrecisionValuesToStr(convertCPUUnit("cores", recommendationObj.Current.Requests.Cpu.Amount)),
				recommendationObj.Current.Requests.Cpu.Format,
				fmt.Sprint(recommendationObj.Current.Requests.Memory.Amount),
				recommendationObj.Current.Requests.Memory.Format,
				recommendationObj.MonitoringEndTime.String(),
				termName,
				fmt.Sprint(recommendationTerm.DurationInHours),
				recommendationTerm.MonitoringStartTime.String(),
				recommendationType,
				formatPrecisionValuesToStr(convertCPUUnit("cores", recommendationEngine.Config.Limits.Cpu.Amount)),
				recommendationEngine.Config.Limits.Cpu.Format,
				fmt.Sprint(recommendationEngine.Config.Limits.Memory.Amount),
				recommendationEngine.Config.Limits.Memory.Format,
				formatPrecisionValuesToStr(convertCPUUnit("cores", recommendationEngine.Config.Requests.Cpu.Amount)),
				recommendationEngine.Config.Requests.Cpu.Format,
				fmt.Sprint(recommendationEngine.Config.Requests.Memory.Amount),
				recommendationEngine.Config.Requests.Memory.Format,
				formatPrecisionValuesToStr(variationCPULimitPercentage),
				variationFormat,
				fmt.Sprint(variationMemoryLimitPercentage),
				variationFormat,
				formatPrecisionValuesToStr(variationCPURequestPercentage),
				variationFormat,
				fmt.Sprint(variationMemoryRequestPercentage),
				variationFormat,
			})
		}
	}
	return rows, nil
}

func GenerateAndStreamCSV(w io.Writer, recommendationSets []model.RecommendationSetResult) error {
	writer := csv.NewWriter(w)
	header := FlattenedCSVHeader

	if err := writer.Write(header); err != nil {
		return fmt.Errorf("unable to write header: %w", err)
	}

	for i := range recommendationSets {
		CSVRows, generateRowErr := GenerateCSVRows(recommendationSets[i])
		if generateRowErr != nil {
			return fmt.Errorf("unable to generate rows: %w", generateRowErr)
		}
		for _, row := range CSVRows {
			if err := writer.Write(row); err != nil {
				return fmt.Errorf("unable to write row: %w", err)
			}
		}

		if (i+1)%config.GetConfig().CSVStreamInterval == 0 { // flush every CSVStreamInterval db records
			writer.Flush()
			if err := writer.Error(); err != nil {
				return fmt.Errorf("periodic flush error at row %d: %w", i+1, err)
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush error: %w", err)
	}
	return nil
}
