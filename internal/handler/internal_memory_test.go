package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/application/repository"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type trustedL3ReceiverStub struct {
	event  types.TrustedL3Event
	called int
	dup    bool
	err    error
}

func (s *trustedL3ReceiverStub) ReceiveTrustedL3Event(_ context.Context, event types.TrustedL3Event) (*interfaces.ExternalMemoryProjection, bool, error) {
	s.called++
	s.event = event
	return &interfaces.ExternalMemoryProjection{}, s.dup, s.err
}

type eventBindingReaderStub struct {
	binding *types.AgentBinding
	err     error
}

func (s *eventBindingReaderStub) GetAgentBinding(context.Context, uint64, string) (*types.AgentBinding, error) {
	return s.binding, s.err
}

func validInternalMemoryEvent() signedTrustedL3Event {
	content := "# Runbook\n\nThe validated escalation window is 15 minutes."
	sum := sha256.Sum256([]byte(content))
	ref := types.EvidenceReference{Type: "memory_l1", ID: "l1-1", Locator: "claim-1", Checksum: "sha256:" + bytesToHex(sum[:])}
	return signedTrustedL3Event{
		TrustedL3Event: types.TrustedL3Event{
			EventID: "evt-1", EventType: types.MemoryL3EventMatured, SchemaVersion: "1.0", OccurredAt: time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC),
			TenantID: 7, DepartmentID: "dept-1", WorkspaceID: "workspace-1", ProjectID: "project-1", TeamID: "team-1",
			BindingID: "binding-1", UserID: "user-1", AgentID: "agent-1", TaskID: "task-1",
			MemoryID: "memory-1", MemoryVersion: 3, MemoryLevel: "L3", Maturity: "matured",
			Title: "Runbook", Summary: "Validated process", ContentMarkdown: content, Confidence: 0.93, Sensitivity: "internal",
			EvidenceRefs: types.EvidenceReferences{ref}, Claims: types.ClaimEvidenceSet{{ClaimID: "claim-1", Text: "The validated escalation window is 15 minutes.", Factual: true, Evidence: types.EvidenceReferences{ref}}},
			ContentChecksum: "sha256:" + bytesToHex(sum[:]),
		},
		BindingPolicyVersion: 4,
	}
}

func validEventBinding() *types.AgentBinding {
	return &types.AgentBinding{
		ID: "binding-1", TenantID: 7, DepartmentID: "dept-1", WorkspaceID: "workspace-1", ProjectID: "project-1", TeamID: "team-1",
		UserID: "user-1", AgentID: "agent-1", TaskID: "task-1", Status: types.AgentBindingStatusActive,
		L3WikiEnabled: true, L3ReviewRequired: true, PolicyVersion: 4,
		CapabilityScopes: types.StringArray{"memory.l3.publish"}, AssetScopes: types.StringArray{"tenant:7", "team:team-1"},
	}
}

func TestInternalMemoryEventAcceptsSignedCurrentBindingScope(t *testing.T) {
	receiver := &trustedL3ReceiverStub{}
	h := newInternalMemoryEventHandler(receiver, &eventBindingReaderStub{binding: validEventBinding()}, []byte("0123456789abcdef0123456789abcdef"), "memory-core")
	h.now = func() time.Time { return time.Unix(1_776_000_000, 0).UTC() }
	w := invokeSignedMemoryEvent(t, h, validInternalMemoryEvent(), h.now(), true)
	if w.Code != http.StatusAccepted || receiver.called != 1 || receiver.event.BindingID != "binding-1" {
		t.Fatalf("status=%d body=%s called=%d event=%#v", w.Code, w.Body.String(), receiver.called, receiver.event)
	}
}

func TestInternalMemoryEventAcceptsSignedRevocation(t *testing.T) {
	receiver := &trustedL3ReceiverStub{}
	h := newInternalMemoryEventHandler(receiver, &eventBindingReaderStub{binding: validEventBinding()}, []byte("0123456789abcdef0123456789abcdef"), "memory-core")
	h.now = func() time.Time { return time.Unix(1_776_000_000, 0).UTC() }
	event := validInternalMemoryEvent()
	event.EventID = "evt-revoked"
	event.EventType = types.MemoryL3EventRevoked
	event.Maturity = "revoked"
	w := invokeSignedMemoryEvent(t, h, event, h.now(), true)
	if w.Code != http.StatusAccepted || receiver.called != 1 || receiver.event.EventType != types.MemoryL3EventRevoked {
		t.Fatalf("status=%d body=%s called=%d event=%#v", w.Code, w.Body.String(), receiver.called, receiver.event)
	}
}

