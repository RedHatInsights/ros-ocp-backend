package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/ros-ocp-backend/internal/api/listoptions"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
)

func GetRecommendationSetList(c echo.Context) error {
	XRHID := c.Get("Identity").(identity.XRHID)
	OrgID := XRHID.Identity.OrgID
	user_permissions := get_user_permissions(c)
	handlerName := "recommendationset-list"

	apiListOptions, err := listoptions.ListAPIOptions(c, listoptions.DefaultContainerRecsDBColumn, listoptions.ContainerAllowedOrderBy)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"status":  "error",
			"message": err.Error(),
		})
	}

	queryParams, err := MapQueryParameters(c)
	if err != nil {
		return apiErrResponse(c, err, http.StatusBadRequest, err.Error())
	}

	unitChoices, setk8sUnits, unitParseErr := ParseUnitParams(c, "cores", "bytes")
	if unitParseErr != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": unitParseErr.Error()})
	}

	recommendationSet := model.RecommendationSet{}
	recommendationSets, count, queryErr := recommendationSet.GetRecommendationSets(OrgID, apiListOptions, queryParams, user_permissions)
	if queryErr != nil {
		log.Errorf("unable to fetch records from database; %v", queryErr)
		return c.JSON(http.StatusServiceUnavailable, echo.Map{
			"status":  "error",
			"message": "unable to fetch records from database",
		})
	}

	for i := range recommendationSets {
		recommendationSets[i].RecommendationsJSON = UpdateRecommendationJSON(
			handlerName,
			recommendationSets[i].ID,
			recommendationSets[i].ClusterUUID,
			unitChoices,
			setk8sUnits,
			recommendationSets[i].Recommendations,
			&recommendationSets[i].StoredVariationPcts,
		)
	}

	switch apiListOptions.Format {
	case listoptions.ResponseFormatJSON:
		interfaceSlice := make([]any, len(recommendationSets))
		for i, v := range recommendationSets {
			interfaceSlice[i] = v
		}
		results := CollectionResponse(interfaceSlice, c.Request(), count, apiListOptions.Limit, apiListOptions.Offset)
		return c.JSON(http.StatusOK, results)
	case listoptions.ResponseFormatCSV:
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

	unitChoices, setk8sUnits, unitParseErr := ParseUnitParams(c, "cores", "MiB")
	if unitParseErr != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"status": "error", "message": unitParseErr.Error()})
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
			recommendationSet.Recommendations,
			&recommendationSet.StoredVariationPcts,
		)
		return c.JSON(http.StatusOK, recommendationSet)
	} else {
		return c.JSON(http.StatusNotFound, echo.Map{"status": "not_found", "message": "recommendation not found"})
	}
}

func GetNamespaceRecommendationSetList(c echo.Context) error {
	XRHID := c.Get("Identity").(identity.XRHID)
	OrgID := XRHID.Identity.OrgID
	user_permissions := get_user_permissions(c)
	handlerName := "namespace-recommendationset-list"

	apiListOptions, listOptionsErr := listoptions.ListAPIOptions(c, listoptions.DefaultNsRecsDBColumn, listoptions.NsAllowedOrderBy)
	if listOptionsErr != nil {
		return apiErrResponse(c, listOptionsErr, http.StatusBadRequest, listOptionsErr.Error())
	}

	queryParams, paramErr := MapNamespaceQueryParameters(c)
	if paramErr != nil {
		return apiErrResponse(c, paramErr, http.StatusBadRequest, paramErr.Error())
	}

	unitChoices, setk8sUnits, err := ParseUnitParams(c, "cores", "bytes")
	if err != nil {
		return apiErrResponse(c, err, http.StatusBadRequest, err.Error())
	}

	NamespaceRecommendationSet := model.NamespaceRecommendationSet{}
	namespaceRecommendationSets, count, queryErr := NamespaceRecommendationSet.GetNamespaceRecommendationSets(
		OrgID, apiListOptions, queryParams, user_permissions,
	)

	if queryErr != nil {
		return apiErrResponse(c, queryErr, http.StatusServiceUnavailable, "unable to fetch records from database")
	}

	for i := range namespaceRecommendationSets {
		namespaceRecommendationSets[i].RecommendationsJSON = UpdateRecommendationJSON(
			handlerName,
			namespaceRecommendationSets[i].ID,
			namespaceRecommendationSets[i].ClusterUUID,
			unitChoices,
			setk8sUnits,
			namespaceRecommendationSets[i].Recommendations,
			&namespaceRecommendationSets[i].StoredVariationPcts,
		)
	}

	switch apiListOptions.Format {
	case listoptions.ResponseFormatJSON:
		interfaceSlice := make([]any, len(namespaceRecommendationSets))
		for i, v := range namespaceRecommendationSets {
			interfaceSlice[i] = v
		}
		results := CollectionResponse(interfaceSlice, c.Request(), count, apiListOptions.Limit, apiListOptions.Offset)
		return c.JSON(http.StatusOK, results)
	case listoptions.ResponseFormatCSV:
		// TODO: Add CSV support when export feature is enabled
		csvErr := errors.New("CSV format is not supported. Please use application/json")
		return apiErrResponse(c, csvErr, http.StatusNotAcceptable, csvErr.Error())
	}
	return nil

}

func GetNamespaceRecommendationSet(c echo.Context) error {
	XRHID := c.Get("Identity").(identity.XRHID)
	OrgID := XRHID.Identity.OrgID
	user_permissions := get_user_permissions(c)
	handlerName := "namespace-recommendationset"

	RecommendationIDStr := c.Param("recommendation-id")
	RecommendationUUID, err := uuid.Parse(RecommendationIDStr)
	if err != nil {
		return apiErrResponse(c, err, http.StatusBadRequest, "bad recommendation-id for project")
	}

	unitChoices, setk8sUnits, unitParseErr := ParseUnitParams(c, "cores", "MiB")
	if unitParseErr != nil {
		return apiErrResponse(c, unitParseErr, http.StatusBadRequest, unitParseErr.Error())
	}

	recommendationSetVar := model.NamespaceRecommendationSet{}
	nsRecommendationSet, getNSRecordErr := recommendationSetVar.GetNamespaceRecommendationSetByID(
		OrgID,
		RecommendationUUID.String(),
		user_permissions,
	)

	if getNSRecordErr != nil {
		return apiErrResponse(c, getNSRecordErr, http.StatusNotFound, "unable to fetch project recommendation")
	}

	if len(nsRecommendationSet.Recommendations) != 0 {
		nsRecommendationSet.RecommendationsJSON = UpdateRecommendationJSON(
			handlerName,
			nsRecommendationSet.ID,
			nsRecommendationSet.ClusterUUID,
			unitChoices,
			setk8sUnits,
			nsRecommendationSet.Recommendations,
			&nsRecommendationSet.StoredVariationPcts,
		)
	}
	return c.JSON(http.StatusOK, nsRecommendationSet)
}

func GetAppStatus(c echo.Context) error {
	status := map[string]string{
		"api-server": "working",
	}
	return c.JSON(http.StatusOK, status)
}
