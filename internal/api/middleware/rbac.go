package middleware

import (
	"encoding/json"
	"fmt"
	"strings"

	"io"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/sirupsen/logrus"
)

var cfg *config.Config = config.GetConfig()
var log *logrus.Logger = logging.GetLogger()

func Rbac(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		permissions := get_user_permissions(c.Request().Header.Get("X-Rh-Identity"))
		if permissions != nil {
			c.Set("user.permissions", permissions)
		} else {
			return echo.NewHTTPError(http.StatusUnauthorized, "User is not authorized")
		}
		return next(c)
	}
}

func aggregate_permissions(acls []types.RbacData) map[string][]string {
	permissions := map[string][]string{}
	for _, acl := range acls {
		resourceType := strings.Split(acl.Permission, ":")[1]
		if strings.Contains(resourceType, "openshift") {
			if _, ok := permissions[resourceType]; !ok {
				permissions[resourceType] = []string{}
			}
			if len(acl.ResourceDefinitions) == 0 {
				permissions[resourceType] = append(permissions[resourceType], "*")
			} else {
				for _, resourceDefinition := range acl.ResourceDefinitions {
					permissions[resourceType] = append(permissions[resourceType], resourceDefinition.AttributeFilter.Value...)
				}
			}
		} else if resourceType == "*" {
			permissions["*"] = []string{}
		}
	}
	return permissions
}

func get_user_permissions(encodedIdentity string) map[string][]string {
	cfg := config.GetConfig()
	url := fmt.Sprintf(
		"%s://%s:%s/api/rbac/v1/access/?application=cost-management&limit=100",
		cfg.RBACProtocol, cfg.RBACHost, cfg.RBACPort,
	)
	acls := request_user_access(url, encodedIdentity)
	if len(acls) > 0 {
		permissions := aggregate_permissions(acls)
		if len(permissions) > 0 {
			return permissions
		}
		return nil
	}
	return nil
}

func request_user_access(url, encodedIdentity string) []types.RbacData {
	access := []types.RbacData{}
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Errorf("an Error Occured %v", err)
	}
	req.Header.Set("x-rh-identity", encodedIdentity)
	res, err := client.Do(req)
	if err != nil {
		log.Errorf("error Occured while calling RBAC API %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	response := types.RbacResponse{}
	if err := json.Unmarshal(body, &response); err != nil {
		log.Errorf("unable to unmarshal response of RBAC API %v", err)
	}
	access = append(access, response.Data...)
	if response.Links.Next != "" {
		next_url := fmt.Sprintf("%s://%s:%s%s", cfg.RBACProtocol, cfg.RBACHost, cfg.RBACPort, response.Links.Next)
		access = append(access, request_user_access(next_url, encodedIdentity)...)
	}
	return access
}
