package services

import (
	"testing"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
)

func TestDisableNamespaceRecommendation(t *testing.T) {
	/*
	  Regression test, feature currently disabled
	*/

	cfg := config.GetConfig()
	if cfg == nil {
		t.Fatal("unable to fetch config")
	}

	if !cfg.DisableNamespaceRecommendation {
		t.Error("feature is currently disabled")
	}
}
