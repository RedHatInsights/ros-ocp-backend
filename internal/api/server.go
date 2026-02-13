package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"

	ros_middleware "github.com/redhatinsights/ros-ocp-backend/internal/api/middleware"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
)

var log *logrus.Entry = logging.GetLogger()
var cfg *config.Config = config.GetConfig()

func StartAPIServer() {
	app := echo.New()
	app.Use(echoprometheus.NewMiddlewareWithConfig(echoprometheus.MiddlewareConfig{
		Subsystem: "rosocp",
		LabelFuncs: map[string]echoprometheus.LabelValueFunc{
			"url": func(c echo.Context, err error) string {
				return c.Path()
			},
		},
	}))

	go func() {
		metrics := echo.New()
		metrics.GET("/metrics", echoprometheus.NewHandler())
		if err := metrics.Start(fmt.Sprintf(":%s", cfg.PrometheusPort)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
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
	v1.GET("/openshift/namespace/recommendations", GetNamespaceRecommendationSetList)

	s := http.Server{
		Addr:              ":" + cfg.API_PORT, // local dev server
		Handler:           app,
		ReadHeaderTimeout: time.Duration(cfg.ReadHeaderTimeout) * time.Second,
	}
	if err := s.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
