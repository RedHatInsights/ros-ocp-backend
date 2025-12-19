package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	database "github.com/redhatinsights/ros-ocp-backend/internal/db"
	"github.com/redhatinsights/ros-ocp-backend/internal/model"
	"github.com/redhatinsights/ros-ocp-backend/internal/types"
	"github.com/redhatinsights/ros-ocp-backend/internal/types/workload"
)

func TestNamespaceRecommendationSuccess(t *testing.T) {

	mockRecommResponseFromKruize := `[{
		"cluster_name": "unittest-cluster",
		"experiment_type": "namespace",
		"experiment_name": "unittest-ros-namespace-experiment-demo",
		"version": "v2.0",
		"kubernetes_objects": [{
			"namespace": "unittest-namespace-demo",
			"containers": [],
			"namespaces": {
				"namespace": "unittest-namespace-demo",
				"recommendations": {
					"version": "1.0",
					"notifications": {"111000": {"type": "info", "message": "Recommendations Are Available", "code": 111000}},
					"data": {
						"2022-01-23T23:58:43.511Z": {
							"monitoring_end_time": "2022-01-23T23:58:43.511Z",
							"current": {
								"requests": {"memory": {"amount": 400.0, "format": "MiB"}, "cpu": {"amount": 6.0, "format": "cores"}},
								"limits": {"memory": {"amount": 600.0, "format": "MiB"}, "cpu": {"amount": 4.5, "format": "cores"}}
							},
							"recommendation_terms": {
								"short_term": {
									"monitoring_start_time": "2022-01-22T23:58:43.511Z",
									"duration_in_hours": 24.0,
									"recommendation_engines": {
										"cost": {
											"pods_count": 2,
											"config": {"requests": {"memory": {"amount": 216.0}, "cpu": {"amount": 0.93}}},
											"variation": {"requests": {"memory": {"amount": -184.0}, "cpu": {"amount": -5.07}}}
										}
									}
								}
							}
						}
					}
				}
			}
		}]
	}]`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, writeErr := w.Write([]byte(mockRecommResponseFromKruize))
		if writeErr != nil {
			t.Fatalf("failed to write response: %v", writeErr.Error())
		}
	}))
	defer server.Close()

	cfg := config.GetConfig()
	originalURL := cfg.KruizeUrl
	originalDisableNS := cfg.DisableNamespaceRecommendation

	cfg.KruizeUrl = server.URL
	cfg.DisableNamespaceRecommendation = false

	before := testutil.ToFloat64(namespaceRecommendationSuccess)

	db := database.GetDB()
	rhAccount := model.RHAccount{
		Account: "unittest-account",
		OrgId:   "12345",
	}
	if err := db.Create(&rhAccount).Error; err != nil {
		t.Fatalf("failed to create rh_account: %v", err)
	}

	cluster := model.Cluster{
		TenantID:     rhAccount.ID,
		ClusterUUID:  "unittest-cluster-uuid",
		ClusterAlias: "unittest-cluster",
	}
	if err := db.Create(&cluster).Error; err != nil {
		t.Fatalf("failed to create cluster: %v", err)
	}

	wl := model.Workload{
		OrgId:           "12345",
		ClusterID:       cluster.ID,
		Namespace:       "unittest-namespace-demo",
		WorkloadType:    workload.Deployment,
		WorkloadName:    "unittest-workload",
		ExperimentName:  "unittest-ros-namespace-experiment-demo",
		MetricsUploadAt: time.Now(),
	}
	if err := db.Create(&wl).Error; err != nil {
		t.Fatalf("failed to create workload: %v", err)
	}

	// teardown
	defer func() {
		db.Delete(&rhAccount)
		// deleting the cluster will delete all related records
		db.Delete(&cluster)
		// restore config vars post test run
		cfg.KruizeUrl = originalURL
		cfg.DisableNamespaceRecommendation = originalDisableNS
	}()

	msg := types.RecommendationKafkaMsg{
		Request_id: "unittest-request-123",
		Metadata: types.RecommendationMetadata{
			Org_id:             "12345",
			Workload_id:        wl.ID,
			Experiment_name:    "unittest-ros-namespace-experiment-demo",
			Max_endtime_report: time.Date(2022, 1, 23, 23, 58, 43, 511000000, time.UTC),
			ExperimentType:     types.PayloadTypeNamespace,
		},
	}

	requestAndSaveRecommendation(msg, "New")
	after := testutil.ToFloat64(namespaceRecommendationSuccess)
	if after <= before {
		t.Errorf("expected namespaceRecommendationSuccess to increment, before: %f, after: %f", before, after)
	}
}
