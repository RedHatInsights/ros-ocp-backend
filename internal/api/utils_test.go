package api

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"gorm.io/datatypes"
)

func float64Ptr(v float64) *float64 { return &v }

// Minimal recommendation JSON that exercises both term and engine maps.
// short_term has cost + performance engines; medium_term has cost only.
const testRecommendationJSON = `{
	"monitoring_end_time": "2024-01-15T00:00:00.000Z",
	"current": {
		"limits": {"cpu": {"amount": 2.0, "format": "cores"}, "memory": {"amount": 4096, "format": "MiB"}},
		"requests": {"cpu": {"amount": 1.0, "format": "cores"}, "memory": {"amount": 2048, "format": "MiB"}}
	},
	"recommendation_terms": {
		"short_term": {
			"duration_in_hours": 24,
			"monitoring_start_time": "2024-01-14T00:00:00.000Z",
			"recommendation_engines": {
				"cost": {
					"config": {"limits": {"cpu": {"amount": 1.5, "format": "cores"}, "memory": {"amount": 3072, "format": "MiB"}}, "requests": {"cpu": {"amount": 0.5, "format": "cores"}, "memory": {"amount": 1024, "format": "MiB"}}},
					"variation": {"limits": {"cpu": {"amount": -0.5, "format": "cores"}, "memory": {"amount": -1024, "format": "MiB"}}, "requests": {"cpu": {"amount": -0.5, "format": "cores"}, "memory": {"amount": -1024, "format": "MiB"}}}
				},
				"performance": {
					"config": {"limits": {"cpu": {"amount": 3.0, "format": "cores"}, "memory": {"amount": 8192, "format": "MiB"}}, "requests": {"cpu": {"amount": 2.0, "format": "cores"}, "memory": {"amount": 4096, "format": "MiB"}}},
					"variation": {"limits": {"cpu": {"amount": 1.0, "format": "cores"}, "memory": {"amount": 4096, "format": "MiB"}}, "requests": {"cpu": {"amount": 1.0, "format": "cores"}, "memory": {"amount": 2048, "format": "MiB"}}}
				}
			}
		},
		"medium_term": {
			"duration_in_hours": 168,
			"monitoring_start_time": "2024-01-08T00:00:00.000Z",
			"recommendation_engines": {
				"cost": {
					"config": {"limits": {"cpu": {"amount": 1.2, "format": "cores"}, "memory": {"amount": 2560, "format": "MiB"}}, "requests": {"cpu": {"amount": 0.4, "format": "cores"}, "memory": {"amount": 800, "format": "MiB"}}},
					"variation": {"limits": {"cpu": {"amount": -0.8, "format": "cores"}, "memory": {"amount": -1536, "format": "MiB"}}, "requests": {"cpu": {"amount": -0.6, "format": "cores"}, "memory": {"amount": -1248, "format": "MiB"}}}
				},
				"performance": {
					"config": {"limits": {"cpu": {"amount": 2.5, "format": "cores"}, "memory": {"amount": 6144, "format": "MiB"}}, "requests": {"cpu": {"amount": 1.5, "format": "cores"}, "memory": {"amount": 3072, "format": "MiB"}}},
					"variation": {"limits": {"cpu": {"amount": 0.5, "format": "cores"}, "memory": {"amount": 2048, "format": "MiB"}}, "requests": {"cpu": {"amount": 0.5, "format": "cores"}, "memory": {"amount": 1024, "format": "MiB"}}}
				}
			}
		},
		"long_term": {}
	}
}`

func TestGenerateCSVRows_DeterministicOrder(t *testing.T) {
	rec := model.RecommendationSetResult{
		ID:              "test-id",
		ClusterUUID:     "cluster-uuid",
		ClusterAlias:    "cluster-alias",
		Container:       "my-container",
		Project:         "my-project",
		Workload:        "my-workload",
		WorkloadType:    "Deployment",
		LastReported:    "2024-01-15",
		SourceID:        "src-1",
		Recommendations: datatypes.JSON(testRecommendationJSON),
	}

	first, err := GenerateCSVRows(rec)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("expected non-empty rows")
	}

	for i := 0; i < 20; i++ {
		again, err := GenerateCSVRows(rec)
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if diff := cmp.Diff(first, again); diff != "" {
			t.Fatalf("iteration %d: rows differ (-first +again):\n%s", i, diff)
		}
	}
}

