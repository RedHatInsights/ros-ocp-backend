package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/kessel"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/sirupsen/logrus"
)

var kesselLog *logrus.Entry = logging.GetLogger()

// kesselPermissions maps RBAC-style permission keys to Kessel permission names.
var kesselPermissions = map[string]string{
	"openshift.cluster": "cost_management_openshift_cluster_read",
	"openshift.node":    "cost_management_openshift_node_read",
	"openshift.project": "cost_management_openshift_project_read",
}

// kesselResourceTypes maps RBAC-style keys that support LookupResources
// to their Kessel resource type identifiers (namespace/name format).
var kesselResourceTypes = map[string]string{
	"openshift.cluster": "cost_management/openshift_cluster",
	"openshift.project": "cost_management/openshift_project",
}

// kesselResourceRelation is the permission name on the resource type to use with LookupResources.
const kesselResourceRelation = "read"

// KesselMiddleware returns an Echo middleware that checks Kessel permissions.
//
// For cluster and project: calls ListAuthorizedResources first.
//   - If specific IDs are returned, those IDs become the permission value.
//   - If empty (or error), falls back to CheckPermission for wildcard.
//
// For node: always uses CheckPermission (no per-resource listing).
//
// This produces user.permissions in the same shape as the RBAC middleware:
//
//	{"openshift.cluster": ["*"] | ["id1","id2"], ...}
func KesselMiddleware(checker kessel.PermissionChecker) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id, ok := c.Get("Identity").(identity.XRHID)
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}

			orgID := id.Identity.OrgID
			username := id.Identity.User.Username
			if orgID == "" || username == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}

			permissions := map[string][]string{}
			ctx := c.Request().Context()

			for rbacKey, kesselPerm := range kesselPermissions {
				resourceType, useList := kesselResourceTypes[rbacKey]

				if useList {
					ids, err := checker.ListAuthorizedResources(ctx, orgID, resourceType, kesselResourceRelation, username)
					if err != nil {
						kesselLog.Errorf("LookupResources failed for %s (org=%s, user=%s): %v", resourceType, orgID, username, err)
					} else if len(ids) > 0 {
						permissions[rbacKey] = ids
						continue
					}
				}

				allowed, err := checker.CheckPermission(ctx, orgID, kesselPerm, username)
				if err != nil {
					kesselLog.Errorf("CheckPermission failed for %s (org=%s, user=%s): %v", kesselPerm, orgID, username, err)
					continue
				}
				if allowed {
					permissions[rbacKey] = []string{"*"}
				}
			}

			if len(permissions) == 0 {
				return echo.NewHTTPError(http.StatusForbidden, "User is not authorized")
			}

			c.Set("user.permissions", permissions)
			return next(c)
		}
	}
}

// SelectAuthMiddleware returns the appropriate authorization middleware based on
// config. Returns nil when authorization is disabled (RBACEnabled=false).
func SelectAuthMiddleware(cfg *config.Config, checker kessel.PermissionChecker) echo.MiddlewareFunc {
	if !cfg.RBACEnabled {
		return nil
	}
	switch strings.ToLower(cfg.AuthorizationBackend) {
	case "kessel":
		return KesselMiddleware(checker)
	default:
		return Rbac
	}
}
