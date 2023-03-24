package types

type RbacResponse struct {
	Meta  interface{}
	Links rbacLinks
	Data  []RbacData
}

type RbacData struct {
	ResourceDefinitions []rbacResourceDefinitions `json:"resourceDefinitions,omitempty"`
	Permission          string
}

type rbacResourceDefinitions struct {
	AttributeFilter attributeFilter `json:"attributeFilter,omitempty"`
}

type attributeFilter struct {
	Key       string
	Value     []string
	Operation string
}

type rbacLinks struct {
	First    string
	Next     string `json:"next,omitempty"`
	Previous string `json:"previous,omitempty"`
	Last     string
}

type AggregatedPermissions struct {
	ResourceType string
	Resources    []string
}
