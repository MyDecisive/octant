package telemetry

import (
	"encoding/json"
	"fmt"

	octantv1alpha "github.com/MyDecisive/octant-contracts/go/pkg/octant/v1alpha"
)

type MLT string

const (
	Metrics MLT = "metrics"
	Logs    MLT = "logs"
	Traces  MLT = "traces"
)

func (t *MLT) UnmarshalJSON(b []byte) error {
	var val string
	if err := json.Unmarshal(b, &val); err != nil {
		return err
	}

	telemetry := MLT(val)
	switch telemetry {
	case Metrics, Logs, Traces:
		*t = telemetry
		return nil
	}
	return fmt.Errorf("invalid telemetry type: %s", val)
}

// ToMLTs converts list of MLTType to list of MLT.
func ToMLTs(val []octantv1alpha.MLTType) []MLT {
	var telemetries []MLT
	for _, t := range val {
		switch t {
		case octantv1alpha.MLTType_MLT_TYPE_METRIC:
			telemetries = append(telemetries, Metrics)
		case octantv1alpha.MLTType_MLT_TYPE_TRACE:
			telemetries = append(telemetries, Traces)
		case octantv1alpha.MLTType_MLT_TYPE_LOG:
			telemetries = append(telemetries, Logs)
		}
	}
	return telemetries
}
