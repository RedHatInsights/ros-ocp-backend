package types

type RbacResponse struct {
	Meta  interface{}
	Links rbacLinks
	Data  []RbacData
}

type RbacData struct {
	ResourceDefinitions []RbacResourceDefinitions `json:"resourceDefinitions,omitempty"`
	Permission          string
}

type RbacResourceDefinitions struct {
	AttributeFilter AttributeFilter `json:"attributeFilter,omitempty"`
}

type AttributeFilter struct {
	Key       string
	Value     interface{}
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
