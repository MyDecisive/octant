package connection

import (
	"encoding/json"
	"fmt"
)

type Telemetry string

const (
	Metrics Telemetry = "metrics"
	Logs    Telemetry = "logs"
	Traces  Telemetry = "traces"
)

func (t *Telemetry) UnmarshalJSON(b []byte) error {
	var val string
	if err := json.Unmarshal(b, &val); err != nil {
		return err
	}

	telemetry := Telemetry(val)
	switch telemetry {
	case Metrics, Logs, Traces:
		*t = telemetry
		return nil
	}
	return fmt.Errorf("invalid telemetry type: %s", val)
}
