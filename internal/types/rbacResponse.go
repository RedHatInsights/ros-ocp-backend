package types

type RbacResponse struct {
	Meta  interface{}
	Links rbacLinks
	Data  []RbacData
}

type RbacData struct {
	ResourceDefinitions interface{}
	Permission          string
}

type rbacLinks struct {
	First    string
	Next     string
	Previous string
	Last     string
}
