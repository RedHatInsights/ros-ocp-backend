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

	// Setup OAuth2 authentication for metrics endpoint (if enabled)
	if cfg.MetricsEnabled {
		oauth2Handler, err := ros_middleware.GetIdentityProviderHandlerFunction(ros_middleware.OAuth2IDProvider)
		if err != nil {
			log.Fatalf("Failed to initialize OAuth2 authentication for metrics endpoint: %v", err)
		}

		go func() {
			metrics := echo.New()
			// Apply OAuth2 authentication to metrics endpoint (RHSSO not supported)
			metrics.Use(oauth2Handler)
			metrics.GET("/metrics", echoprometheus.NewHandler())
			log.Infof("Starting metrics endpoint on port %s with OAuth2 authentication", cfg.PrometheusPort)
			if err := metrics.Start(fmt.Sprintf(":%s", cfg.PrometheusPort)); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatal(err)
			}
		}()
	} else {
		log.Info("Metrics endpoint disabled (METRICS_ENABLED=false)")
	}

	app.Use(middleware.Logger())
	app.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowMethods: []string{http.MethodGet},
	}))

	app.GET("/status", GetAppStatus)
	app.File("/api/cost-management/v1/recommendations/openshift/openapi.json", "openapi.json")

	// Setup RHSSO authentication for REST API endpoints only
	rhssoHandler, err := ros_middleware.GetIdentityProviderHandlerFunction(ros_middleware.RHSSOIDProvider)
	if err != nil {
		log.Fatalf("Failed to initialize RHSSO authentication for REST API endpoints: %v", err)
	}

	// REST endpoints - RHSSO authentication required
	v1 := app.Group("/api/cost-management/v1")
	v1.Use(rhssoHandler)
	if cfg.RBACEnabled {
		v1.Use(ros_middleware.Rbac)
	}
	v1.GET("/recommendations/openshift", GetRecommendationSetList)
	v1.GET("/recommendations/openshift/:recommendation-id", GetRecommendationSet)

	s := http.Server{
		Addr:              ":" + cfg.API_PORT, // local dev server
		Handler:           app,
		ReadHeaderTimeout: time.Duration(cfg.ReadHeaderTimeout) * time.Second,
	}
	if err := s.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
