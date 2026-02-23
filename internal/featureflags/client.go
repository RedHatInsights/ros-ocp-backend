package featureflags

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Unleash/unleash-go-sdk/v6"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
)

var log *logrus.Entry = logging.GetLogger()
var cfg = config.GetConfig()

func Init() error {
	// Initialize Unleash client
	err := unleash.Initialize(
		unleash.WithAppName(cfg.ServiceName),
		unleash.WithUrl(cfg.UnleashFullURL),
		unleash.WithCustomHeaders(http.Header{
			"Authorization": {cfg.UnleashClientAccessToken},
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
