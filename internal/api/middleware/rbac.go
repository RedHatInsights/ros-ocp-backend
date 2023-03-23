package middleware

import (
	"encoding/json"
	"fmt"

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
		if is_user_authorized(c.Request().Header.Get("X-Rh-Identity")) {
			return next(c)
		} else {
			return echo.NewHTTPError(http.StatusUnauthorized, "User is not authorized")
		}
	}
}

func is_user_authorized(encodedIdentity string) bool {
	cfg := config.GetConfig()
	url := fmt.Sprintf(
		"%s://%s:%s/api/rbac/v1/access/?application=cost-management&limit=100",
		cfg.RBACProtocol, cfg.RBACHost, cfg.RBACPort,
	)
	acls := request_user_access(url, encodedIdentity)
	if len(acls) > 0 {
		for _, acl := range acls {
			if acl.Permission == "cost-management:openshift.cluster:*" {
				return true
			}
		}
	}
	return false
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
