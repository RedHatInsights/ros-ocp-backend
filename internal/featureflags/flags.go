package featureflags

import (
	"github.com/Unleash/unleash-go-sdk/v5"
	"github.com/Unleash/unleash-go-sdk/v5/context"
)

func IsNamespaceEnabled(isNamespaceENVDisabled bool, org_id string) bool {
	if isNamespaceENVDisabled {
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
