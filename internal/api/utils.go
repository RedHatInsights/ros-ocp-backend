package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

func MapQueryParameters(c echo.Context) map[string]interface{} {
	log := logging.GetLogger()
	queryParams := make(map[string]interface{})

	now := time.Now().UTC()
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

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
	queryParams["DATE(recommendation_sets.monitoring_start_time) >= ?"] = startDate.Format("2006-01-02")

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
	queryParams["DATE(recommendation_sets.monitoring_end_time) <= ?"] = endDate.Format("2006-01-02")

	cluster := c.QueryParam("cluster")
	if cluster != "" {
		queryParams["clusters.cluster_alias LIKE ?"] = "%" + cluster + "%"
	}

	project := c.QueryParam("project")
	if project != "" {
		queryParams["workloads.namespace LIKE ?"] = "%" + project + "%"
	}

	workloadType := c.QueryParam("workload_type")
	if workloadType != "" {
		queryParams["workloads.workload_type = ?"] = workloadType
	}

	workloadName := c.QueryParam("workload")
	if workloadName != "" {
		queryParams["workloads.workload_name LIKE ?"] = "%" + workloadName + "%"
	}

	container := c.QueryParam("container")
	if container != "" {
		queryParams["recommendation_sets.container_name LIKE ?"] = "%" + container + "%"
	}

	return queryParams

}
