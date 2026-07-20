package common

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	rootcommon "github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const (
	RequestDebugModeOff       = "off"
	RequestDebugModeErrorOnly = "error_only"
	RequestDebugModeAlways    = "always"

	defaultRequestDebugMaxBodyBytes = 32 * 1024
	maxRequestDebugStringValueBytes = 64
)

type RequestDebugSnapshotInput struct {
	Mode         string
	RequestPath  string
	RelayMode    int
	ContentType  string
	Downstream   []byte
	Upstream     []byte
	MaxBodyBytes int
}

type RequestDebugSnapshot struct {
	Mode        string            `json:"mode"`
	RequestPath string            `json:"request_path,omitempty"`
	RelayMode   int               `json:"relay_mode,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
	Error       string            `json:"request_debug_error,omitempty"`
	Downstream  *RequestDebugBody `json:"downstream,omitempty"`
	Upstream    *RequestDebugBody `json:"upstream,omitempty"`
}

type RequestDebugBody struct {
	Size      int    `json:"size"`
	SHA256    string `json:"sha256"`
	Truncated bool   `json:"truncated"`
	Body      string `json:"body"`
}

func ShouldAttachRequestDebug(mode string, success bool) bool {
	switch normalizeRequestDebugMode(mode) {
	case RequestDebugModeAlways:
		return true
	case RequestDebugModeErrorOnly:
		return !success
	default:
		return false
	}
}

func BuildRequestDebugSnapshot(input RequestDebugSnapshotInput) *RequestDebugSnapshot {
	mode := normalizeRequestDebugMode(input.Mode)
	if mode == RequestDebugModeOff {
		return nil
	}
	maxBytes := input.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = defaultRequestDebugMaxBodyBytes
	}
	snapshot := &RequestDebugSnapshot{
		Mode:        mode,
		RequestPath: input.RequestPath,
		RelayMode:   input.RelayMode,
		ContentType: input.ContentType,
	}
	if input.Downstream != nil {
		snapshot.Downstream = buildRequestDebugBody(input.Downstream, maxBytes)
	}
	if input.Upstream != nil {
		snapshot.Upstream = buildRequestDebugBody(input.Upstream, maxBytes)
	}
	return snapshot
}

func CaptureDownstreamRequestDebug(c *gin.Context, info *RelayInfo) {
	if info == nil || normalizeRequestDebugMode(rootcommon.RequestDebugLogging) == RequestDebugModeOff {
		return
	}
	storage, err := rootcommon.GetBodyStorage(c)
	if err != nil {
		setRequestDebugError(c, info, fmt.Sprintf("failed to read downstream body: %v", err))
		return
	}
	data, err := storage.Bytes()
	if err != nil {
		setRequestDebugError(c, info, fmt.Sprintf("failed to read downstream body: %v", err))
		return
	}
	_, _ = storage.Seek(0, io.SeekStart)
	snapshot := BuildRequestDebugSnapshot(requestDebugSnapshotInput(c, info, data, nil))
	if snapshot == nil {
		return
	}
	if info.RequestDebugSnapshot != nil && info.RequestDebugSnapshot.Upstream != nil {
		snapshot.Upstream = info.RequestDebugSnapshot.Upstream
	}
	info.RequestDebugSnapshot = snapshot
}

func CaptureUpstreamRequestDebug(c *gin.Context, info *RelayInfo, upstream []byte) {
	if info == nil || normalizeRequestDebugMode(rootcommon.RequestDebugLogging) == RequestDebugModeOff {
		return
	}
	snapshot := BuildRequestDebugSnapshot(requestDebugSnapshotInput(c, info, nil, upstream))
	if snapshot == nil {
		return
	}
	if info.RequestDebugSnapshot != nil && info.RequestDebugSnapshot.Downstream != nil {
		snapshot.Downstream = info.RequestDebugSnapshot.Downstream
	}
	info.RequestDebugSnapshot = snapshot
}

func CaptureUpstreamRequestDebugFromStorage(c *gin.Context, info *RelayInfo) {
	if info == nil || normalizeRequestDebugMode(rootcommon.RequestDebugLogging) == RequestDebugModeOff {
		return
	}
	storage, err := rootcommon.GetBodyStorage(c)
	if err != nil {
		setRequestDebugError(c, info, fmt.Sprintf("failed to read upstream body: %v", err))
		return
	}
	data, err := storage.Bytes()
	if err != nil {
		setRequestDebugError(c, info, fmt.Sprintf("failed to read upstream body: %v", err))
		return
	}
	CaptureUpstreamRequestDebug(c, info, data)
}

func setRequestDebugError(c *gin.Context, info *RelayInfo, message string) {
	if info.RequestDebugSnapshot == nil {
		info.RequestDebugSnapshot = BuildRequestDebugSnapshot(requestDebugSnapshotInput(c, info, nil, nil))
	}
	if info.RequestDebugSnapshot != nil {
		info.RequestDebugSnapshot.Error = message
	}
}

func requestDebugSnapshotInput(c *gin.Context, info *RelayInfo, downstream []byte, upstream []byte) RequestDebugSnapshotInput {
	requestPath := ""
	relayMode := 0
	if info != nil {
		requestPath = info.RequestURLPath
		relayMode = info.RelayMode
	}
	contentType := ""
	if c != nil && c.Request != nil {
		contentType = c.Request.Header.Get("Content-Type")
		if requestPath == "" && c.Request.URL != nil {
			requestPath = c.Request.URL.Path
		}
	}
	return RequestDebugSnapshotInput{
		Mode:         rootcommon.RequestDebugLogging,
		RequestPath:  requestPath,
		RelayMode:    relayMode,
		ContentType:  contentType,
		Downstream:   downstream,
		Upstream:     upstream,
		MaxBodyBytes: rootcommon.RequestDebugMaxBodyBytes,
	}
}

func normalizeRequestDebugMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case RequestDebugModeAlways:
		return RequestDebugModeAlways
	case RequestDebugModeErrorOnly:
		return RequestDebugModeErrorOnly
	default:
		return RequestDebugModeOff
	}
}

func buildRequestDebugBody(data []byte, maxBytes int) *RequestDebugBody {
	sanitized, truncated := sanitizeRequestDebugBody(data)
	body := string(sanitized)
	if maxBytes > 0 && len(sanitized) > maxBytes {
		body = string(sanitized[:maxBytes]) + fmt.Sprintf("...[TRUNCATED %d/%d bytes]", maxBytes, len(sanitized))
		truncated = true
	}
	sum := sha256.Sum256(data)
	return &RequestDebugBody{
		Size:      len(data),
		SHA256:    hex.EncodeToString(sum[:]),
		Truncated: truncated,
		Body:      body,
	}
}

func sanitizeRequestDebugBody(data []byte) ([]byte, bool) {
	var value any
	if err := rootcommon.Unmarshal(data, &value); err != nil {
		return data, false
	}
	sanitized, truncated := sanitizeRequestDebugValue(value)
	result, err := rootcommon.Marshal(sanitized)
	if err != nil {
		return data, false
	}
	return result, truncated
}

func sanitizeRequestDebugValue(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		truncated := false
		for key, item := range typed {
			if isRequestDebugSecretKey(key) {
				result[key] = "[REDACTED]"
				truncated = true
				continue
			}
			var childTruncated bool
			if isRequestDebugTextKey(key) {
				result[key], childTruncated = sanitizeRequestDebugTextValue(item)
			} else {
				result[key], childTruncated = sanitizeRequestDebugValue(item)
			}
			truncated = truncated || childTruncated
		}
		return result, truncated
	case []any:
		result := make([]any, len(typed))
		truncated := false
		for i, item := range typed {
			var childTruncated bool
			result[i], childTruncated = sanitizeRequestDebugValue(item)
			truncated = truncated || childTruncated
		}
		return result, truncated
	case string:
		if len(typed) > maxRequestDebugStringValueBytes {
			return typed[:maxRequestDebugStringValueBytes] + fmt.Sprintf("...[TRUNCATED string %d bytes]", len(typed)), true
		}
		return typed, false
	default:
		return value, false
	}
}

func sanitizeRequestDebugTextValue(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		truncated := false
		for key, item := range typed {
			if isRequestDebugSecretKey(key) {
				result[key] = "[REDACTED]"
				truncated = true
				continue
			}
			var childTruncated bool
			if isRequestDebugTextKey(key) {
				result[key], childTruncated = sanitizeRequestDebugTextValue(item)
			} else {
				result[key], childTruncated = sanitizeRequestDebugValue(item)
			}
			truncated = truncated || childTruncated
		}
		return result, truncated
	case []any:
		result := make([]any, len(typed))
		truncated := false
		for i, item := range typed {
			var childTruncated bool
			result[i], childTruncated = sanitizeRequestDebugTextValue(item)
			truncated = truncated || childTruncated
		}
		return result, truncated
	case string:
		if typed == "" {
			return typed, false
		}
		return fmt.Sprintf("[TRUNCATED text %d bytes]", len(typed)), true
	default:
		return sanitizeRequestDebugValue(value)
	}
}

func isRequestDebugSecretKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "authorization", "api_key", "apikey", "access_token", "refresh_token", "key", "token", "password", "secret":
		return true
	default:
		return false
	}
}

func isRequestDebugTextKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "content", "input", "instructions", "prompt", "suffix", "system", "text":
		return true
	default:
		return false
	}
}
