package rbac

import (
	"fmt"

	"github.com/redhatinsights/ros-ocp-backend/internal/config"
	"github.com/redhatinsights/ros-ocp-backend/internal/utils"
	"gorm.io/gorm"
)

type ResourceType string

const (
	ResourceContainer ResourceType = "container"
	ResourceProject   ResourceType = "namespace"
)

func AddRBACFilter(query *gorm.DB, userPermissions map[string][]string, resourceType ResourceType) error {
	cfg := config.GetConfig()
	if !cfg.RBACEnabled {
		return nil
	}

	// Validate resource type
	switch resourceType {
	case ResourceContainer, ResourceProject:
		// valid supported type
	default:
		return fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	// Global wildcard -> full access
	if _, ok := userPermissions["*"]; ok {
		return nil
	}

	clusterPerms, hasCluster := userPermissions["openshift.cluster"]
	projectPerms, hasProject := userPermissions["openshift.project"]

	clusterAll := hasCluster && utils.StringInSlice("*", clusterPerms)
	projectAll := hasProject && utils.StringInSlice("*", projectPerms)

	applyClusterFilter := func() {
		switch resourceType {
		case ResourceContainer, ResourceProject:
			query.Where("clusters.cluster_uuid IN (?)", clusterPerms)
		}
	}

	applyProjectFilter := func() {
		switch resourceType {
		case ResourceContainer:
			query.Where("workloads.namespace IN (?)", projectPerms)
		case ResourceProject:
			query.Where("namespace_recommendation_sets.namespace_name IN (?)", projectPerms)
		}
	}

	// Both cluster + project permissions explicitly set
	if hasCluster && hasProject {
		switch {
		case clusterAll && projectAll:
			return nil

		case clusterAll:
			applyProjectFilter()
			return nil

		case projectAll:
			applyClusterFilter()
			return nil

		default:
			applyClusterFilter()
			applyProjectFilter()
			return nil
		}
	}

	// Cluster-only permission -> access all projects in those clusters
	if hasCluster && !hasProject {
		if !clusterAll {
			applyClusterFilter()
		}
		return nil
	}

	// Project-only permission -> access project across all clusters
	if hasProject && !hasCluster {
		if !projectAll {
			applyProjectFilter()
		}
		return nil
	}

	return nil
}
