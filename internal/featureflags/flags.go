package featureflags

import (
	"github.com/Unleash/unleash-go-sdk/v6"
	"github.com/Unleash/unleash-go-sdk/v6/context"
)

func IsNamespaceEnabled(org_id string) bool {
	if cfg.DisableNamespaceRecommendation {
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
