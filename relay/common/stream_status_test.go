package common

import (
    "errors"
    "syscall"
    "testing"
)

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
