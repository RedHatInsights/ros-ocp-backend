package middleware

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
)

const (
	// Identity header for RHSSO authentication
	RHSSOIdentityHeader = "X-Rh-Identity"
)

type IdentityProvider interface {
	GetHandlerFunction() echo.MiddlewareFunc
}

type RHSSOIdentityProvider struct {
}

func NewRHSSOIdentityProvider() IdentityProvider {
	return &RHSSOIdentityProvider{}
}

func (r *RHSSOIdentityProvider) GetHandlerFunction() echo.MiddlewareFunc {
	return r.rhSSOIdentityHandlerFunction
}

func (r *RHSSOIdentityProvider) rhSSOIdentityHandlerFunction(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		decodedIdentity, err := decodeIdentity(c, RHSSOIdentityHeader)
		if err != nil {
			return err
		}

		id, err := identity.NewXRHIDFromHeader(decodedIdentity)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unable to marshal %s into struct", RHSSOIdentityHeader))
		}

		c.Set("Identity", id)
		return next(c)
	}
}

// GetIdentityProviderHandlerFunction returns the RHSSO identity provider middleware
// which validates X-Rh-Identity headers for all authenticated endpoints
func GetIdentityProviderHandlerFunction() (echo.MiddlewareFunc, error) {
	hf := NewRHSSOIdentityProvider()
	return hf.GetHandlerFunction(), nil
}

func decodeIdentity(c echo.Context, header string) ([]byte, error) {
	encodedIdentity := c.Request().Header.Get(header)
	decodedIdentity, err := base64.StdEncoding.DecodeString(encodedIdentity)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, fmt.Sprintf("Unable to decode %s", header))
	}
	return decodedIdentity, nil
}
