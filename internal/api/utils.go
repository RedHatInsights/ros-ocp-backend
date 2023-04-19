package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
)

type Collection struct {
	Data  []interface{} `json:"data"`
	Meta  Metadata      `json:"meta"`
	Links Links         `json:"links"`
}

type Metadata struct {
	Count  int `json:"count"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type Links struct {
	First    string `json:"first"`
	Previous string `json:"previous,omitempty"`
	Next     string `json:"next,omitempty"`
	Last     string `json:"last"`
}

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

func MapQueryParameters(c echo.Context) map[string][]string {
	log := logging.GetLogger()
	queryParams := make(map[string][]string)

	now := time.Now().UTC()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	dateSlice := []string{}
	startDateStr := c.QueryParam("start_date")
	var startDate time.Time
	if startDateStr == "" {
		startDate = firstOfMonth
	} else {
		var err error
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			log.Error("error parsing start_date:", err)
		}
	}
	startDateSlice := append(dateSlice, startDate.Format("2006-01-02"))
	queryParams["DATE(recommendation_sets.monitoring_start_time) >= ?"] = startDateSlice

	endDateStr := c.QueryParam("end_date")
	var endDate time.Time
	if endDateStr == "" {
		endDate = now
	} else {
		var err error
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			log.Error("error parsing end_date:", err)
		}
	}
	endDateSlice := append(dateSlice, endDate.Format("2006-01-02"))

	queryParams["DATE(recommendation_sets.monitoring_end_time) <= ?"] = endDateSlice

	clusters := c.QueryParams()["cluster"]
	if len(clusters) > 0{
		paramString, values := parseQueryParams("cluster", clusters)
		queryParams[paramString] = values
	}

	projects := c.QueryParams()["project"]
	if len(projects) > 0{
		paramString, values := parseQueryParams("project", projects)
		queryParams[paramString] = values
	}

	workloadNames := c.QueryParams()["workload"]
	if len(workloadNames) > 0{
		paramString, values := parseQueryParams("workload", workloadNames)
		queryParams[paramString] = values
	}
	
	workloadTypes := c.QueryParams()["workload_type"]
	if len(workloadTypes) > 0{
		paramString, values := parseQueryParams("workload_type", workloadTypes)
		queryParams[paramString] = values
	}

	containers := c.QueryParams()["container"]
	if len(containers) > 0{
		paramString, values := parseQueryParams("container", containers)
		queryParams[paramString] = values
	}

	return queryParams

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

	if len(values) > 1{
		for _, value := range values {
			if param == "cluster"{
				parsedKeyMultipleVal = parsedKeyMultipleVal + paramMap[param] + " OR " + "clusters.cluster_uuid ILIKE ?" + " OR "
				valuesSlice = append(valuesSlice, "%" + value + "%")
				valuesSlice = append(valuesSlice, "%" + value + "%")
			} else {
				parsedKeyMultipleVal = parsedKeyMultipleVal + paramMap[param] + " OR "
				if param == "workload_type"{
					valuesSlice = append(valuesSlice, value)
				} else {
					valuesSlice = append(valuesSlice, "%" + value + "%")
				}
			}
		}
		parsedKeyMultipleVal = strings.TrimSuffix(parsedKeyMultipleVal, " OR ")
		return parsedKeyMultipleVal, valuesSlice
	} else {
		if param == "cluster"{
			paramMap[param] = paramMap[param] + " OR " + "clusters.cluster_uuid ILIKE ?"
			valuesSlice = append(valuesSlice, "%" + values[0] + "%")
			valuesSlice = append(valuesSlice, "%" + values[0] + "%")
		} else if param == "workload_type" {
			valuesSlice = append(valuesSlice, values[0])
		} else {
			valuesSlice = append(valuesSlice, "%" + values[0] + "%")
		}
		return paramMap[param], valuesSlice
	}

}
