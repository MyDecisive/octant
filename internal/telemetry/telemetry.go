package telemetry

import (
	"encoding/json"
	"fmt"
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
