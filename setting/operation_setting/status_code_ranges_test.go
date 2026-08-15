package operation_setting

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestParseHTTPStatusCodeRanges_CommaSeparated(t *testing.T) {
	ranges, err := ParseHTTPStatusCodeRanges("401,403,500-599")
	require.NoError(t, err)
	require.Equal(t, []StatusCodeRange{
		{Start: 401, End: 401},
		{Start: 403, End: 403},
		{Start: 500, End: 599},
	}, ranges)
}

func TestParseHTTPStatusCodeRanges_MergeAndNormalize(t *testing.T) {
	ranges, err := ParseHTTPStatusCodeRanges("500-505,504,401,403,402")
	require.NoError(t, err)
	require.Equal(t, []StatusCodeRange{
		{Start: 401, End: 403},
		{Start: 500, End: 505},
	}, ranges)
}

func TestParseHTTPStatusCodeRanges_Invalid(t *testing.T) {
	_, err := ParseHTTPStatusCodeRanges("99,600,foo,500-400,500-")
	require.Error(t, err)
}

func TestParseHTTPStatusCodeRanges_NoComma_IsInvalid(t *testing.T) {
	_, err := ParseHTTPStatusCodeRanges("401 403")
	require.Error(t, err)
}

func TestShouldDisableByStatusCode(t *testing.T) {
	origRanges := AutomaticDisableStatusCodeRanges
	origFlag := common.AutomaticRetryTimeoutEnabled
	t.Cleanup(func() {
		AutomaticDisableStatusCodeRanges = origRanges
		common.AutomaticRetryTimeoutEnabled = origFlag
	})

	AutomaticDisableStatusCodeRanges = []StatusCodeRange{
		{Start: 401, End: 403},
		{Start: 500, End: 503},
		{Start: 505, End: 523},
		{Start: 525, End: 599},
	}

	common.AutomaticRetryTimeoutEnabled = false
	require.True(t, ShouldDisableByStatusCode(401))
	require.True(t, ShouldDisableByStatusCode(403))
	require.False(t, ShouldDisableByStatusCode(404))
	require.True(t, ShouldDisableByStatusCode(500))
	require.True(t, ShouldDisableByStatusCode(504))
	require.True(t, ShouldDisableByStatusCode(524))
	require.False(t, ShouldDisableByStatusCode(200))

	common.AutomaticRetryTimeoutEnabled = true
	require.True(t, ShouldDisableByStatusCode(504))
	require.True(t, ShouldDisableByStatusCode(524))
}

func TestShouldRetryByStatusCode(t *testing.T) {
	orig := AutomaticRetryStatusCodeRanges
	t.Cleanup(func() { AutomaticRetryStatusCodeRanges = orig })

	AutomaticRetryStatusCodeRanges = []StatusCodeRange{
		{Start: 429, End: 429},
		{Start: 500, End: 599},
	}

	require.True(t, ShouldRetryByStatusCode(429))
	require.True(t, ShouldRetryByStatusCode(500))
	require.False(t, ShouldRetryByStatusCode(504))
	require.False(t, ShouldRetryByStatusCode(524))
	require.False(t, ShouldRetryByStatusCode(400))
	require.False(t, ShouldRetryByStatusCode(200))
}

func TestShouldRetryByStatusCode_DefaultMatchesLegacyBehavior(t *testing.T) {
	require.False(t, ShouldRetryByStatusCode(200))
	require.False(t, ShouldRetryByStatusCode(400))
	require.True(t, ShouldRetryByStatusCode(401))
	require.False(t, ShouldRetryByStatusCode(408))
	require.True(t, ShouldRetryByStatusCode(429))
	require.True(t, ShouldRetryByStatusCode(500))
	require.False(t, ShouldRetryByStatusCode(504))
	require.False(t, ShouldRetryByStatusCode(524))
	require.True(t, ShouldRetryByStatusCode(599))
}

func TestIsAlwaysSkipRetryStatusCode(t *testing.T) {
	require.True(t, IsAlwaysSkipRetryStatusCode(504))
	require.True(t, IsAlwaysSkipRetryStatusCode(524))
	require.False(t, IsAlwaysSkipRetryStatusCode(500))
}

func TestShouldRetryByStatusCode_TimeoutRetryEnabled(t *testing.T) {
	// 开关关闭时（默认），504/524 始终不重试，即使范围配置包含它们。
	origRanges := AutomaticRetryStatusCodeRanges
	origFlag := common.AutomaticRetryTimeoutEnabled
	t.Cleanup(func() {
		AutomaticRetryStatusCodeRanges = origRanges
		common.AutomaticRetryTimeoutEnabled = origFlag
	})

	AutomaticRetryStatusCodeRanges = []StatusCodeRange{
		{Start: 500, End: 599},
	}

	common.AutomaticRetryTimeoutEnabled = false
	require.False(t, ShouldRetryByStatusCode(504))
	require.False(t, ShouldRetryByStatusCode(524))

	// 开关开启后，504/524 放行到用户配置的重试范围。
	common.AutomaticRetryTimeoutEnabled = true
	require.True(t, ShouldRetryByStatusCode(504))
	require.True(t, ShouldRetryByStatusCode(524))

	// 开关开启但仍不在重试范围内的 5xx 不重试。
	common.AutomaticRetryTimeoutEnabled = true
	require.False(t, ShouldRetryByStatusCode(599+1))

	// 开关开启不影响始终跳过的非超时语义（400 仍不重试）。
	require.False(t, ShouldRetryByStatusCode(400))
}
