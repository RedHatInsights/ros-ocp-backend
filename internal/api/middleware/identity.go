package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
)

func Identity(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		id := identity.XRHID{}
		encodedIdentity := c.Request().Header.Get("X-Rh-Identity")
		decodedIdentity, err := base64.StdEncoding.DecodeString(encodedIdentity)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "Unable to decode X-Rh-Identity")
		}
		if err := json.Unmarshal(decodedIdentity, &id); err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "Unable to marshal X-Rh-Identity into struct")
		}
		c.Set("Identity", id)
		return next(c)
	}
}
