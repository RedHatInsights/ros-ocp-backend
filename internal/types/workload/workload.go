package workload

import (
	"database/sql/driver"
	"fmt"
)

type WorkloadType string

const (
	Daemonset             WorkloadType = "daemonset"
	Deployment            WorkloadType = "deployment"
	Deploymentconfig      WorkloadType = "deploymentconfig"
	Replicaset            WorkloadType = "replicaset"
	Replicationcontroller WorkloadType = "replicationcontroller"
	Statefulset           WorkloadType = "statefulset"
	Namespace             WorkloadType = "namespace"
)

func (p *WorkloadType) Scan(value interface{}) error {
	if value == nil {
		*p = "" // workload.workload_type is nullable
		return nil
	}
	strVal, ok := value.(string)
	if !ok {
		return fmt.Errorf("WorkloadType.Scan: expected string, got %T", value)
	}
	*p = WorkloadType(strVal)
	return nil
}

func (p WorkloadType) Value() (driver.Value, error) {
	return string(p), nil
}

func (p WorkloadType) String() string {
	switch p {
	case Deployment:
		return "deployment"
	case Deploymentconfig:
		return "deploymentconfig"
	case Replicaset:
		return "replicaset"
	case Replicationcontroller:
		return "replicationcontroller"
	case Statefulset:
		return "statefulset"
	case Daemonset:
		return "daemonset"
	case Namespace:
		return "namespace"
	}
	return "unknown"
}
