package budget

import (
	"time"

	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
	budgetdata "github.com/mydecisive/octant/internal/budget/data"
)

// ValidTimeframe returns the timeframe that represents latest timeframe with data.
//
// e.g., if this returns `Timeframe_TIMEFRAME_MTD`, then that means both
// `Timeframe_TIMEFRAME_24HR` and `Timeframe_TIMEFRAME_MTD` have data.
// But `Timeframe_TIMEFRAME_LM` doesn't.
//
// e.g., if this returns `Timeframe_TIMEFRAME_UNSPECIFIED`, then that means no timeframe have data.
func ValidTimeframe(creationTime time.Time) budgetv1alpha.Timeframe {
	delta := time.Since(creationTime)
	if delta < 0 {
		return budgetv1alpha.Timeframe_TIMEFRAME_UNSPECIFIED
	}

	if delta <= budgetdata.MonthInHR*time.Hour {
		return budgetv1alpha.Timeframe_TIMEFRAME_MTD
	}

	return budgetv1alpha.Timeframe_TIMEFRAME_LM
}
