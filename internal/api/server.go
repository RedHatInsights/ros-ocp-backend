package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"

	ros_middleware "github.com/redhatinsights/ros-ocp-backend/internal/api/middleware"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
)

var log *logrus.Logger = logging.GetLogger()
var cfg *config.Config = config.GetConfig()

func StartAPIServer() {
	app := echo.New()
	app.Use(middleware.Logger())
	app.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowMethods: []string{http.MethodGet},
	}))

	app.GET("/status", GetAppStatus)
	app.File("/api/cost-management/v1/recommendations/openshift/openapi.json", "openapi.json")

	v1 := app.Group("/api/cost-management/v1")
	v1.Use(ros_middleware.Identity)
	if cfg.RBACEnabled {
		v1.Use(ros_middleware.Rbac)
	}
	v1.GET("/recommendations/openshift", GetRecommendationSetList)
	v1.GET("/recommendations/openshift/:recommendation-id", GetRecommendationSet)

	s := http.Server{
		Addr:    ":" + cfg.API_PORT, //local dev server
		Handler: app,
	}
	if err := s.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
