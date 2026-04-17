package testutil

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

// CaptureLogOutput runs fn while capturing all logrus output at the given level.
// It restores the original output and level after fn returns.
func CaptureLogOutput(logger *logrus.Logger, level logrus.Level, fn func()) string {
	origOut := logger.Out
	origLevel := logger.Level
	var buf bytes.Buffer
	logger.SetOutput(&buf)
	logger.SetLevel(level)
	defer func() {
		logger.SetOutput(origOut)
		logger.SetLevel(origLevel)
	}()
	fn()
	return buf.String()
}

// NewEchoContext creates an echo.Context backed by a real HTTP request/response
// for use in handler unit tests.
func NewEchoContext(method, path string, body io.Reader) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

// NewEchoContextWithHeaders is like NewEchoContext but also sets request headers.
func NewEchoContextWithHeaders(method, path string, body io.Reader, headers http.Header) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, body)
	for k, vals := range headers {
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	if req.Header.Get(echo.HeaderContentType) == "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}
