package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gorm.io/datatypes"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/kruizePayload"
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
	var clusters, projects, workloadNames, workloadTypes, containers []string

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

	clusters = c.QueryParams()["cluster"]
	if len(clusters) > 0 {
		paramString, values := parseQueryParams("cluster", clusters)
		queryParams[paramString] = values
	}

	projects = c.QueryParams()["project"]
	if len(projects) > 0 {
		paramString, values := parseQueryParams("project", projects)
		queryParams[paramString] = values
	}

	workloadNames = c.QueryParams()["workload"]
	if len(workloadNames) > 0 {
		paramString, values := parseQueryParams("workload", workloadNames)
		queryParams[paramString] = values
	}

	workloadTypes = c.QueryParams()["workload_type"]
	if len(workloadTypes) > 0 {
		paramString, values := parseQueryParams("workload_type", workloadTypes)
		queryParams[paramString] = values
	}

	containers = c.QueryParams()["container"]
	if len(containers) > 0 {
		paramString, values := parseQueryParams("container", containers)
		queryParams[paramString] = values
	}

	return queryParams, nil
}

func parseQueryParams(param string, values []string) (string, []string) {
	parsedKeyMultipleVal := ""
	valuesSlice := []string{}

	paramMap := map[string]string{
		"cluster":       "clusters.cluster_alias ILIKE ?",
		"workload_type": "workloads.workload_type = ?",
		"workload":      "workloads.workload_name ILIKE ?",
		"project":       "workloads.namespace ILIKE ?",
		"container":     "recommendation_sets.container_name ILIKE ?",
	}

	if len(values) > 1 {
		for _, value := range values {
			if param == "cluster" {
				parsedKeyMultipleVal = parsedKeyMultipleVal + paramMap[param] + " OR " + "clusters.cluster_uuid ILIKE ?" + " OR "
				valuesSlice = append(valuesSlice, "%"+value+"%")
				valuesSlice = append(valuesSlice, "%"+value+"%")
			} else {
				parsedKeyMultipleVal = parsedKeyMultipleVal + paramMap[param] + " OR "
				if param == "workload_type" {
					valuesSlice = append(valuesSlice, value)
				} else {
					valuesSlice = append(valuesSlice, "%"+value+"%")
				}
			}
		}
		parsedKeyMultipleVal = strings.TrimSuffix(parsedKeyMultipleVal, " OR ")
		return parsedKeyMultipleVal, valuesSlice
	} else {
		switch param {
		case "cluster":
			paramMap[param] = paramMap[param] + " OR " + "clusters.cluster_uuid ILIKE ?"
			valuesSlice = append(valuesSlice, "%"+values[0]+"%")
			valuesSlice = append(valuesSlice, "%"+values[0]+"%")
		case "workload_type":
			valuesSlice = append(valuesSlice, values[0])
		default:
			valuesSlice = append(valuesSlice, "%"+values[0]+"%")
		}
		return paramMap[param], valuesSlice
	}
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

func hasMoreThanThreeDecimals(value float64) bool {
	const decimalPrecision int = 3
	str := strconv.FormatFloat(value, 'f', -1, 64)
	decimalPart := strings.Split(str, ".")
	return (len(decimalPart) > 1) && (len(decimalPart[1]) > decimalPrecision)
}

func truncateToThreeDecimalPlaces(value float64) float64 {
	if hasMoreThanThreeDecimals(value) {
		truncated := math.Trunc(value * 1000) // Pushes decimal by 3 places and then truncates
		return truncated / 1000
	}
	return value
}

func convertCPUUnit(cpuUnit string, cpuValue float64) float64 {
	var convertedValueCPU float64

	switch cpuUnit {
	case "millicores":
		convertedValueCPU = math.Round(cpuValue * 1000) // millicore values don't require decimal precision
	case "cores":
		convertedValueCPU = truncateToThreeDecimalPlaces(cpuValue)
	default:
		convertedValueCPU = cpuValue
	}

	return convertedValueCPU
}

func convertMemoryUnit(memoryUnit string, memoryValue float64) float64 {
	var convertedValueMemory float64

	switch memoryUnit {
	case "MiB":
		memoryInMiB := memoryValue / 1024 / 1024
		convertedValueMemory = math.Trunc(memoryInMiB*100) / 100
	case "GiB":
		memoryInGiB := memoryValue / 1024 / 1024 / 1024
		convertedValueMemory = math.Trunc(memoryInGiB*100) / 100
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

func calculatePercentage(numerator float64, denominator float64) float64 {
	if numerator == 0.0 || denominator == 0.0 {
		// This block avoids below conditions and returns 0.0 instead,
		// When numerator is 0.0 the Go returns 0.0, valid number however division can be skipped
		// When denominator is 0.0 the Go returns Infinity(+Inf)
		// When both numerator and denominator are 0.0 the Go returns Not A Number(NaN)
		return 0.0
	}
	result := (numerator / denominator) * 100
	return result
}

func convertVariationToPercentage(recommendationJSON map[string]interface{}) map[string]interface{} {
	var currentCpuLimits, currentMemoryLimits, currentCpuRequests, currentMemoryRequests float64
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

					for _, section := range []string{"limits", "requests"} {
						sectionObject, ok := recommendationSection[section].(map[string]interface{})
						if ok {
							memoryObject, ok := sectionObject["memory"].(map[string]interface{})
							if ok {
								if memoryValue, ok := memoryObject["amount"].(float64); ok {
									switch section {
									case "limits":
										percentageMemoryValue := calculatePercentage(memoryValue, currentMemoryLimits)
										memoryObject["amount"] = truncateToThreeDecimalPlaces(percentageMemoryValue)
									case "requests":
										percentageMemoryValue := calculatePercentage(memoryValue, currentMemoryRequests)
										memoryObject["amount"] = truncateToThreeDecimalPlaces(percentageMemoryValue)
									}
									memoryObject["format"] = "percent"
								}
							}

							cpuObject, ok := sectionObject["cpu"].(map[string]interface{})
							if ok {
								if cpuValue, ok := cpuObject["amount"].(float64); ok {
									switch section {
									case "limits":
										percentageCpuValue := calculatePercentage(cpuValue, currentCpuLimits)
										cpuObject["amount"] = truncateToThreeDecimalPlaces(percentageCpuValue)
									case "requests":
										percentageCpuValue := calculatePercentage(cpuValue, currentCpuRequests)
										cpuObject["amount"] = truncateToThreeDecimalPlaces(percentageCpuValue)
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

func UpdateRecommendationJSON(handlerName string, recommendationID string, clusterUUID string, unitsToTransform map[string]string, updateUnitsk8s bool, jsonData datatypes.JSON) map[string]interface{} {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(jsonData), &data)
	if err != nil {
		log.Error("unable to unmarshall recommendation json")
		return nil
	}

	// box-plots data is not required on the list endpoint
	if handlerName == "recommendationset-list" {
		data = dropBoxPlotsObject(data)
	}

	data = transformComponentUnits(unitsToTransform, updateUnitsk8s, data) // cpu: core values require truncation
	data = filterNotifications(recommendationID, clusterUUID, data)
	data = convertVariationToPercentage(data)
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

	recommendationTermMap := map[string]kruizePayload.RecommendationTerm{
		"short_term":  recommendationObj.RecommendationTerms.Short_term,
		"medium_term": recommendationObj.RecommendationTerms.Medium_term,
		"long_term":   recommendationObj.RecommendationTerms.Long_term,
	}

	for termName, recommendationTerm := range recommendationTermMap {
		if recommendationTerm.RecommendationEngines == nil {
			continue
		}
		recommendationEngineMap := map[string]kruizePayload.RecommendationEngineObject{
			"cost":        recommendationTerm.RecommendationEngines.Cost,
			"performance": recommendationTerm.RecommendationEngines.Performance,
		}
		for recommendationType, recommendationEngine := range recommendationEngineMap {

			if _, objExists := recommendationEngineMap[recommendationType]; !objExists {
				continue
			}

			variationCPULimitPercentage := truncateToThreeDecimalPlaces(
				calculatePercentage(
					truncateToThreeDecimalPlaces(recommendationEngine.Variation.Limits.Cpu.Amount),
					truncateToThreeDecimalPlaces(recommendationObj.Current.Limits.Cpu.Amount),
				))

			variationMemoryLimitPercentage := truncateToThreeDecimalPlaces(
				calculatePercentage(
					recommendationEngine.Variation.Limits.Memory.Amount,
					recommendationObj.Current.Limits.Memory.Amount,
				))

			variationCPURequestPercentage := truncateToThreeDecimalPlaces(
				calculatePercentage(
					truncateToThreeDecimalPlaces(recommendationEngine.Variation.Requests.Cpu.Amount),
					truncateToThreeDecimalPlaces(recommendationObj.Current.Requests.Cpu.Amount),
				))

			variationMemoryRequestPercentage := truncateToThreeDecimalPlaces(
				calculatePercentage(
					recommendationEngine.Variation.Requests.Memory.Amount,
					recommendationObj.Current.Requests.Memory.Amount,
				))
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

func resolveResponseFormat(acceptHeaderVal string, formatQueryParamVal string) (string, error) {
	if acceptHeaderVal == "" && formatQueryParamVal == "" {
		return "json", nil // default format
	}

	responseFormat := ""
	switch acceptHeaderVal {
	case "text/csv":
		responseFormat = "csv"
	case "application/json":
		responseFormat = "json"
	}

	if responseFormat != "" {
		return responseFormat, nil // preferring header value
	} else {
		switch formatQueryParamVal {
		case "", "json":
			responseFormat = "json"
		case "csv":
			responseFormat = "csv"
		default:
			return "", fmt.Errorf("invalid value for format: %q", formatQueryParamVal)
		}
	}

	return responseFormat, nil
}
