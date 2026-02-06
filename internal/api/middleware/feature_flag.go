package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/ros-ocp-backend/internal/featureflags"
)

func RequireNamespaceFeature() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			XRHID := c.Get("Identity").(identity.XRHID)
			OrgID := XRHID.Identity.OrgID

			if !featureflags.IsNamespaceEnabled(cfg.DisableNamespaceRecommendation, OrgID) {
				return echo.NewHTTPError(
					http.StatusServiceUnavailable,
					"Namespace Feature temporarily disabled / rollout pending",
				)
			}

			return next(c)
		}
	}
}
