package handler

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/application/repository"
	"github.com/justaboyhai-wq/fmind/internal/application/service/memorywiki"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

const (
	memoryEventServiceHeader    = "X-FMind-Service-ID"
	memoryEventTimestampHeader  = "X-FMind-Event-Timestamp"
	memoryEventSignatureHeader  = "X-FMind-Event-Signature"
	memoryEventSignaturePrefix  = "v1="
	memoryEventSignatureDomain  = "fmind-memory-event/v1"
	memoryEventMaxClockSkew     = 5 * time.Minute
	maxMemoryEventRequestBytes  = 2 << 20
	minimumMemoryEventSecretLen = 32
)

var memoryEventServiceIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

type trustedL3EventReceiver interface {
	ReceiveTrustedL3Event(context.Context, types.TrustedL3Event) (*interfaces.ExternalMemoryProjection, bool, error)
}

type eventBindingReader interface {
	GetAgentBinding(context.Context, uint64, string) (*types.AgentBinding, error)
}

type signedTrustedL3Event struct {
	types.TrustedL3Event
	BindingPolicyVersion uint64 `json:"binding_policy_version"`
}

type InternalMemoryEventHandler struct {
	receiver  trustedL3EventReceiver
	bindings  eventBindingReader
	secret    []byte
	serviceID string
	enabled   bool
	now       func() time.Time
}

// NewInternalMemoryEventHandler keeps standalone FMind deployments working,
// but requires a strong independent service credential as soon as MemoryCore
// integration is configured.
func NewInternalMemoryEventHandler(receiver *memorywiki.Service, bindings interfaces.AgentBindingRepository) (*InternalMemoryEventHandler, error) {
	secret := strings.TrimSpace(os.Getenv("FMIND_MEMORY_EVENT_SECRET"))
	serviceID := strings.TrimSpace(os.Getenv("FMIND_MEMORY_EVENT_SERVICE_ID"))
	if serviceID == "" {
		serviceID = "memory-core"
	}
	integrationConfigured := strings.TrimSpace(os.Getenv("FMIND_MEMORY_CORE_URL")) != "" || secret != "" || environmentEnabled(os.Getenv("FMIND_MEMORY_EVENTS_ENABLED"))
	if !integrationConfigured {
		return &InternalMemoryEventHandler{receiver: receiver, bindings: bindings, serviceID: serviceID, now: time.Now}, nil
	}
	if len(secret) < minimumMemoryEventSecretLen || secret != os.Getenv("FMIND_MEMORY_EVENT_SECRET") {
		return nil, errors.New("FMIND_MEMORY_EVENT_SECRET must be at least 32 bytes without surrounding whitespace")
	}
	if !memoryEventServiceIDPattern.MatchString(serviceID) {
		return nil, errors.New("FMIND_MEMORY_EVENT_SERVICE_ID is invalid")
	}
	return newInternalMemoryEventHandler(receiver, bindings, []byte(secret), serviceID), nil
}

func newInternalMemoryEventHandler(receiver trustedL3EventReceiver, bindings eventBindingReader, secret []byte, serviceID string) *InternalMemoryEventHandler {
	return &InternalMemoryEventHandler{
		receiver: receiver, bindings: bindings, secret: append([]byte(nil), secret...),
		serviceID: serviceID, enabled: true, now: time.Now,
	}
}

func environmentEnabled(value string) bool {
	enabled, err := strconv.ParseBool(strings.TrimSpace(value))
	return err == nil && enabled
}

func (h *InternalMemoryEventHandler) Receive(c *gin.Context) {
	if h == nil || !h.enabled || h.receiver == nil || h.bindings == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory event intake is not configured"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMemoryEventRequestBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "memory event is too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memory event"})
		return
	}
	if !h.verifyRequest(c.Request, body) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid memory event credentials"})
		return
	}
	var envelope signedTrustedL3Event
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&envelope); err != nil || !jsonAtEOF(decoder) || envelope.BindingPolicyVersion == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid memory event"})
		return
	}
	if err := memorywiki.ValidateTrustedL3Event(envelope.TrustedL3Event); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid memory event"})
		return
	}
	authorized, authorizationErr := h.authorizeBinding(c.Request.Context(), envelope)
	if authorizationErr != nil {
		// A transient control-plane read failure must remain retryable by the
		// durable MemoryCore outbox. Treat only authoritative scope/policy
		// denials as a permanent 403.
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory event authorization is temporarily unavailable"})
		return
	}
	if !authorized {
		c.JSON(http.StatusForbidden, gin.H{"error": "memory event binding scope denied"})
		return
	}
	_, duplicate, err := h.receiver.ReceiveTrustedL3Event(c.Request.Context(), envelope.TrustedL3Event)
	if err != nil {
		if errors.Is(err, repository.ErrExternalMemoryEventConflict) || errors.Is(err, repository.ErrExternalMemoryStateConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "memory event conflicts with durable state"})
			return
		}
		// The body and repository error are deliberately omitted. A 503 tells
		// the durable outbox worker to retry without leaking internal state.
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory event could not be persisted"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"accepted": true, "duplicate": duplicate, "event_id": envelope.EventID})
}

