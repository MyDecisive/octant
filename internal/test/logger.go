package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

type LogOutput struct {
	Level   zapcore.Level
	Message string
	Fields  map[string]any
}

// NewTestLogger wraps a zaptest logger to tee output and provide ObservedLogs to validate in tests.
func NewTestLogger(t *testing.T, level zapcore.LevelEnabler) (*zap.Logger, *observer.ObservedLogs) {
	t.Helper()
	// capture log output at the provided level, but tee to warn level and above.
	testCore, logOutput := observer.New(level)
	testLogger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		theCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
			zaptest.NewTestingWriter(t),
			zapcore.WarnLevel,
		)
		return zapcore.NewTee(theCore, testCore)
	})))
	return testLogger, logOutput
}

// ValidateOutput takes the provided ObservedLogs and validates all the provided LogOutput entries.
func ValidateOutput(t *testing.T, logOutput *observer.ObservedLogs, toValidate []LogOutput) {
	t.Helper()
	for _, recordToValidate := range toValidate {
		filtered := logOutput.FilterMessage(recordToValidate.Message).All()
		require.Len(t, filtered, 1, "unable to find log message: %s", recordToValidate.Message)
		assert.Equal(t, recordToValidate.Level, filtered[0].Level)

		// validate fields
		actualFields := filtered[0].ContextMap()
		for key, val := range recordToValidate.Fields {
			require.Contains(t, actualFields, key, "field '%s' not found on log entry", key)
			assert.Equal(t, val, actualFields[key], "value for field '%s' didn't match", key)
		}
	}
}
