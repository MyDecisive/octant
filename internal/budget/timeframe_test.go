package budget

import (
	"testing"
	"time"

	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	budgetdata "github.com/mydecisive/octant/internal/budget/data"
	"github.com/stretchr/testify/assert"
)

func TestValidTimeframe(t *testing.T) {
	t.Parallel()

	cases := []struct {
		des      string
		in       time.Time
		expected budgetv1alpha.Timeframe
	}{
		{
			"no valid",
			time.Now(),
			budgetv1alpha.Timeframe_TIMEFRAME_UNSPECIFIED,
		},
		{
			"24h",
			time.Now().Add(-budgetdata.DayInHR * time.Hour),
			budgetv1alpha.Timeframe_TIMEFRAME_24HR,
		},
		{
			"month to date",
			time.Now().Add(-budgetdata.MonthInHR * time.Hour),
			budgetv1alpha.Timeframe_TIMEFRAME_MTD,
		},
		{
			"last month",
			time.Now().Add(-budgetdata.LastMonthInHR * time.Hour),
			budgetv1alpha.Timeframe_TIMEFRAME_LM,
		},
	}
	for _, tt := range cases {
		t.Run(tt.des, func(t *testing.T) {
			t.Parallel()
			actual := ValidTimeframe(tt.in)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
