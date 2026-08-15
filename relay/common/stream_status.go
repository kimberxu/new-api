package common

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"
)

type StreamEndReason string

const (
	StreamEndReasonNone        StreamEndReason = ""
	StreamEndReasonDone        StreamEndReason = "done"
	StreamEndReasonTimeout     StreamEndReason = "timeout"
	StreamEndReasonClientGone  StreamEndReason = "client_gone"
	StreamEndReasonScannerErr  StreamEndReason = "scanner_error"
	StreamEndReasonHandlerStop StreamEndReason = "handler_stop"
	StreamEndReasonEOF         StreamEndReason = "eof"
	StreamEndReasonPanic       StreamEndReason = "panic"
	StreamEndReasonPingFail    StreamEndReason = "ping_fail"
)

const maxStreamErrorEntries = 20

// StreamOutcome classifies what a stream termination means for the request.
// It is derived from the end reason plus how much was consumed, and feeds
// success metrics, debug-snapshot attach policy and downstream termination
// semantics (see relay/helper/stream_scanner.go and relay/channel/openai/relay-openai.go).
type StreamOutcome string

const (
	// StreamOutcomeSuccess: the stream completed normally (done/eof).
	StreamOutcomeSuccess StreamOutcome = "success"
	// StreamOutcomePartialFailure: upstream terminated abnormally after some
	// data had already been consumed; the response is incomplete.
	StreamOutcomePartialFailure StreamOutcome = "partial_failure"
	// StreamOutcomeFailed: upstream terminated abnormally before any usable
	// data was consumed.
	StreamOutcomeFailed StreamOutcome = "failed"
	// StreamOutcomeCancelled: the downstream client went away (client_gone /
	// ping_fail). Not an upstream fault.
	StreamOutcomeCancelled StreamOutcome = "cancelled"
)

// StreamFailureDomain attributes a non-successful termination to a party.
// It keeps channel health metrics free of downstream/gateway noise while
// still flagging upstream transport faults.
type StreamFailureDomain string

const (
	StreamFailureDomainNone       StreamFailureDomain = "none"
	StreamFailureDomainUpstream   StreamFailureDomain = "upstream"
	StreamFailureDomainDownstream StreamFailureDomain = "downstream"
	StreamFailureDomainGateway    StreamFailureDomain = "gateway"
	StreamFailureDomainProtocol   StreamFailureDomain = "protocol"
)

type StreamErrorEntry struct {
	Message   string
	Timestamp time.Time
}

type StreamStatus struct {
	EndReason StreamEndReason
	EndError  error
	endOnce   sync.Once

	mu         sync.Mutex
	Errors     []StreamErrorEntry
	ErrorCount int
}

func NewStreamStatus() *StreamStatus {
	return &StreamStatus{}
}

func (s *StreamStatus) SetEndReason(reason StreamEndReason, err error) {
	if s == nil {
		return
	}
	s.endOnce.Do(func() {
		s.EndReason = reason
		s.EndError = err
	})
}

func (s *StreamStatus) RecordError(msg string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ErrorCount++
	if len(s.Errors) < maxStreamErrorEntries {
		s.Errors = append(s.Errors, StreamErrorEntry{
			Message:   msg,
			Timestamp: time.Now(),
		})
	}
}

func (s *StreamStatus) HasErrors() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ErrorCount > 0
}

func (s *StreamStatus) TotalErrorCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ErrorCount
}

func (s *StreamStatus) IsNormalEnd() bool {
	if s == nil {
		return true
	}
	return s.EndReason == StreamEndReasonDone ||
		s.EndReason == StreamEndReasonEOF ||
		s.EndReason == StreamEndReasonHandlerStop
}

// isDownstreamWriteError reports whether the recorded end error indicates a
// failed write toward the downstream client (as opposed to an upstream or
// protocol fault).
func isDownstreamWriteError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, net.ErrClosed) || errors.Is(err, syscall.EPIPE)
}

// Outcome classifies the stream termination. receivedCount is the number of
// complete upstream data lines consumed (RelayInfo.ReceivedResponseCount); it
// distinguishes a mid-stream break (partial_failure) from a failure before
// any usable output (failed).
func (s *StreamStatus) Outcome(receivedCount int) StreamOutcome {
	if s == nil {
		return StreamOutcomeSuccess
	}
	switch s.EndReason {
	case StreamEndReasonDone, StreamEndReasonEOF:
		return StreamOutcomeSuccess
	case StreamEndReasonClientGone, StreamEndReasonPingFail:
		return StreamOutcomeCancelled
	case StreamEndReasonScannerErr, StreamEndReasonTimeout:
		if receivedCount > 0 {
			return StreamOutcomePartialFailure
		}
		return StreamOutcomeFailed
	case StreamEndReasonHandlerStop:
		if s.HasErrors() {
			return StreamOutcomeFailed
		}
		return StreamOutcomeSuccess
	case StreamEndReasonPanic:
		return StreamOutcomeFailed
	case StreamEndReasonNone:
		if s.HasErrors() {
			return StreamOutcomeFailed
		}
		return StreamOutcomeSuccess
	}
	return StreamOutcomeFailed
}

// FailureDomain attributes the termination to a party. Only meaningful when
// Outcome() != success.
func (s *StreamStatus) FailureDomain() StreamFailureDomain {
	if s == nil {
		return StreamFailureDomainNone
	}
	switch s.EndReason {
	case StreamEndReasonDone, StreamEndReasonEOF, StreamEndReasonNone:
		return StreamFailureDomainNone
	case StreamEndReasonScannerErr:
		return StreamFailureDomainUpstream
	case StreamEndReasonTimeout:
		return StreamFailureDomainGateway
	case StreamEndReasonClientGone, StreamEndReasonPingFail:
		return StreamFailureDomainDownstream
	case StreamEndReasonHandlerStop:
		if isDownstreamWriteError(s.EndError) {
			return StreamFailureDomainDownstream
		}
		return StreamFailureDomainProtocol
	case StreamEndReasonPanic:
		return StreamFailureDomainGateway
	}
	return StreamFailureDomainGateway
}

func (s *StreamStatus) Summary() string {
	if s == nil {
		return "StreamStatus<nil>"
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "reason=%s", s.EndReason)
	if s.EndError != nil {
		fmt.Fprintf(b, " end_error=%q", s.EndError.Error())
	}
	s.mu.Lock()
	if s.ErrorCount > 0 {
		fmt.Fprintf(b, " soft_errors=%d", s.ErrorCount)
	}
	s.mu.Unlock()
	return b.String()
}
