package identity

import (
	"encoding/json"
	"encoding/base64"

	"github.com/labstack/echo/v4"
	"github.com/redhatinsights/ros-ocp-backend/internal/logging"
)


type System struct {
	CN       string `json:"cn"`
	CertType string `json:"cert_type"`
}

type Internal struct {
	OrgID     string `json:"org_id"`
	AuthTime  int    `json:"auth_time"`
}

type Identity struct {
	OrgID      string   `json:"org_id"`
	Type       string   `json:"type"`
	AuthType   string   `json:"auth_type"`
	System     System   `json:"system"`
	Internal   Internal `json:"internal"`
}

type IdentityData struct {
	Identity Identity `json:"identity"`
}

func (id *IdentityData) GetOrgIDFromRequest(c echo.Context) (string, error) {
    log := logging.GetLogger()
    encodedIdentity := c.Request().Header.Get("X-Rh-Identity")
    decodedIdentity, err := base64.StdEncoding.DecodeString(encodedIdentity)
    if err != nil {
        log.Error("unable to ascertain identity")
        return "", err
    }

    if err := json.Unmarshal(decodedIdentity, &id); err != nil {
        log.Error("unable to unmarshal identity data")
        return "", err
    }

    return id.Identity.OrgID, nil
}