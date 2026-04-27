package budget

import (
	"time"

	budgetv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/budget/v1alpha"
)

const (
	dayInHR   = 24 * time.Hour  // 1 day
	monthInHR = 730 * time.Hour // 30 days (i.e., closest approx. to a month)
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
	if delta < dayInHR {
		return budgetv1alpha.Timeframe_TIMEFRAME_UNSPECIFIED
	}

	if delta < monthInHR {
		return budgetv1alpha.Timeframe_TIMEFRAME_24HR
	}

	if delta < 2*monthInHR {
		return budgetv1alpha.Timeframe_TIMEFRAME_MTD
	}

	return budgetv1alpha.Timeframe_TIMEFRAME_LM
}
