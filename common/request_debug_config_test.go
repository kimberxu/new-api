package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitRequestDebugConfigFromEnvDefaultsOff(t *testing.T) {
	t.Setenv("REQUEST_DEBUG_LOGGING", "")
	t.Setenv("REQUEST_DEBUG_MAX_BODY_BYTES", "")

	RequestDebugLogging = "always"
	RequestDebugMaxBodyBytes = 10
	initRequestDebugConfigFromEnv()

	assert.Equal(t, "off", RequestDebugLogging)
	assert.Equal(t, 32768, RequestDebugMaxBodyBytes)
}

func TestInitRequestDebugConfigFromEnvAcceptsModesAndMaxBytes(t *testing.T) {
	t.Setenv("REQUEST_DEBUG_LOGGING", "error_only")
	t.Setenv("REQUEST_DEBUG_MAX_BODY_BYTES", "4096")

	initRequestDebugConfigFromEnv()

	assert.Equal(t, "error_only", RequestDebugLogging)
	assert.Equal(t, 4096, RequestDebugMaxBodyBytes)
}

func TestInitRequestDebugConfigFromEnvRejectsInvalidMode(t *testing.T) {
	t.Setenv("REQUEST_DEBUG_LOGGING", "yes")
	t.Setenv("REQUEST_DEBUG_MAX_BODY_BYTES", "-1")

	initRequestDebugConfigFromEnv()

	assert.Equal(t, "off", RequestDebugLogging)
	assert.Equal(t, 32768, RequestDebugMaxBodyBytes)
}