func jsonAtEOF(decoder *json.Decoder) bool {
	var trailing any
	return errors.Is(decoder.Decode(&trailing), io.EOF)
}

func (h *InternalMemoryEventHandler) verifyRequest(request *http.Request, body []byte) bool {
	if request == nil || request.Method != http.MethodPost || request.URL.Path != "/internal/v1/memory/events" {
		return false
	}
	serviceID := strings.TrimSpace(request.Header.Get(memoryEventServiceHeader))
	rawTimestamp := strings.TrimSpace(request.Header.Get(memoryEventTimestampHeader))
	signature := strings.TrimSpace(request.Header.Get(memoryEventSignatureHeader))
	if serviceID != h.serviceID || rawTimestamp == "" || signature == "" {
		return false
	}
	timestamp, err := strconv.ParseInt(rawTimestamp, 10, 64)
	if err != nil || strconv.FormatInt(timestamp, 10) != rawTimestamp {
		return false
	}
	delta := h.now().UTC().Sub(time.Unix(timestamp, 0).UTC())
	if delta < -memoryEventMaxClockSkew || delta > memoryEventMaxClockSkew {
		return false
	}
	expected := ComputeMemoryEventSignature(h.secret, serviceID, rawTimestamp, body)
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (h *InternalMemoryEventHandler) authorizeBinding(ctx context.Context, envelope signedTrustedL3Event) (bool, error) {
	event := envelope.TrustedL3Event
	binding, err := h.bindings.GetAgentBinding(ctx, event.TenantID, event.BindingID)
	if err != nil {
		if errors.Is(err, repository.ErrAgentBindingNotFound) {
			return false, nil
		}
		return false, err
	}
	if binding == nil || binding.DeletedAt.Valid {
		return false, nil
	}
	scopeMatches := binding.TenantID == event.TenantID && binding.ID == event.BindingID &&
		binding.DepartmentID == event.DepartmentID && binding.WorkspaceID == event.WorkspaceID && binding.ProjectID == event.ProjectID &&
		binding.TeamID == event.TeamID && binding.UserID == event.UserID && binding.AgentID == event.AgentID && binding.TaskID == event.TaskID
	if !scopeMatches {
		return false, nil
	}
	// Revocation is a cleanup event, not a new publication authority. It must
	// remain deliverable after the binding is revoked, expires, or has L3
	// publication disabled; otherwise already-published Wiki content can never
	// be archived. The service HMAC and immutable binding scope remain required,
	// and a claimed policy version may not be from the future.
	if event.EventType == types.MemoryL3EventRevoked {
		return envelope.BindingPolicyVersion <= binding.PolicyVersion, nil
	}
	if binding.Status != types.AgentBindingStatusActive || (binding.ExpiresAt != nil && !binding.ExpiresAt.After(h.now().UTC())) {
		return false, nil
	}
	if binding.PolicyVersion != envelope.BindingPolicyVersion || !binding.L3WikiEnabled || !binding.L3ReviewRequired ||
		!containsAnyScope(binding.CapabilityScopes, "memory.publish", "memory.l3.publish") || !containsScope(binding.AssetScopes, "team:"+binding.TeamID) {
		return false, nil
	}
	return true, nil
}

func containsAnyScope(scopes types.StringArray, targets ...string) bool {
	for _, target := range targets {
		if containsScope(scopes, target) {
			return true
		}
	}
	return false
}

func containsScope(scopes types.StringArray, target string) bool {
	for _, scope := range scopes {
		if scope == target {
			return true
		}
	}
	return false
}

// ComputeMemoryEventSignature is the frozen cross-language HMAC contract used
// by MemoryCore's durable outbox worker and FMind's intake.
func ComputeMemoryEventSignature(secret []byte, serviceID, timestamp string, body []byte) string {
	bodyDigest := sha256.Sum256(body)
	canonical := fmt.Sprintf("%s\n%s\n%s\n%s", memoryEventSignatureDomain, serviceID, timestamp, hex.EncodeToString(bodyDigest[:]))
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	return memoryEventSignaturePrefix + hex.EncodeToString(mac.Sum(nil))
}
