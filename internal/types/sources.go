package types

type SourcesEvent struct {
	Id                  int    `validate:"required"`
	Source_id           int    `validate:"required"`
	Application_type_id int    `validate:"required"`
	Tenant              string `validate:"required"`
}
