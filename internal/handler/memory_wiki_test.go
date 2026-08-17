package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/justaboyhai-wq/fmind/internal/application/service/memorywiki"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type memoryWikiReviewServiceStub struct {
	decision string
	tenantID uint64
	id       string
	reviewer string
	comment  string
	err      error
}

func (s *memoryWikiReviewServiceStub) List(context.Context, uint64, string) ([]*types.MemoryWikiPublication, error) {
	return []*types.MemoryWikiPublication{}, s.err
}
func (s *memoryWikiReviewServiceStub) GetReview(context.Context, uint64, string) (*interfaces.ExternalMemoryProjection, error) {
	return &interfaces.ExternalMemoryProjection{}, s.err
}
func (s *memoryWikiReviewServiceStub) record(decision string, tenantID uint64, id, reviewer, comment string) (*types.MemoryReviewTask, error) {
	s.decision, s.tenantID, s.id, s.reviewer, s.comment = decision, tenantID, id, reviewer, comment
	return &types.MemoryReviewTask{ID: "review-task-1", Status: decision}, s.err
}
func (s *memoryWikiReviewServiceStub) ApprovePublication(_ context.Context, tenantID uint64, id, reviewer, comment string) (*types.MemoryReviewTask, error) {
	return s.record("approved", tenantID, id, reviewer, comment)
}
func (s *memoryWikiReviewServiceStub) RejectPublication(_ context.Context, tenantID uint64, id, reviewer, comment string) (*types.MemoryReviewTask, error) {
	return s.record("rejected", tenantID, id, reviewer, comment)
}
func (s *memoryWikiReviewServiceStub) RequestPublicationChanges(_ context.Context, tenantID uint64, id, reviewer, comment string) (*types.MemoryReviewTask, error) {
	return s.record("changes_requested", tenantID, id, reviewer, comment)
}
func (s *memoryWikiReviewServiceStub) PublishApproved(context.Context, uint64, string, string) (*types.WikiPage, error) {
	return &types.WikiPage{ID: "page-1"}, s.err
}

func TestMemoryWikiHandlerUsesAuthenticatedReviewerIdentity(t *testing.T) {
	stub := &memoryWikiReviewServiceStub{}
	h := newMemoryWikiHandler(stub)
	w := invokeMemoryWikiHandler(t, http.MethodPost, "/external-memory/l3/reviews/pub-1/approve", `{"comment":" verified "}`, h.Approve)
	if w.Code != http.StatusOK || stub.decision != "approved" || stub.tenantID != 7 || stub.id != "pub-1" || stub.reviewer != "reviewer-1" || stub.comment != "verified" {
		t.Fatalf("status=%d body=%s stub=%#v", w.Code, w.Body.String(), stub)
	}
}

func TestMemoryWikiHandlerRequiresCommentForChangesRequested(t *testing.T) {
	stub := &memoryWikiReviewServiceStub{}
	h := newMemoryWikiHandler(stub)
	w := invokeMemoryWikiHandler(t, http.MethodPost, "/external-memory/l3/reviews/pub-1/request-changes", `{}`, h.RequestChanges)
	if w.Code != http.StatusBadRequest || stub.decision != "" {
		t.Fatalf("status=%d body=%s stub=%#v", w.Code, w.Body.String(), stub)
	}
}

func TestMemoryWikiHandlerRedactsRepositoryErrors(t *testing.T) {
	stub := &memoryWikiReviewServiceStub{err: errors.New("postgres password=secret-host")}
	h := newMemoryWikiHandler(stub)
	w := invokeMemoryWikiHandler(t, http.MethodGet, "/external-memory/l3/reviews", "", h.ListReviews)
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "postgres") || strings.Contains(w.Body.String(), "secret-host") {
		t.Fatalf("status=%d unsafe body=%s", w.Code, w.Body.String())
	}
	stub.err = memorywiki.ErrMemoryWikiReviewerRequired
	w = invokeMemoryWikiHandler(t, http.MethodGet, "/external-memory/l3/reviews", "", h.ListReviews)
	if w.Code != http.StatusForbidden {
		t.Fatalf("permission status=%d body=%s", w.Code, w.Body.String())
	}
}

func invokeMemoryWikiHandler(t *testing.T, method, path, body string, target gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		ctx := context.WithValue(c.Request.Context(), types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.UserIDContextKey, "reviewer-1")
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleAdmin)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	route := strings.Replace(path, "pub-1", ":id", 1)
	r.Handle(method, route, target)
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
