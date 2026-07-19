package common

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestDebugAttachModes(t *testing.T) {
	assert.False(t, ShouldAttachRequestDebug("", false))
	assert.False(t, ShouldAttachRequestDebug("off", true))
	assert.False(t, ShouldAttachRequestDebug("error_only", true))
	assert.True(t, ShouldAttachRequestDebug("error_only", false))
	assert.True(t, ShouldAttachRequestDebug("always", true))
	assert.True(t, ShouldAttachRequestDebug("always", false))
}

func TestBuildRequestDebugSnapshotRedactsSecretsBeforeTruncation(t *testing.T) {
	body := []byte(`{"model":"gpt-test","api_key":"sk-secret","messages":[{"role":"user","content":"hello"}],"nested":{"token":"secret-token"},"image_url":{"url":"data:image/png;base64,` + strings.Repeat("A", 200) + `"}}`)

	snapshot := BuildRequestDebugSnapshot(RequestDebugSnapshotInput{
		Mode:         "always",
		RequestPath:  "/v1/chat/completions",
		RelayMode:    2,
		ContentType:  "application/json",
		Downstream:   body,
		Upstream:     body,
		MaxBodyBytes: 160,
	})

	require.NotNil(t, snapshot.Downstream)
	assert.Equal(t, len(body), snapshot.Downstream.Size)
	assert.Equal(t, sha256Hex(body), snapshot.Downstream.SHA256)
	assert.True(t, snapshot.Downstream.Truncated)
	assert.NotContains(t, snapshot.Downstream.Body, "sk-secret")
	assert.NotContains(t, snapshot.Downstream.Body, "secret-token")
	assert.NotContains(t, snapshot.Downstream.Body, strings.Repeat("A", 80))
	assert.Contains(t, snapshot.Downstream.Body, "[REDACTED]")
	assert.Contains(t, snapshot.Downstream.Body, "[TRUNCATED")

	require.NotNil(t, snapshot.Upstream)
	assert.Equal(t, snapshot.Downstream.Body, snapshot.Upstream.Body)
}

func TestCaptureRequestDebugSnapshotsFromBodyStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	downstream := []byte(`{"model":"client","api_key":"sk-client"}`)
	upstream := []byte(`{"model":"upstream","api_key":"sk-upstream"}`)
	storage, err := rootcommon.CreateBodyStorage(downstream)
	require.NoError(t, err)
	defer storage.Close()

	ctx := &gin.Context{Request: httptest.NewRequest("POST", "/v1/chat/completions", nil)}
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set(rootcommon.KeyBodyStorage, storage)
	info := &RelayInfo{RelayMode: 2, RequestURLPath: "/v1/chat/completions"}

	oldMode := rootcommon.RequestDebugLogging
	oldMax := rootcommon.RequestDebugMaxBodyBytes
	rootcommon.RequestDebugLogging = "always"
	rootcommon.RequestDebugMaxBodyBytes = 1024
	defer func() {
		rootcommon.RequestDebugLogging = oldMode
		rootcommon.RequestDebugMaxBodyBytes = oldMax
	}()

	CaptureDownstreamRequestDebug(ctx, info)
	CaptureUpstreamRequestDebug(ctx, info, upstream)

	require.NotNil(t, info.RequestDebugSnapshot)
	require.NotNil(t, info.RequestDebugSnapshot.Downstream)
	require.NotNil(t, info.RequestDebugSnapshot.Upstream)
	assert.Contains(t, info.RequestDebugSnapshot.Downstream.Body, `"api_key":"[REDACTED]"`)
	assert.Contains(t, info.RequestDebugSnapshot.Upstream.Body, `"api_key":"[REDACTED]"`)

	_, err = storage.Seek(0, io.SeekStart)
	require.NoError(t, err)
	got, err := io.ReadAll(storage)
	require.NoError(t, err)
	assert.Equal(t, downstream, got)
}

func TestCaptureDownstreamRequestDebugRecordsReadFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := &gin.Context{Request: httptest.NewRequest("POST", "/v1/chat/completions", nil)}
	storage, err := rootcommon.CreateBodyStorage([]byte(`{"model":"test"}`))
	require.NoError(t, err)
	require.NoError(t, storage.Close())
	ctx.Set(rootcommon.KeyBodyStorage, storage)
	info := &RelayInfo{}

	oldMode := rootcommon.RequestDebugLogging
	rootcommon.RequestDebugLogging = "always"
	defer func() { rootcommon.RequestDebugLogging = oldMode }()

	CaptureDownstreamRequestDebug(ctx, info)

	require.NotNil(t, info.RequestDebugSnapshot)
	assert.Contains(t, info.RequestDebugSnapshot.Error, "failed to read downstream body")
}

func TestCaptureUpstreamRequestDebugFromStorageCapturesPassThroughBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"pass-through","token":"secret"}`)
	storage, err := rootcommon.CreateBodyStorage(body)
	require.NoError(t, err)
	defer storage.Close()
	ctx := &gin.Context{Request: httptest.NewRequest("POST", "/v1/messages", nil)}
	ctx.Set(rootcommon.KeyBodyStorage, storage)
	info := &RelayInfo{}

	oldMode := rootcommon.RequestDebugLogging
	rootcommon.RequestDebugLogging = "always"
	defer func() { rootcommon.RequestDebugLogging = oldMode }()

	CaptureUpstreamRequestDebugFromStorage(ctx, info)

	require.NotNil(t, info.RequestDebugSnapshot)
	require.NotNil(t, info.RequestDebugSnapshot.Upstream)
	assert.Contains(t, info.RequestDebugSnapshot.Upstream.Body, `"token":"[REDACTED]"`)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
