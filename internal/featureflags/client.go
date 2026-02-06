package featureflags

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Unleash/unleash-go-sdk/v5"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
)

var log *logrus.Entry = logging.GetLogger()

func Init() error {
	cfg := config.GetConfig()
	// Initialize Unleash client
	err := unleash.Initialize(
		unleash.WithAppName(cfg.ServiceName),
		unleash.WithUrl(cfg.FeatureFlagsFullURL),
		unleash.WithCustomHeaders(http.Header{
			"Authorization": {cfg.FeatureFlagsClientAccessToken},
		}),
		unleash.WithRefreshInterval(15*time.Second),
		unleash.WithListener(&unleash.DebugListener{}),
	)

	if err != nil {
		return fmt.Errorf("failed to initialize Unleash client: %w", err)
	}

	log.Info("Unleash client initialized successfully")
	return nil
}
