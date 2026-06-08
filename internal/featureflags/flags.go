package featureflags

import (
	"sync"

	"github.com/Unleash/unleash-go-sdk/v5"
	"github.com/Unleash/unleash-go-sdk/v5/context"
)

var namespaceRecommendationDisabledOnce sync.Once

func IsNamespaceEnabled(org_id string) bool {
	if cfg.DisableNamespaceRecommendation {
		// warn once per pod lifecycle
		namespaceRecommendationDisabledOnce.Do(func() {
			log.Warn("namespace recommendation feature disabled application-wide")
		})
		return false
	}

	flag := "rosocp.namespace_enabled"

	if org_id == "" {
		return unleash.IsEnabled(flag)
	}

	ctx := context.Context{
		Properties: map[string]string{
			"orgId": org_id,
		},
	}

	return unleash.IsEnabled(flag, unleash.WithContext(ctx))
}
