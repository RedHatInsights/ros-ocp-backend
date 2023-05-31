package workload

import "database/sql/driver"

type WorkloadType string

const (
	Daemonset             WorkloadType = "daemonset"
	Deployment            WorkloadType = "deployment"
	Deploymentconfig      WorkloadType = "deploymentconfig"
	Replicaset            WorkloadType = "replicaset"
	Replicationcontroller WorkloadType = "replicationcontroller"
	Statefulset           WorkloadType = "statefulset"
)

func (p *WorkloadType) Scan(value interface{}) error {
	*p = WorkloadType(value.(string))
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
		return "statefulsets"
	case Daemonset:
		return "daemonset"
	}
	return "unknown"
}
