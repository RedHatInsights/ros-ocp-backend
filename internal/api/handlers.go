package api

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
)

func GetRecommendationSetList(c echo.Context) error {
	XRHID := c.Get("Identity").(identity.XRHID)
	OrgID := XRHID.Identity.OrgID
	user_permissions := get_user_permissions(c)
	handlerName := "recommendationset-list"
	unitChoices := make(map[string]string)

	cpuUnitParam := c.QueryParam("cpu-unit")
	cpuUnitOptions := map[string]bool{
		"millicores": true,
		"cores":      true,
	}

	if cpuUnitParam != "" {
		if !cpuUnitOptions[cpuUnitParam] {
			return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "invalid cpu unit"})
		} else {
			unitChoices["cpu"] = cpuUnitParam
		}
	} else {
		unitChoices["cpu"] = "cores"
	}

	memoryUnitParam := c.QueryParam("memory-unit")
	memoryUnitOptions := map[string]bool{
		"bytes": true,
		"MiB":   true,
		"GiB":   true,
	}

	if memoryUnitParam != "" {
		if !memoryUnitOptions[memoryUnitParam] {
			return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "invalid memory unit"})
		} else {
			unitChoices["memory"] = memoryUnitParam
		}
	} else {
		unitChoices["memory"] = "bytes"
	}

	var orderHow string
	var orderBy string
	// Default values
	var limit int = 10
	var offset int = 0

	orderBy = c.QueryParam("order_by")
	if orderBy != "" {
		orderByOptions := map[string]string{
			"cluster":       "clusters.cluster_alias",
			"workload_type": "workloads.workload_type",
			"workload":      "workloads.workload_name",
			"project":       "workloads.namespace",
			"container":     "recommendation_sets.container_name",
			"last_reported": "clusters.last_reported_at",
		}
		orderByOption, keyError := orderByOptions[orderBy]

		if !keyError {
			return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "invalid order_by value"})
		}
		orderBy = orderByOption
	} else {
		orderBy = "clusters.last_reported_at"
	}

	orderHow = c.QueryParam("order_how")
	if orderHow != "" {
		orderHowUpper := strings.ToUpper(orderHow)
		if (orderHowUpper != "ASC") && (orderHowUpper != "DESC") {
			return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "invalid order_how value"})
		}
		orderHow = orderHowUpper
	} else {
		orderHow = "DESC"
	}

	orderQuery := orderBy + " " + orderHow

	limitStr := c.QueryParam("limit")
	if limitStr != "" {
		limitInt, err := strconv.Atoi(limitStr)
		if err == nil {
			limit = limitInt
		}
	}

	offsetStr := c.QueryParam("offset")

	if offsetStr != "" {
		offsetInt, err := strconv.Atoi(offsetStr)
		if err == nil {
			offset = offsetInt
		}
	}

	format := c.QueryParam("format")

	if format == "" {
		format = "json"
	} else {
		formatLower := strings.ToLower(format)
		switch formatLower {
		case "csv":
			format = "csv"
		case "json":
			format = "json"
		default:
			return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "invalid value for format"})
		}
	}

	queryParams, err := MapQueryParameters(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": err.Error()})
	}
	recommendationSet := model.RecommendationSet{}
	recommendationSets, count, queryErr := recommendationSet.GetRecommendationSets(OrgID, orderQuery, format, limit, offset, queryParams, user_permissions)
	if queryErr != nil {
		log.Errorf("unable to fetch records from database; %v", queryErr)
	}

	trueUnitsStr := c.QueryParam("true-units")
	var trueUnits bool

	if trueUnitsStr != "" {
		trueUnits, err = strconv.ParseBool(trueUnitsStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "invalid value for true-units"})
		}
	}
	setk8sUnits := !trueUnits

	for i := range recommendationSets {
		recommendationSets[i].RecommendationsJSON = UpdateRecommendationJSON(
			handlerName,
			recommendationSets[i].ID,
			recommendationSets[i].ClusterUUID,
			unitChoices,
			setk8sUnits,
			recommendationSets[i].Recommendations,
		)
	}

	switch format {
	case "json":
		interfaceSlice := make([]interface{}, len(recommendationSets))
		for i, v := range recommendationSets {
			interfaceSlice[i] = v
		}
		results := CollectionResponse(interfaceSlice, c.Request(), count, limit, offset)
		return c.JSON(http.StatusOK, results)
	case "csv":
		filename := "recommendations-" + time.Now().Format("20060102")
		c.Response().Header().Set(echo.HeaderContentType, "text/csv")
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", filename))
		pipeReader, pipeWriter := io.Pipe()

		go func() {
			var generationErr error
			defer func() {
				if r := recover(); r != nil {
					generationErr = fmt.Errorf("panic in CSV generation goroutine: %v", r)
				}
				if generationErr != nil {
					_ = pipeWriter.CloseWithError(generationErr)
					log.Errorf("error during CSV generation (recovered or returned): %v", generationErr)
				} else {
					_ = pipeWriter.Close() // graceful closure
				}
			}()
			generationErr = GenerateAndStreamCSV(pipeWriter, recommendationSets)
		}()
		return c.Stream(http.StatusOK, "text/csv", pipeReader)
	}
	return nil
}

