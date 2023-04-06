package api

import (
	"net/http"
	"strconv"
	"strings"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/identity"
)

var variationDummyObject = map[string]interface{}{
    "limits": map[string]interface{}{
        "cpu": map[string]interface{}{
            "amount": 0.02,
            "format": "cores",
        },
        "memory": map[string]interface{}{
            "amount": 513900,
            "format": "MiB",
        },
    },
    "requests": map[string]interface{}{
        "cpu": map[string]interface{}{
            "amount": 0.01,
            "format": "cores",
        },
        "memory": map[string]interface{}{
            "amount": 4933.5,
            "format": "MiB",
        },
    },
}

func GetRecommendationSetList(c echo.Context) error {
	var identity identity.IdentityData
	OrgID, err := identity.GetOrgIDFromRequest(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "org_id not found"})
	}

	log := logging.GetLogger()

	orderBy := c.QueryParam("order_by")
	if orderBy != "" {
		var orderByOptions = map[string]string{
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

	orderHow := c.QueryParam("order_how")
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
	log.Info("============================")
	log.Infof("User orgID = %s", OrgID)
	log.Info("============================")
	recommendationSets, error := recommendationSet.GetRecommendationSets(OrgID, orderQuery, limit, offset, queryParams)
	log.Info("============================")
	log.Infof("recommendationSets got from DB = %v", recommendationSets)
	log.Info("============================")
	if error != nil {
		log.Error("unable to fetch records from database", error)
	}

	allRecommendations := []map[string]interface{}{}

	for _, recommendation := range recommendationSets {
		recommendationData := make(map[string]interface{})

		// Adding dummy variation object
		var recommendationObject map[string]interface{}
    	if err := json.Unmarshal(recommendation.Recommendations, &recommendationObject); err != nil {
			log.Error("unable to unmarshall duration based recommendations", error)
		}
		
		longTermSection := recommendationObject["duration_based"].(map[string]interface{})["long_term"].(map[string]interface{})
		shortTermSection := recommendationObject["duration_based"].(map[string]interface{})["short_term"].(map[string]interface{})
		mediumTermSection := recommendationObject["duration_based"].(map[string]interface{})["medium_term"].(map[string]interface{})
		shortTermSection["variation"] = variationDummyObject
		mediumTermSection["variation"] = variationDummyObject
		longTermSection["variation"] = variationDummyObject

		recommendationData["id"] = recommendation.ID
		recommendationData["cluster_uuid"] = recommendation.Workload.Cluster.ClusterUUID
		recommendationData["cluster_alias"] = recommendation.Workload.Cluster.ClusterAlias
		recommendationData["project"] = recommendation.Workload.Namespace
		recommendationData["workload_type"] = recommendation.Workload.WorkloadType
		recommendationData["workload"] = recommendation.Workload.WorkloadName
		recommendationData["container"] = recommendation.ContainerName
		recommendationData["last_reported"] = recommendation.Workload.Cluster.LastReportedAtStr
		recommendationData["recommendations"] = recommendationObject
		allRecommendations = append(allRecommendations, recommendationData)
	}

	interfaceSlice := make([]interface{}, len(allRecommendations))
	for i, v := range allRecommendations {
		interfaceSlice[i] = v
	}
	results := CollectionResponse(interfaceSlice, c.Request(), len(allRecommendations), limit, offset)
	log.Info("============================")
	log.Infof("Data we are sending in response = %+v", results)
	log.Info("============================")
	return c.JSON(http.StatusOK, results)

}

func GetRecommendationSet(c echo.Context) error {
	log := logging.GetLogger()
	var identity identity.IdentityData
	OrgID, err := identity.GetOrgIDFromRequest(c)
	if err != nil {
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

		// Adding dummy variation object
		var recommendationObject map[string]interface{}
		if err := json.Unmarshal(recommendationSet.Recommendations, &recommendationObject); err != nil {
			log.Error("unable to unmarshall duration based recommendations", error)
		}
		longTermSection := recommendationObject["duration_based"].(map[string]interface{})["long_term"].(map[string]interface{})
		shortTermSection := recommendationObject["duration_based"].(map[string]interface{})["short_term"].(map[string]interface{})
		mediumTermSection := recommendationObject["duration_based"].(map[string]interface{})["medium_term"].(map[string]interface{})
		shortTermSection["variation"] = variationDummyObject
		mediumTermSection["variation"] = variationDummyObject
		longTermSection["variation"] = variationDummyObject

		recommendationSlice["id"] = recommendationSet.ID
		recommendationSlice["cluster_uuid"] = recommendationSet.Workload.Cluster.ClusterUUID
		recommendationSlice["cluster_alias"] = recommendationSet.Workload.Cluster.ClusterAlias
		recommendationSlice["project"] = recommendationSet.Workload.Namespace
		recommendationSlice["workload_type"] = recommendationSet.Workload.WorkloadType
		recommendationSlice["workload"] = recommendationSet.Workload.WorkloadName
		recommendationSlice["container"] = recommendationSet.ContainerName
		recommendationSlice["last_reported"] = recommendationSet.Workload.Cluster.LastReportedAtStr
		recommendationSlice["recommendations"] = recommendationObject
	}

	return c.JSON(http.StatusOK, recommendationSlice)
}
