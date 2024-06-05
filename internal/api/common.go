package api

const timeLayout = "2006-01-02"

type Collection struct {
	Data  []interface{} `json:"data"`
	Meta  Metadata      `json:"meta"`
	Links Links         `json:"links"`
}

type Metadata struct {
	Count  int `json:"count"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type Links struct {
	First    string `json:"first"`
	Previous string `json:"previous,omitempty"`
	Next     string `json:"next,omitempty"`
	Last     string `json:"last"`
}

var NotificationsToShow = map[string]string{
	"323004": "NOTICE",
	"323005": "NOTICE",
	"324003": "NOTICE",
	"324004": "NOTICE",
}

var MemoryUnitk8s = map[string]string{
	"bytes": "bytes",
	"MiB":   "Mi",
	"GiB":   "Gi",
}

var CPUUnitk8s = map[string]string{
	"millicores": "m",
	"cores":     "",
}
