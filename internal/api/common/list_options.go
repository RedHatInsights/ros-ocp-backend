package common

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	DefaultLimit = 10
	DefaultOffset = 0

	OrderAsc  = "asc"
	OrderDesc = "desc"
	ResponseFormatJSON = "json"
	ResponseFormatCSV  = "csv"
)

type ListOptions struct {
	Limit    int
	Offset   int
	OrderBy  string
	OrderHow string
	Format string
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

func ListAPIOptions(c echo.Context, defaultDBColumn string, allowedOrderBy map[string]string) (ListOptions, error) {

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
		Format: format,
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
