package api

import (
	"os"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/google/uuid"

	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/identity"
)

func GetRecommendationSetList(c echo.Context) error {
	var identity identity.IdentityData
	OrgID, err := identity.GetOrgIDFromRequest(c)
	if err != nil{
		return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "org_id not found"})
	}
	
	log := logging.GetLogger()

	limitStr := c.QueryParam("limit")
	limit := 10 // default value
	if limitStr != "" {
		limitInt, err := strconv.Atoi(limitStr)
		if err == nil {
			limit = limitInt
		}
	}

	offsetStr := c.QueryParam("offset")
	offset := 0 // default value
	if offsetStr != "" {
		offsetInt, err := strconv.Atoi(offsetStr)
		if err == nil {
			offset = offsetInt
		}
	}

	queryParams := MapQueryParameters(c)
	recommendationSet := model.RecommendationSet{}
	recommendationSets, error := recommendationSet.GetRecommendationSets(OrgID, limit, offset, queryParams)

	if error != nil {
		log.Error("unable to fetch records from database", error)
	}

	allRecommendations := []map[string]interface{}{}

	for _, recommendation := range recommendationSets {
		recommendationData := make(map[string]interface{})
		recommendationData["id"] = recommendation.ID
		recommendationData["cluster_uuid"] = recommendation.Workload.Cluster.ClusterUUID
		recommendationData["cluster_alias"] = recommendation.Workload.Cluster.ClusterAlias
		recommendationData["project"] = recommendation.Workload.Namespace
		recommendationData["workload_type"] = recommendation.Workload.WorkloadType
		recommendationData["workload"] = recommendation.Workload.WorkloadName
		recommendationData["containers"] = recommendation.Workload.Containers
		recommendationData["last_report"] = recommendation.Workload.Cluster.LastReportedAtStr
		recommendationData["values"] = recommendation.Recommendations
		allRecommendations = append(allRecommendations, recommendationData)
	}	

	interfaceSlice := make([]interface{}, len(allRecommendations))
	for i, v := range allRecommendations {
		interfaceSlice[i] = v
	}

	return c.JSON(http.StatusOK, CollectionResponse(interfaceSlice, c.Request(), len(allRecommendations), limit, offset))

}

func GetRecommendationSet(c echo.Context) error {
	log := logging.GetLogger()
	var identity identity.IdentityData
	OrgID, err := identity.GetOrgIDFromRequest(c)
	if err != nil{
		return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "org_id not found"})
	}
	
	RecommendationIDStr := c.Param("recommendation-id")
	RecommendationUUID, err := uuid.Parse(RecommendationIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "bad recommendation_id"})
	}
	
	recommendationSetVar := model.RecommendationSet{}
	recommendationSet, error := recommendationSetVar.GetRecommendationSetByID(OrgID, RecommendationUUID.String())

	if error != nil {
		log.Error("unable to fetch records from database", error)
	}

	recommendationSlice := make(map[string]interface{})

	if len(recommendationSet.Recommendations) != 0 {
		recommendationSlice["id"] = recommendationSet.ID
		recommendationSlice["cluster_uuid"] = recommendationSet.Workload.Cluster.ClusterUUID
		recommendationSlice["cluster_alias"] = recommendationSet.Workload.Cluster.ClusterAlias
		recommendationSlice["project"] = recommendationSet.Workload.Namespace
		recommendationSlice["workload_type"] = recommendationSet.Workload.WorkloadType
		recommendationSlice["workload"] = recommendationSet.Workload.WorkloadName
		recommendationSlice["containers"] = recommendationSet.Workload.Containers
		recommendationSlice["last_report"] = recommendationSet.Workload.Cluster.LastReportedAtStr
		recommendationSlice["values"] = recommendationSet.Recommendations
	}

	return c.JSON(http.StatusOK, recommendationSlice)
}

func GetOpenAPISpec(c echo.Context) error{

	path, err := os.Getwd()
        if err != nil {
            log.Error("unable to fetch working directory")
        }
	return c.File(path + "/internal/api/docs/v1/openapi.json")
}