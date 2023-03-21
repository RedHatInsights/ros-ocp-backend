package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"

)

func StartAPIServer() {
	app := echo.New()
	log := logging.GetLogger()
	cfg := config.GetConfig()
	app.Use(middleware.Logger())
	app.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowMethods: []string{http.MethodGet},
	}))

	app.GET("/api/cost-management/v1/recommendations/openshift", GetRecommendationSetList)
	app.GET("/api/cost-management/v1/recommendations/openshift/:recommendation-id", GetRecommendationSet)
	app.GET("/api/cost-management/v1/recommendations/openshift/openapi.json", GetOpenAPISpec)

	s := http.Server{
		Addr:    ":" + cfg.API_PORT, //local dev server
		Handler: app,
	}
	if err := s.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