func GetRecommendationSet(c echo.Context) error {
	XRHID := c.Get("Identity").(identity.XRHID)
	OrgID := XRHID.Identity.OrgID
	user_permissions := get_user_permissions(c)
	handlerName := "recommendationset"

	RecommendationIDStr := c.Param("recommendation-id")
	RecommendationUUID, err := uuid.Parse(RecommendationIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "bad recommendation_id"})
	}

	unitChoices := make(map[string]string)

	trueUnitsStr := c.QueryParam("true-units")
	var trueUnits bool

	if trueUnitsStr != "" {
		trueUnits, err = strconv.ParseBool(trueUnitsStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "invalid value for true-units"})
		}
	}
	setk8sUnits := !trueUnits

	cpuUnitParam := c.QueryParam("cpu-unit")
	cpuUnitOptions := map[string]bool{
		"millicores": true,
		"cores":      true,
	}

	if cpuUnitParam != "" {
		if !cpuUnitOptions[cpuUnitParam] {
			return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "invalid cpu unit"})
		} else {
			unitChoices["cpu"] = cpuUnitParam
		}
	} else {
		unitChoices["cpu"] = "cores"
	}

	memoryUnitParam := c.QueryParam("memory-unit")
	memoryUnitOptions := map[string]bool{
		"bytes": true,
		"MiB":   true,
		"GiB":   true,
	}

	if memoryUnitParam != "" {
		if !memoryUnitOptions[memoryUnitParam] {
			return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "invalid memory unit"})
		} else {
			unitChoices["memory"] = memoryUnitParam
		}
	} else {
		unitChoices["memory"] = "MiB"
	}

	recommendationSetVar := model.RecommendationSet{}
	recommendationSet, error := recommendationSetVar.GetRecommendationSetByID(OrgID, RecommendationUUID.String(), user_permissions)

	if error != nil {
		log.Errorf("unable to fetch recommendation %s; error %v", RecommendationIDStr, error)
		return c.JSON(http.StatusNotFound, echo.Map{"status": "error", "message": "unable to fetch recommendation"})
	}

	if len(recommendationSet.Recommendations) != 0 {
		recommendationSet.RecommendationsJSON = UpdateRecommendationJSON(
			handlerName,
			recommendationSet.ID,
			recommendationSet.ClusterUUID,
			unitChoices,
			setk8sUnits,
			recommendationSet.Recommendations)
		return c.JSON(http.StatusOK, recommendationSet)
	} else {
		return c.JSON(http.StatusNotFound, echo.Map{"status": "not_found", "message": "recommendation not found"})
	}
}

func GetAppStatus(c echo.Context) error {
	status := map[string]string{
		"api-server": "working",
	}
	return c.JSON(http.StatusOK, status)
}