func TestGenerateCSVRows_TermOrdering(t *testing.T) {
	rec := model.RecommendationSetResult{
		ID:              "test-id",
		ClusterUUID:     "cluster-uuid",
		ClusterAlias:    "cluster-alias",
		Container:       "c",
		Project:         "p",
		Workload:        "w",
		WorkloadType:    "Deployment",
		LastReported:    "2024-01-15",
		SourceID:        "src-1",
		Recommendations: datatypes.JSON(testRecommendationJSON),
	}

	rows, err := GenerateCSVRows(rec)
	if err != nil {
		t.Fatal(err)
	}

	// short_term has cost+performance (2 rows), medium_term has cost+performance (2 rows), long_term has none.
	// Expected order: short_term/cost, short_term/performance, medium_term/cost, medium_term/performance
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}

	// Column index 18 = termName, column index 21 = recommendationType
	expectedOrder := [][2]string{
		{"short_term", "cost"},
		{"short_term", "performance"},
		{"medium_term", "cost"},
		{"medium_term", "performance"},
	}

	for i, exp := range expectedOrder {
		if rows[i][18] != exp[0] || rows[i][21] != exp[1] {
			t.Errorf("row %d: got term=%q engine=%q, want term=%q engine=%q",
				i, rows[i][18], rows[i][21], exp[0], exp[1])
		}
	}
}

// injectTestJSON is minimal: one term/engine with variation limits + requests (raw units before inject).
const injectTestJSON = `{
	"recommendation_terms": {
		"short_term": {
			"recommendation_engines": {
				"cost": {
					"variation": {
						"limits": {"cpu": {"amount": -1.0, "format": "cores"}},
						"requests": {"cpu": {"amount": 0.1, "format": "cores"}, "memory": {"amount": 512, "format": "bytes"}}
					}
				}
			}
		}
	}
}`

func TestInjectStoredRequestVariationPct(t *testing.T) {
	t.Run("writes requests from stored pcts and sets format percent", func(t *testing.T) {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(injectTestJSON), &data); err != nil {
			t.Fatal(err)
		}
		pcts := &model.StoredVariationPcts{
			CPUVariationShortCostPct:    float64Ptr(12.5),
			MemoryVariationShortCostPct: float64Ptr(3.25),
		}
		out := injectStoredRequestVariationPct(data, pcts)
		v := out["recommendation_terms"].(map[string]interface{})["short_term"].(map[string]interface{})["recommendation_engines"].(map[string]interface{})["cost"].(map[string]interface{})["variation"].(map[string]interface{})
		req := v["requests"].(map[string]interface{})
		lim := v["limits"].(map[string]interface{})

		if got := req["cpu"].(map[string]interface{})["amount"]; got != 12.5 {
			t.Fatalf("requests.cpu.amount: got %v, want 12.5", got)
		}
		if got := req["cpu"].(map[string]interface{})["format"]; got != "percent" {
			t.Fatalf("requests.cpu.format: got %v, want percent", got)
		}
		if got := req["memory"].(map[string]interface{})["amount"]; got != 3.25 {
			t.Fatalf("requests.memory.amount: got %v, want 3.25", got)
		}
		if got := req["memory"].(map[string]interface{})["format"]; got != "percent" {
			t.Fatalf("requests.memory.format: got %v, want percent", got)
		}
		if got := lim["cpu"].(map[string]interface{})["amount"]; got != -1.0 {
			t.Fatalf("limits.cpu.amount should be unchanged: got %v", got)
		}
	})

	t.Run("skips field when stored pointer is nil", func(t *testing.T) {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(injectTestJSON), &data); err != nil {
			t.Fatal(err)
		}
		pcts := &model.StoredVariationPcts{
			CPUVariationShortCostPct:    float64Ptr(9.0),
			MemoryVariationShortCostPct: nil,
		}
		out := injectStoredRequestVariationPct(data, pcts)
		req := out["recommendation_terms"].(map[string]interface{})["short_term"].(map[string]interface{})["recommendation_engines"].(map[string]interface{})["cost"].(map[string]interface{})["variation"].(map[string]interface{})["requests"].(map[string]interface{})

		if got := req["cpu"].(map[string]interface{})["amount"]; got != 9.0 {
			t.Fatalf("cpu: got %v, want 9", got)
		}
		// memory not overwritten
		if got := req["memory"].(map[string]interface{})["amount"]; got != 512.0 {
			t.Fatalf("memory amount: got %v, want 512 (unchanged)", got)
		}
		if got := req["memory"].(map[string]interface{})["format"]; got != "bytes" {
			t.Fatalf("memory format: got %v, want bytes", got)
		}
	})
}
