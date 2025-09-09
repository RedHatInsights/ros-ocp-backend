package sources

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
)

var cfg *config.Config = config.GetConfig()

func GetCostApplicationID() (int, error) {
	url := cfg.SourceApiBaseUrl + cfg.SourceApiPrefix + "/application_types?filter[name][eq]=/insights/platform/cost-management"
	res, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("error while calling sources API: %v", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		return 0, fmt.Errorf("%v", res)
	}
	payload := map[string]interface{}{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, fmt.Errorf("unable to unmarshal response of sources /application_types API %v", err)
	}
	data := payload["data"].([]interface{})
	app := data[0].(map[string]interface{})
	cost_app_id, _ := strconv.Atoi(app["id"].(string))
	return cost_app_id, nil
}
