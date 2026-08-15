package common

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStreamStatus_SetEndReason_FirstWins(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	s.SetEndReason(StreamEndReasonDone, nil)
	s.SetEndReason(StreamEndReasonTimeout, nil)
	s.SetEndReason(StreamEndReasonClientGone, fmt.Errorf("context canceled"))

	assert.Equal(t, StreamEndReasonDone, s.EndReason)
	assert.Nil(t, s.EndError)
}

func TestStreamStatus_SetEndReason_WithError(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	expectedErr := fmt.Errorf("read: connection reset")
	s.SetEndReason(StreamEndReasonScannerErr, expectedErr)

	assert.Equal(t, StreamEndReasonScannerErr, s.EndReason)
	assert.Equal(t, expectedErr, s.EndError)
}

func TestStreamStatus_SetEndReason_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	s.SetEndReason(StreamEndReasonDone, nil)
}

func TestStreamStatus_SetEndReason_Concurrent(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	reasons := []StreamEndReason{
		StreamEndReasonDone,
		StreamEndReasonTimeout,
		StreamEndReasonClientGone,
		StreamEndReasonScannerErr,
		StreamEndReasonHandlerStop,
		StreamEndReasonEOF,
		StreamEndReasonPanic,
		StreamEndReasonPingFail,
	}

	var wg sync.WaitGroup
	for _, r := range reasons {
		wg.Add(1)
		go func(reason StreamEndReason) {
			defer wg.Done()
			s.SetEndReason(reason, nil)
		}(r)
	}
	wg.Wait()

	assert.NotEqual(t, StreamEndReasonNone, s.EndReason)
}

func TestStreamStatus_RecordError_Basic(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	s.RecordError("bad json")
	s.RecordError("another bad json")
	s.RecordError("client gone")

	assert.True(t, s.HasErrors())
	assert.Equal(t, 3, s.TotalErrorCount())
	assert.Len(t, s.Errors, 3)
}

func TestStreamStatus_RecordError_CapAtMax(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	for i := range 30 {
		s.RecordError(fmt.Sprintf("error_%d", i))
	}

	assert.Equal(t, maxStreamErrorEntries, len(s.Errors))
	assert.Equal(t, 30, s.TotalErrorCount())
}

func TestStreamStatus_RecordError_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	s.RecordError("should not panic")
}

func TestStreamStatus_RecordError_Concurrent(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s.RecordError(fmt.Sprintf("error_%d", idx))
		}(i)
	}
	wg.Wait()

	assert.Equal(t, 100, s.TotalErrorCount())
	assert.LessOrEqual(t, len(s.Errors), maxStreamErrorEntries)
}

func TestStreamStatus_HasErrors_Empty(t *testing.T) {
	t.Parallel()
	s := NewStreamStatus()
	assert.False(t, s.HasErrors())
	assert.Equal(t, 0, s.TotalErrorCount())
}

func TestStreamStatus_HasErrors_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	assert.False(t, s.HasErrors())
	assert.Equal(t, 0, s.TotalErrorCount())
}

func TestStreamStatus_IsNormalEnd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		reason StreamEndReason
		normal bool
	}{
		{StreamEndReasonDone, true},
		{StreamEndReasonEOF, true},
		{StreamEndReasonHandlerStop, true},
		{StreamEndReasonTimeout, false},
		{StreamEndReasonClientGone, false},
		{StreamEndReasonScannerErr, false},
		{StreamEndReasonPanic, false},
		{StreamEndReasonPingFail, false},
		{StreamEndReasonNone, false},
	}
	for _, tt := range tests {
		s := NewStreamStatus()
		s.SetEndReason(tt.reason, nil)
		assert.Equal(t, tt.normal, s.IsNormalEnd(), "reason=%s", tt.reason)
	}
}