func TestInternalMemoryRevocationRemainsAuthorizedAfterBindingShutdown(t *testing.T) {
	now := time.Unix(1_776_000_000, 0).UTC()
	binding := validEventBinding()
	binding.Status = types.AgentBindingStatusRevoked
	binding.ExpiresAt = ptrTime(now.Add(-time.Hour))
	binding.L3WikiEnabled = false
	binding.L3ReviewRequired = false
	binding.CapabilityScopes = nil
	binding.AssetScopes = nil
	binding.PolicyVersion = 5
	receiver := &trustedL3ReceiverStub{}
	h := newInternalMemoryEventHandler(receiver, &eventBindingReaderStub{binding: binding}, []byte("0123456789abcdef0123456789abcdef"), "memory-core")
	h.now = func() time.Time { return now }
	event := validInternalMemoryEvent()
	event.EventID = "evt-revoked-after-shutdown"
	event.EventType = types.MemoryL3EventRevoked
	event.Maturity = "revoked"
	w := invokeSignedMemoryEvent(t, h, event, now, true)
	if w.Code != http.StatusAccepted || receiver.called != 1 {
		t.Fatalf("status=%d body=%s called=%d", w.Code, w.Body.String(), receiver.called)
	}
	event.BindingPolicyVersion = 6
	w = invokeSignedMemoryEvent(t, h, event, now, true)
	if w.Code != http.StatusForbidden {
		t.Fatalf("future policy revocation status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestInternalMemoryEventReturnsConflictForPermanentIdempotencyMismatch(t *testing.T) {
	receiver := &trustedL3ReceiverStub{err: repository.ErrExternalMemoryEventConflict}
	h := newInternalMemoryEventHandler(receiver, &eventBindingReaderStub{binding: validEventBinding()}, []byte("0123456789abcdef0123456789abcdef"), "memory-core")
	h.now = func() time.Time { return time.Unix(1_776_000_000, 0).UTC() }
	w := invokeSignedMemoryEvent(t, h, validInternalMemoryEvent(), h.now(), true)
	if w.Code != http.StatusConflict || receiver.called != 1 {
		t.Fatalf("status=%d body=%s called=%d", w.Code, w.Body.String(), receiver.called)
	}
}

func TestInternalMemoryEventReturnsRetryableForConcurrentLifecycleMutation(t *testing.T) {
	receiver := &trustedL3ReceiverStub{err: repository.ErrExternalMemoryConcurrentMutation}
	h := newInternalMemoryEventHandler(receiver, &eventBindingReaderStub{binding: validEventBinding()}, []byte("0123456789abcdef0123456789abcdef"), "memory-core")
	h.now = func() time.Time { return time.Unix(1_776_000_000, 0).UTC() }
	w := invokeSignedMemoryEvent(t, h, validInternalMemoryEvent(), h.now(), true)
	if w.Code != http.StatusServiceUnavailable || receiver.called != 1 {
		t.Fatalf("status=%d body=%s called=%d", w.Code, w.Body.String(), receiver.called)
	}
}

func TestInternalMemoryEventReturnsRetryableWhenBindingAuthorityIsUnavailable(t *testing.T) {
	receiver := &trustedL3ReceiverStub{}
	h := newInternalMemoryEventHandler(receiver, &eventBindingReaderStub{err: errors.New("binding database unavailable")}, []byte("0123456789abcdef0123456789abcdef"), "memory-core")
	h.now = func() time.Time { return time.Unix(1_776_000_000, 0).UTC() }
	w := invokeSignedMemoryEvent(t, h, validInternalMemoryEvent(), h.now(), true)
	if w.Code != http.StatusServiceUnavailable || receiver.called != 0 {
		t.Fatalf("status=%d body=%s called=%d", w.Code, w.Body.String(), receiver.called)
	}
}

func TestInternalMemoryEventReturnsForbiddenWhenBindingDoesNotExist(t *testing.T) {
	receiver := &trustedL3ReceiverStub{}
	h := newInternalMemoryEventHandler(receiver, &eventBindingReaderStub{err: repository.ErrAgentBindingNotFound}, []byte("0123456789abcdef0123456789abcdef"), "memory-core")
	h.now = func() time.Time { return time.Unix(1_776_000_000, 0).UTC() }
	w := invokeSignedMemoryEvent(t, h, validInternalMemoryEvent(), h.now(), true)
	if w.Code != http.StatusForbidden || receiver.called != 0 {
		t.Fatalf("status=%d body=%s called=%d", w.Code, w.Body.String(), receiver.called)
	}
}

func TestInternalMemoryEventRejectsMissingStaleOrInvalidSignature(t *testing.T) {
	now := time.Unix(1_776_000_000, 0).UTC()
	for _, tc := range []struct {
		name      string
		timestamp time.Time
		sign      bool
		mutate    func(*http.Request)
	}{
		{name: "missing", timestamp: now},
		{name: "stale", timestamp: now.Add(-10 * time.Minute), sign: true},
		{name: "wrong service", timestamp: now, sign: true, mutate: func(r *http.Request) { r.Header.Set(memoryEventServiceHeader, "attacker") }},
		{name: "bad signature", timestamp: now, sign: true, mutate: func(r *http.Request) { r.Header.Set(memoryEventSignatureHeader, "v1=00") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			receiver := &trustedL3ReceiverStub{}
			h := newInternalMemoryEventHandler(receiver, &eventBindingReaderStub{binding: validEventBinding()}, []byte("0123456789abcdef0123456789abcdef"), "memory-core")
			h.now = func() time.Time { return now }
			w := invokeSignedMemoryEventWithMutation(t, h, validInternalMemoryEvent(), tc.timestamp, tc.sign, tc.mutate)
			if w.Code != http.StatusUnauthorized || receiver.called != 0 {
				t.Fatalf("status=%d body=%s called=%d", w.Code, w.Body.String(), receiver.called)
			}
		})
	}
}

func TestInternalMemoryEventRejectsBindingScopeAndPolicyMismatch(t *testing.T) {
	now := time.Unix(1_776_000_000, 0).UTC()
	for _, mutate := range []func(*signedTrustedL3Event){
		func(v *signedTrustedL3Event) { v.TeamID = "team-2" },
		func(v *signedTrustedL3Event) { v.UserID = "user-2" },
		func(v *signedTrustedL3Event) { v.BindingPolicyVersion = 3 },
	} {
		receiver := &trustedL3ReceiverStub{}
		h := newInternalMemoryEventHandler(receiver, &eventBindingReaderStub{binding: validEventBinding()}, []byte("0123456789abcdef0123456789abcdef"), "memory-core")
		h.now = func() time.Time { return now }
		event := validInternalMemoryEvent()
		mutate(&event)
		w := invokeSignedMemoryEvent(t, h, event, now, true)
		if w.Code != http.StatusForbidden || receiver.called != 0 {
			t.Fatalf("status=%d body=%s called=%d", w.Code, w.Body.String(), receiver.called)
		}
	}
}

func TestMemoryEventSignatureFixture(t *testing.T) {
	got := ComputeMemoryEventSignature([]byte("0123456789abcdef0123456789abcdef"), "memory-core", "1776000000", []byte(`{"event_id":"evt-1"}`))
	const want = "v1=d441276ad67df93530b1562378994741b66dd3ac033e8404d889130e833346ee"
	if got != want {
		t.Fatalf("signature=%s want=%s", got, want)
	}
}

func TestNewInternalMemoryEventHandlerConfigurationIsFailClosed(t *testing.T) {
	t.Setenv("FMIND_MEMORY_CORE_URL", "")
	t.Setenv("FMIND_MEMORY_EVENTS_ENABLED", "")
	t.Setenv("FMIND_MEMORY_EVENT_SECRET", "")
	disabled, err := NewInternalMemoryEventHandler(nil, nil)
	if err != nil || disabled == nil || disabled.enabled {
		t.Fatalf("disabled handler=%#v err=%v", disabled, err)
	}

	t.Setenv("FMIND_MEMORY_CORE_URL", "http://memory-core:3000")
	if _, err := NewInternalMemoryEventHandler(nil, nil); err == nil {
		t.Fatal("configured MemoryCore without event secret must fail startup")
	}
	t.Setenv("FMIND_MEMORY_EVENT_SECRET", " 0123456789abcdef0123456789abcdef ")
	if _, err := NewInternalMemoryEventHandler(nil, nil); err == nil {
		t.Fatal("event secret with surrounding whitespace must be rejected")
	}
}

func invokeSignedMemoryEvent(t *testing.T, h *InternalMemoryEventHandler, event signedTrustedL3Event, timestamp time.Time, sign bool) *httptest.ResponseRecorder {
	return invokeSignedMemoryEventWithMutation(t, h, event, timestamp, sign, nil)
}

func invokeSignedMemoryEventWithMutation(t *testing.T, h *InternalMemoryEventHandler, event signedTrustedL3Event, timestamp time.Time, sign bool, mutate func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/memory/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(memoryEventServiceHeader, "memory-core")
	ts := strconv.FormatInt(timestamp.Unix(), 10)
	req.Header.Set(memoryEventTimestampHeader, ts)
	if sign {
		req.Header.Set(memoryEventSignatureHeader, ComputeMemoryEventSignature(h.secret, "memory-core", ts, body))
	}
	if mutate != nil {
		mutate(req)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/internal/v1/memory/events", h.Receive)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func bytesToHex(value []byte) string {
	const alphabet = "0123456789abcdef"
	result := make([]byte, len(value)*2)
	for i, b := range value {
		result[i*2] = alphabet[b>>4]
		result[i*2+1] = alphabet[b&0xf]
	}
	return string(result)
}

func ptrTime(value time.Time) *time.Time { return &value }
