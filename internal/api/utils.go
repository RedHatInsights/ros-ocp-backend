package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gorm.io/datatypes"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
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

func MapQueryParameters(c echo.Context) (map[string][]string, error) {
	log := logging.GetLogger()
	queryParams := make(map[string][]string)
	var startDate, endDate time.Time
	var clusters, projects, workloadNames, workloadTypes, containers []string

	now := time.Now().UTC()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	dateSlice := []string{}
	startDateStr := c.QueryParam("start_date")

	if startDateStr == "" {
		startDate = firstOfMonth
	} else {
		var err error
		startDate, err = time.Parse(timeLayout, startDateStr)
		if err != nil {
			log.Error("error parsing start_date:", err)
			return queryParams, err
		}
	}
	startDateSlice := append(dateSlice, startDate.Format(timeLayout))
	queryParams["DATE(recommendation_sets.monitoring_end_time) >= ?"] = startDateSlice
	endDateStr := c.QueryParam("end_date")

	if endDateStr == "" {
		endDate = now
	} else {
		var err error
		endDate, err = time.Parse(timeLayout, endDateStr)
		if err != nil {
			log.Error("error parsing end_date:", err)
			return queryParams, err
		}
	}
	endDateSlice := append(dateSlice, endDate.Format(timeLayout))

	queryParams["DATE(recommendation_sets.monitoring_end_time) <= ?"] = endDateSlice

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

	var paramMap = map[string]string{
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
		if param == "cluster" {
			paramMap[param] = paramMap[param] + " OR " + "clusters.cluster_uuid ILIKE ?"
			valuesSlice = append(valuesSlice, "%"+values[0]+"%")
			valuesSlice = append(valuesSlice, "%"+values[0]+"%")
		} else if param == "workload_type" {
			valuesSlice = append(valuesSlice, values[0])
		} else {
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

	if cpuUnit == "millicores" {
		convertedValueCPU = math.Round(cpuValue * 1000) // millicore values don't require decimal precision
	} else if cpuUnit == "cores" {
		convertedValueCPU = truncateToThreeDecimalPlaces(cpuValue)
	} else {
		convertedValueCPU = cpuValue
	}

	return convertedValueCPU

}

func convertMemoryUnit(memoryUnit string, memoryValue float64) float64 {

	var convertedValueMemory float64

	if memoryUnit == "MiB" {
		memoryInMiB := memoryValue / 1024 / 1024
		convertedValueMemory = math.Trunc(memoryInMiB*100) / 100
	} else if memoryUnit == "GiB" {
		memoryInGiB := memoryValue / 1024 / 1024 / 1024
		convertedValueMemory = math.Trunc(memoryInGiB*100) / 100
	} else if memoryUnit == "bytes" {
		convertedValueMemory = memoryValue
	}

	return convertedValueMemory

}

func transformComponentUnits(unitsToTransform map[string]string, recommendationJSON map[string]interface{}) map[string]interface{} {
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
					memoryObject["format"] = MemoryUnitk8s[memoryUnit]
				}
			}

			cpuObject, ok := sectionObject["cpu"].(map[string]interface{})
			if ok {
				if cpuValue, ok := cpuObject["amount"].(float64); ok {
					cpuUnit := unitsToTransform["cpu"]
					convertedCPUValue := convertCPUUnit(cpuUnit, cpuValue)
					cpuObject["amount"] = convertedCPUValue
					cpuObject["format"] = CPUUnitk8s[cpuUnit]
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
								cpuUsage["format"] = cpuUnit
							}
							for _, key := range []string{"q1", "q3", "min", "max", "median"} {
								cpuValue, _ := cpuUsage[key].(float64)
								cpuUsage[key] = convertCPUUnit(cpuUnit, cpuValue)
							}
						}
						if memoryUsage, ok := datapointMap["memoryUsage"].(map[string]interface{}); ok {
							memoryUnit := unitsToTransform["memory"]
							if _, ok := memoryUsage["format"].(string); ok {
								memoryUsage["format"] = memoryUnit
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
									memoryObject["format"] = MemoryUnitk8s[memoryUnit]
								}
							}

							cpuObject, ok := sectionObject["cpu"].(map[string]interface{})
							if ok {
								if cpuValue, ok := cpuObject["amount"].(float64); ok {
									cpuUnit := unitsToTransform["cpu"]
									convertedCPUValue := convertCPUUnit(cpuUnit, cpuValue)
									cpuObject["amount"] = convertedCPUValue
									cpuObject["format"] = CPUUnitk8s[cpuUnit]
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

func convertVariationToPercentage(recommendationJSON map[string]interface{}) map[string]interface{} {
	var currentCpuLimits, currentMemoryLimits, currentCpuRequests, currentMemoryRequests float64
	// Current section of recommendation

	calculatePercentage := func(numerator float64, denominator float64) float64 {
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
					if section == "limits" {
						currentMemoryLimits = memoryValue
					} else if section == "requests" {
						currentMemoryRequests = memoryValue
					}
				}
			}

			cpuObject, ok := sectionObject["cpu"].(map[string]interface{})
			if ok {
				if cpuValue, ok := cpuObject["amount"].(float64); ok {
					if section == "limits" {
						currentCpuLimits = cpuValue
					} else if section == "requests" {
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
									if section == "limits" {
										percentageMemoryValue := calculatePercentage(memoryValue, currentMemoryLimits)
										memoryObject["amount"] = truncateToThreeDecimalPlaces(percentageMemoryValue)
									} else if section == "requests" {
										percentageMemoryValue := calculatePercentage(memoryValue, currentMemoryRequests)
										memoryObject["amount"] = truncateToThreeDecimalPlaces(percentageMemoryValue)
									}
									memoryObject["format"] = "percent"
								}
							}

							cpuObject, ok := sectionObject["cpu"].(map[string]interface{})
							if ok {
								if cpuValue, ok := cpuObject["amount"].(float64); ok {
									if section == "limits" {
										percentageCpuValue := calculatePercentage(cpuValue, currentCpuLimits)
										cpuObject["amount"] = truncateToThreeDecimalPlaces(percentageCpuValue)
									} else if section == "requests" {
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

func UpdateRecommendationJSON(handlerName string, recommendationID string, clusterUUID string, unitsToTransform map[string]string, jsonData datatypes.JSON) map[string]interface{} {

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

	data = transformComponentUnits(unitsToTransform, data) // cpu: core values require truncation
	data = filterNotifications(recommendationID, clusterUUID, data)
	data = convertVariationToPercentage(data)
	return data
}
