package common

import (
	"errors"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamOutcomeAndFailureDomain(t *testing.T) {
	// scanner error with some data → partial_failure, upstream
	ss := &StreamStatus{EndReason: StreamEndReasonScannerErr, EndError: errors.New("scanner fail")}
	info := &RelayInfo{IsStream: true, ReceivedResponseCount: 5, StreamStatus: ss}
	assert.Equal(t, StreamOutcomePartialFailure, ss.Outcome(info.ReceivedResponseCount))
	assert.Equal(t, StreamFailureDomainUpstream, ss.FailureDomain())

	// scanner error with zero data → failed, upstream
	ss2 := &StreamStatus{EndReason: StreamEndReasonScannerErr, EndError: errors.New("scanner fail")}
	info2 := &RelayInfo{IsStream: true, ReceivedResponseCount: 0, StreamStatus: ss2}
	assert.Equal(t, StreamOutcomeFailed, ss2.Outcome(info2.ReceivedResponseCount))
	assert.Equal(t, StreamFailureDomainUpstream, ss2.FailureDomain())

	// client_gone → cancelled, downstream
	ss3 := &StreamStatus{EndReason: StreamEndReasonClientGone, EndError: errors.New("client gone")}
	info3 := &RelayInfo{IsStream: true, ReceivedResponseCount: 3, StreamStatus: ss3}
	assert.Equal(t, StreamOutcomeCancelled, ss3.Outcome(info3.ReceivedResponseCount))
	assert.Equal(t, StreamFailureDomainDownstream, ss3.FailureDomain())

	// timeout → failed, gateway
	ss4 := &StreamStatus{EndReason: StreamEndReasonTimeout, EndError: errors.New("timeout")}
	info4 := &RelayInfo{IsStream: true, ReceivedResponseCount: 0, StreamStatus: ss4}
	assert.Equal(t, StreamOutcomeFailed, ss4.Outcome(info4.ReceivedResponseCount))
	assert.Equal(t, StreamFailureDomainGateway, ss4.FailureDomain())

	// handler stop with downstream write error (e.g., broken pipe) → success with downstream domain
	ss5 := &StreamStatus{EndReason: StreamEndReasonHandlerStop, EndError: syscall.EPIPE}
	info5 := &RelayInfo{IsStream: true, ReceivedResponseCount: 2, StreamStatus: ss5}
	assert.Equal(t, StreamOutcomeSuccess, ss5.Outcome(info5.ReceivedResponseCount))
	assert.Equal(t, StreamFailureDomainDownstream, ss5.FailureDomain())

	// nil safety
	var nilSS *StreamStatus
	require.NotPanics(t, func() { nilSS.Outcome(0) })
	assert.Equal(t, StreamOutcomeSuccess, nilSS.Outcome(0))
	assert.Equal(t, StreamFailureDomainNone, nilSS.FailureDomain())
}
