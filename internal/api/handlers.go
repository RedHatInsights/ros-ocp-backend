package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
)

func GetRecommendationSetList(c echo.Context) error {
	XRHID := c.Get("Identity").(identity.XRHID)
	OrgID := XRHID.Identity.OrgID
	user_permissions := get_user_permissions(c)

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
	recommendationSets, count, error := recommendationSet.GetRecommendationSets(OrgID, orderQuery, limit, offset, queryParams, user_permissions)
	log.Info("============================")
	log.Infof("recommendationSets got from DB = %v", recommendationSets)
	log.Info("============================")
	if error != nil {
		log.Error("unable to fetch records from database", error)
	}

	allRecommendations := []map[string]interface{}{}

	for _, recommendation := range recommendationSets {
		recommendationData := make(map[string]interface{})

		recommendationData["id"] = recommendation.ID
		recommendationData["source_id"] = recommendation.Workload.Cluster.SourceId
		recommendationData["cluster_uuid"] = recommendation.Workload.Cluster.ClusterUUID
		recommendationData["cluster_alias"] = recommendation.Workload.Cluster.ClusterAlias
		recommendationData["project"] = recommendation.Workload.Namespace
		recommendationData["workload_type"] = recommendation.Workload.WorkloadType
		recommendationData["workload"] = recommendation.Workload.WorkloadName
		recommendationData["container"] = recommendation.ContainerName
		recommendationData["last_reported"] = recommendation.Workload.Cluster.LastReportedAtStr
		recommendationData["recommendations"] = recommendation.Recommendations
		allRecommendations = append(allRecommendations, recommendationData)

	}

	interfaceSlice := make([]interface{}, len(allRecommendations))
	for i, v := range allRecommendations {
		interfaceSlice[i] = v
	}
	results := CollectionResponse(interfaceSlice, c.Request(), count, limit, offset)
	log.Info("============================")
	log.Infof("Data we are sending in response = %+v", results)
	log.Info("============================")
	return c.JSON(http.StatusOK, results)

}

func GetRecommendationSet(c echo.Context) error {
	XRHID := c.Get("Identity").(identity.XRHID)
	OrgID := XRHID.Identity.OrgID
	user_permissions := get_user_permissions(c)

	RecommendationIDStr := c.Param("recommendation-id")
	RecommendationUUID, err := uuid.Parse(RecommendationIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": "bad recommendation_id"})
	}

	recommendationSetVar := model.RecommendationSet{}
	recommendationSet, error := recommendationSetVar.GetRecommendationSetByID(OrgID, RecommendationUUID.String(), user_permissions)

	if error != nil {
		log.Error("unable to fetch records from database", error)
	}

	recommendationSlice := make(map[string]interface{})

	if len(recommendationSet.Recommendations) != 0 {
		recommendationSlice["id"] = recommendationSet.ID
		recommendationSlice["source_id"] = recommendationSet.Workload.Cluster.SourceId
		recommendationSlice["cluster_uuid"] = recommendationSet.Workload.Cluster.ClusterUUID
		recommendationSlice["cluster_alias"] = recommendationSet.Workload.Cluster.ClusterAlias
		recommendationSlice["project"] = recommendationSet.Workload.Namespace
		recommendationSlice["workload_type"] = recommendationSet.Workload.WorkloadType
		recommendationSlice["workload"] = recommendationSet.Workload.WorkloadName
		recommendationSlice["container"] = recommendationSet.ContainerName
		recommendationSlice["last_reported"] = recommendationSet.Workload.Cluster.LastReportedAtStr
		recommendationSlice["recommendations"] = recommendationSet.Recommendations
	}

	return c.JSON(http.StatusOK, recommendationSlice)

}

func GetAppStatus(c echo.Context) error {
	status := map[string]string{
		"api-server": "working",
	}
	return c.JSON(http.StatusOK, status)
}