func TestStreamOutcomeAndFailureDomain(t *testing.T) {
	// scanner error with some data → partial_failure, upstream
	ss := &StreamStatus{EndReason: StreamEndReasonScannerErr, EndError: errors.New("scanner fail")}
	info := &RelayInfo{IsStream: true, ReceivedResponseCount: 5, StreamStatus: ss}
	if got := ss.Outcome(info.ReceivedResponseCount); got != StreamOutcomePartialFailure {
		t.Fatalf("expected partial_failure, got %s", got)
	}
	if fd := ss.FailureDomain(); fd != StreamFailureDomainUpstream {
		t.Fatalf("expected upstream domain, got %s", fd)
	}

	// scanner error with zero data → failed, upstream
	ss2 := &StreamStatus{EndReason: StreamEndReasonScannerErr, EndError: errors.New("scanner fail")}
	info2 := &RelayInfo{IsStream: true, ReceivedResponseCount: 0, StreamStatus: ss2}
	if got := ss2.Outcome(info2.ReceivedResponseCount); got != StreamOutcomeFailed {
		t.Fatalf("expected failed, got %s", got)
	}
	if fd := ss2.FailureDomain(); fd != StreamFailureDomainUpstream {
		t.Fatalf("expected upstream domain, got %s", fd)
	}

	// client_gone → cancelled, downstream
	ss3 := &StreamStatus{EndReason: StreamEndReasonClientGone, EndError: errors.New("client gone")}
	info3 := &RelayInfo{IsStream: true, ReceivedResponseCount: 3, StreamStatus: ss3}
	if got := ss3.Outcome(info3.ReceivedResponseCount); got != StreamOutcomeCancelled {
		t.Fatalf("expected cancelled, got %s", got)
	}
	if fd := ss3.FailureDomain(); fd != StreamFailureDomainDownstream {
		t.Fatalf("expected downstream domain, got %s", fd)
	}

	// timeout → failed, gateway
	ss4 := &StreamStatus{EndReason: StreamEndReasonTimeout, EndError: errors.New("timeout")}
	info4 := &RelayInfo{IsStream: true, ReceivedResponseCount: 0, StreamStatus: ss4}
	if got := ss4.Outcome(info4.ReceivedResponseCount); got != StreamOutcomeFailed {
		t.Fatalf("expected failed on timeout, got %s", got)
	}
	if fd := ss4.FailureDomain(); fd != StreamFailureDomainGateway {
		t.Fatalf("expected gateway domain, got %s", fd)
	}

	// handler stop with downstream write error (e.g., broken pipe) → success with downstream domain
	ss5 := &StreamStatus{EndReason: StreamEndReasonHandlerStop, EndError: syscall.EPIPE}
	info5 := &RelayInfo{IsStream: true, ReceivedResponseCount: 2, StreamStatus: ss5}
	if got := ss5.Outcome(info5.ReceivedResponseCount); got != StreamOutcomeSuccess {
		t.Fatalf("expected success for handler stop without errors list, got %s", got)
	}
	if fd := ss5.FailureDomain(); fd != StreamFailureDomainDownstream {
		t.Fatalf("expected downstream domain for handler stop write error, got %s", fd)
	}
}

func TestStreamStatus_IsNormalEnd_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	assert.True(t, s.IsNormalEnd())
}

func TestStreamStatus_Summary(t *testing.T) {
	t.Parallel()

	s := NewStreamStatus()
	s.SetEndReason(StreamEndReasonDone, nil)
	summary := s.Summary()
	assert.Contains(t, summary, "reason=done")
	assert.NotContains(t, summary, "soft_errors")

	s2 := NewStreamStatus()
	s2.SetEndReason(StreamEndReasonTimeout, nil)
	s2.RecordError("bad json")
	s2.RecordError("write failed")
	summary2 := s2.Summary()
	assert.Contains(t, summary2, "reason=timeout")
	assert.Contains(t, summary2, "soft_errors=2")
}

func TestStreamStatus_Summary_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamStatus
	assert.Equal(t, "StreamStatus<nil>", s.Summary())
}
