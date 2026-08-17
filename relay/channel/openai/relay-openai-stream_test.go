package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errReader 首次 Read 即返回错误，模拟上游 HTTP/2 流中断。
type errReader struct{ err error }

func (r *errReader) Read([]byte) (int, error) { return 0, r.err }

// ctxBlockReader 阻塞 Read 直到 ctx 取消，用于确定性触发 client_gone。
type ctxBlockReader struct{ ctx context.Context }

func (r *ctxBlockReader) Read([]byte) (int, error) {
	<-r.ctx.Done()
	return 0, io.EOF
}

func setupOaiStreamHandlerTest(t *testing.T, body io.Reader) (*gin.Context, *httptest.ResponseRecorder, *relaycommon.RelayInfo, *http.Response) {
	t.Helper()

	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set(common.RequestIdKey, "oai-stream-test")

	info := &relaycommon.RelayInfo{
		ChannelMeta:        &relaycommon.ChannelMeta{UpstreamModelName: "gpt-test"},
		IsStream:           true,
		RelayFormat:        types.RelayFormatOpenAI,
		RelayMode:          relayconstant.RelayModeChatCompletions,
		ShouldIncludeUsage: true,
		DisablePing:        true,
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(body),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}

	return c, recorder, info, resp
}

// 零输出时上游中断（scanner_error）→ 可重试渠道错误，且不向客户端写任何字节。
func TestOaiStreamHandlerZeroOutputScannerErrorIsRetryable(t *testing.T) {
	c, recorder, info, resp := setupOaiStreamHandlerTest(t, &errReader{err: io.ErrUnexpectedEOF})

	usage, apiErr := OaiStreamHandler(c, info, resp)

	require.NotNil(t, apiErr, "zero-output scanner error must surface as a retryable API error")
	assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	assert.Nil(t, usage)
	assert.Empty(t, recorder.Body.String(), "no bytes may reach the client before a retry decision")
	assert.Equal(t, relaycommon.StreamEndReasonScannerErr, info.StreamStatus.EndReason)
	assert.Equal(t, relaycommon.StreamOutcomeFailed, info.StreamStatus.Outcome(info.ReceivedResponseCount))
}

// 已输出部分内容后上游中断 → finish_reason=length 终止块 + [DONE] 正常收尾，
// 不发送 error event（避免下游 SDK/编程工具中断），也不伪装成完整成功。
func TestOaiStreamHandlerPartialOutputTerminatesWithLength(t *testing.T) {
	chunk := `{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1710000000,"model":"gpt-test","choices":[{"index":0,"delta":{"content":"hi"},"finish_reason":null}]}`
	body := io.MultiReader(
		strings.NewReader("data: "+chunk+"\n"),
		&errReader{err: io.ErrUnexpectedEOF},
	)

	c, recorder, info, resp := setupOaiStreamHandlerTest(t, body)

	usage, apiErr := OaiStreamHandler(c, info, resp)

	require.Nil(t, apiErr, "partial output must not surface as a retryable error")
	require.NotNil(t, usage, "partial usage must still be returned for settlement")

	got := recorder.Body.String()
	assert.Contains(t, got, `"content":"hi"`, "already-received chunk should still be delivered")
	assert.Contains(t, got, `"finish_reason":"length"`, "interrupted stream must terminate with a non-stop finish reason")
	assert.Contains(t, got, `[DONE]`, "interrupted stream must still be terminated cleanly for SDKs")
	assert.NotContains(t, got, `"upstream_stream_error"`, "error event would abort the client task")
	assert.NotContains(t, got, `"finish_reason":"stop"`, "must not fake a complete success")
	assert.Equal(t, relaycommon.StreamEndReasonScannerErr, info.StreamStatus.EndReason)
	assert.Equal(t, relaycommon.StreamOutcomePartialFailure, info.StreamStatus.Outcome(info.ReceivedResponseCount))
}

// 客户端放弃（client_gone）→ 不重试、不输出、按已收内容结算。
func TestOaiStreamHandlerClientGoneReturnsUsageWithoutOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, recorder, info, resp := setupOaiStreamHandlerTest(t, &ctxBlockReader{ctx: ctx})
	c.Request = c.Request.WithContext(ctx)

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	usage, apiErr := OaiStreamHandler(c, info, resp)

	require.Nil(t, apiErr, "client cancellation is not an upstream fault")
	require.NotNil(t, usage)
	assert.Empty(t, recorder.Body.String(), "nothing should be written to a gone client")
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason)
	assert.Equal(t, relaycommon.StreamOutcomeCancelled, info.StreamStatus.Outcome(info.ReceivedResponseCount))
}
