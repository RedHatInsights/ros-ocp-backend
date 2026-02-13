package listoptions

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	DefaultLimit  = 10
	DefaultOffset = 0

	OrderAsc           = "asc"
	OrderDesc          = "desc"
	ResponseFormatJSON = "json"
	ResponseFormatCSV  = "csv"

	// Default DB columns for OrderBy.
	DefaultContainerRecsDBColumn = "clusters.last_reported_at"
	DefaultNsRecsDBColumn        = "clusters.last_reported_at"
)

type ListOptions struct {
	Limit    int
	Offset   int
	OrderBy  string
	OrderHow string
	Format   string
}

// OrderByMap maps allowed JSON keys to DB columns.
type OrderByMap map[string]string

// API-specific maps and defaults.
var ContainerAllowedOrderBy = OrderByMap{
	"cluster":       "clusters.cluster_alias",
	"workload_type": "workloads.workload_type",
	"workload":      "workloads.workload_name",
	"project":       "workloads.namespace",
	"container":     "recommendation_sets.container_name",
	"last_reported": "clusters.last_reported_at",
}

var NsAllowedOrderBy = OrderByMap{
	"cluster":                "clusters.cluster_alias",
	"project":                "namespace_recommendation_sets.namespace_name",
	"cpu_request_current":    "namespace_recommendation_sets.cpu_request_current",
	"cpu_variation":          "namespace_recommendation_sets.cpu_variation",
	"memory_request_current": "namespace_recommendation_sets.memory_request_current",
	"memory_variation":       "namespace_recommendation_sets.memory_variation",
	"last_reported":          "clusters.last_reported_at",
}

func parseInt(val string, def int) int {
	if val == "" {
		return def
	}
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	return def
}

func ListAPIOptions(c echo.Context, defaultDBColumn string, allowedOrderBy OrderByMap) (ListOptions, error) {

	limit := parseInt(c.QueryParam("limit"), DefaultLimit)
	offset := parseInt(c.QueryParam("offset"), DefaultOffset)
	orderBy := strings.TrimSpace(c.QueryParam("order_by"))
	orderHow := strings.ToLower(c.QueryParam("order_how"))

	// Format handling
	acceptHeader := c.Request().Header.Get("Accept")
	formatParam := strings.ToLower(c.QueryParam("format"))

	format, err := resolveResponseFormat(acceptHeader, formatParam)
	if err != nil {
		return ListOptions{}, err
	}

	if offset < 0 {
		offset = DefaultOffset
	}

	if orderHow == "" {
		orderHow = OrderDesc
	}
	if orderHow != OrderAsc && orderHow != OrderDesc {
		return ListOptions{}, fmt.Errorf("invalid order_how value: %s", orderHow)
	}

	if orderBy != "" {
		dbColumn, ok := allowedOrderBy[orderBy]
		if !ok {
			return ListOptions{}, fmt.Errorf("invalid order_by value: %s", orderBy)
		}
		orderBy = dbColumn
	} else {
		// default orderBY
		orderBy = defaultDBColumn
	}

	return ListOptions{
		Limit:    limit,
		Offset:   offset,
		OrderBy:  orderBy,
		OrderHow: orderHow,
		Format:   format,
	}, nil
}

func resolveResponseFormat(acceptHeaderVal string, formatQueryParamVal string) (string, error) {
	if acceptHeaderVal == "" && formatQueryParamVal == "" {
		return ResponseFormatJSON, nil // default format
	}

	switch acceptHeaderVal {
	case "text/csv":
		return ResponseFormatCSV, nil
	case "application/json":
		return ResponseFormatJSON, nil
	}

	switch formatQueryParamVal {
	case "", "json":
		return ResponseFormatJSON, nil
	case "csv":
		return ResponseFormatCSV, nil
	default:
		return "", fmt.Errorf("invalid value for format: %q", formatQueryParamVal)
	}

}
