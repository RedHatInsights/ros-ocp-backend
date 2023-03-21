package workload

import "database/sql/driver"

type WorkloadType string

const (
	Deployment            WorkloadType = "deployment"
	Deploymentconfig      WorkloadType = "deploymentconfig"
	Replicaset            WorkloadType = "replicaset"
	Replicationcontroller WorkloadType = "replicationcontroller"
	Statefulsets          WorkloadType = "statefulsets"
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
	case Statefulsets:
		return "statefulsets"
	}
	return "unknown"
}
