package api

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	v1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcInsecure "google.golang.org/grpc/credentials/insecure"

	ros_middleware "github.com/redhatinsights/ros-ocp-backend/internal/api/middleware"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/kessel"
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

	var kesselClient kessel.PermissionChecker
	if strings.EqualFold(cfg.AuthorizationBackend, "kessel") {
		var dialOpts []grpc.DialOption
		if cfg.KesselRelationsCAPath != "" {
			tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
			caCert, err := os.ReadFile(cfg.KesselRelationsCAPath)
			if err != nil {
				log.Fatalf("failed to read Kessel CA cert at %s: %v", cfg.KesselRelationsCAPath, err)
			}
			certPool := x509.NewCertPool()
			if !certPool.AppendCertsFromPEM(caCert) {
				log.Fatal("failed to parse Kessel CA certificate")
			}
			tlsConfig.RootCAs = certPool
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		} else {
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(grpcInsecure.NewCredentials()))
		}
		conn, err := grpc.NewClient(cfg.KesselRelationsURL, dialOpts...)
		if err != nil {
			log.Fatalf("failed to create Kessel gRPC connection: %v", err)
		}
		kesselClient = kessel.NewKesselClient(
			v1beta1.NewKesselCheckServiceClient(conn),
			v1beta1.NewKesselLookupServiceClient(conn),
		)
	}

	v1 := app.Group("/api/cost-management/v1")
	v1.Use(ros_middleware.Identity)
	if authMW := ros_middleware.SelectAuthMiddleware(cfg, kesselClient); authMW != nil {
		v1.Use(authMW)
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
