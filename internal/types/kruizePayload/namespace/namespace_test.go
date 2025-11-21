package namespace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestUnmarshalCreateNamespaceExperiment(t *testing.T) {
	jsonPath := filepath.Join("..", "..", "..", "..", "scripts", "samples", "namespace_create_experiment.json")
	createExperimentJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read JSON file: %v", err)
	}

	var experiments []CreateNamespaceExperiment
	err = json.Unmarshal(createExperimentJSON, &experiments)
	if err != nil {
		t.Fatalf("failed to read JSON file: %v", err)
	}

	if len(experiments) == 0 {
		t.Fatal("expected at least one experiment, got 0")
	}

	exp := experiments[0]

	if exp.Version != "v2.0" {
		t.Fatalf("expected version 'v2.0', got '%s'", exp.Version)
	}

	if exp.ExperimentName != "ros-namespace-experiment-demo" {
		t.Fatalf("expected experiment_name 'ros-namespace-experiment-demo', got '%s'", exp.ExperimentName)
	}

	if exp.ClusterName != "cluster-one-division-bell" {
		t.Fatalf("expected cluster_name 'cluster-one-division-bell', got '%s'", exp.ClusterName)
	}

	if exp.PerformanceProfile != "resource-optimization-openshift" {
		t.Fatalf("expected performance_profile 'resource-optimization-openshift', got '%s'", exp.PerformanceProfile)
	}

	if exp.Mode != "monitor" {
		t.Fatalf("expected mode 'monitor', got '%s'", exp.Mode)
	}

	if exp.TargetCluster != "remote" {
		t.Fatalf("expected target_cluster 'remote', got '%s'", exp.TargetCluster)
	}

	if len(exp.KubernetesObjects) == 0 {
		t.Fatal("expected at least one kubernetes object, got 0")
	}

	k8sObj := exp.KubernetesObjects[0]

	if k8sObj.Namespaces.Namespace != "namespace-demo" {
		t.Fatalf("expected namespace 'namespace-demo', got '%s'", k8sObj.Namespaces.Namespace)
	}

	if exp.TrialSettings.Measurement_duration != "15min" {
		t.Fatalf("expected trial_settings.measurement_duration '15min', got '%s'", exp.TrialSettings.Measurement_duration)
	}

	if exp.RecommendationSettings.Threshold != "0.1" {
		t.Fatalf("expected recommendation_settings.threshold '0.1', got '%s'", exp.RecommendationSettings.Threshold)
	}

}

func TestUnmarshalNamespaceUpdateResult(t *testing.T) {
	jsonPath := filepath.Join("..", "..", "..", "..", "scripts", "samples", "namespace_update_result.json")
	updateResultJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read JSON file: %v", err)
	}

	var response []UpdateNamespaceResult
	err = json.Unmarshal(updateResultJSON, &response)
	if err != nil {
		t.Fatalf("expected unmarshal to fail or have issues: %v", err)
		return
	}

	result := response[0]

	if result.Version != "v2.0" {
		t.Fatalf("expected version 'v2.0', got '%s'", result.Version)
	}

	if result.ExperimentName != "ros-namespace-experiment-demo" {
		t.Fatalf("expected experiment_name 'ros-namespace-experiment-demo', got '%s'", result.ExperimentName)
	}

	if len(result.KubernetesObjects) == 0 {
		t.Fatal("expected at least one kubernetes object, got 0")
	}

	k8sObj := result.KubernetesObjects[0]

	if k8sObj.Namespaces.Namespace != "namespace-demo" {
		t.Fatalf("expected namespace 'namespace-demo', got '%s'", k8sObj.Namespaces.Namespace)
	}
}

func TestUnmarshalNamespaceRecommendationResponse(t *testing.T) {
	jsonPath := filepath.Join("..", "..", "..", "..", "scripts", "samples", "namespace_recommendation.json")
	recommendationJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("failed to read JSON file: %v", err)
	}

	var response NamespaceRecommendationResponse
	err = json.Unmarshal(recommendationJSON, &response)
	if err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	exp := response[0]

	if exp.ClusterName != "cluster-one-division-bell" {
		t.Fatalf("expected cluster_name 'cluster-one-division-bell', got '%s'", exp.ClusterName)
	}

	if exp.ExperimentType != "namespace" {
		t.Fatalf("expected experiment_type 'namespace', got '%s'", exp.ExperimentType)
	}

	if exp.Version != "v2.0" {
		t.Fatalf("expected version 'v2.0', got '%s'", exp.Version)
	}

	if exp.ExperimentName != "ros-namespace-experiment-demo" {
		t.Fatalf("expected experiment_name 'ros-namespace-experiment-demo', got '%s'", exp.ExperimentName)
	}
}
