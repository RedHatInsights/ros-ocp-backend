package listoptions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSQLOrderByFragment(t *testing.T) {
	tests := []struct {
		name     string
		column   string
		orderHow string
		want     string
	}{
		{
			name:     "container column desc appends NULLS LAST",
			column:   ContainerAllowedOrderBy["container"],
			orderHow: OrderDesc,
			want:     "recommendation_sets.container_name desc NULLS LAST",
		},
		{
			name:     "container column asc omits NULLS LAST",
			column:   ContainerAllowedOrderBy["container"],
			orderHow: OrderAsc,
			want:     "recommendation_sets.container_name asc",
		},
		{
			name:     "container cpu_request_current desc appends NULLS LAST",
			column:   ContainerAllowedOrderBy["cpu_request_current"],
			orderHow: OrderDesc,
			want:     "recommendation_sets.cpu_request_current desc NULLS LAST",
		},
		{
			name:     "container cpu variation desc appends NULLS LAST",
			column:   ContainerAllowedOrderBy["cpu_variation_short_cost"],
			orderHow: OrderDesc,
			want:     "recommendation_sets.cpu_variation_short_cost_pct desc NULLS LAST",
		},
		{
			name:     "container cpu variation asc omits NULLS LAST",
			column:   ContainerAllowedOrderBy["cpu_variation_short_cost"],
			orderHow: OrderAsc,
			want:     "recommendation_sets.cpu_variation_short_cost_pct asc",
		},
		{
			name:     "container memory variation desc appends NULLS LAST",
			column:   ContainerAllowedOrderBy["memory_variation_long_performance"],
			orderHow: OrderDesc,
			want:     "recommendation_sets.memory_variation_long_performance_pct desc NULLS LAST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SQLOrderByFragment(tt.column, tt.orderHow)
			assert.Equal(t, tt.want, got)
		})
	}
}
