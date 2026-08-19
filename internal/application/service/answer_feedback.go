package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"gorm.io/gorm"
)

var (
	ErrFeedbackInvalid   = errors.New("invalid answer feedback")
	ErrFeedbackNotFound  = errors.New("answer feedback not found")
	ErrFeedbackForbidden = errors.New("answer feedback access denied")
)

var feedbackCategories = map[string]bool{"wrong_fact": true, "outdated": true, "citation_mismatch": true, "incomplete": true, "misunderstood": true, "unsafe": true, "other": true}
var feedbackStatuses = map[string]bool{types.FeedbackStatusPending: true, types.FeedbackStatusReviewing: true, types.FeedbackStatusNeedsInfo: true, types.FeedbackStatusFixing: true, types.FeedbackStatusResolved: true, types.FeedbackStatusDismissed: true}

type answerFeedbackService struct {
	repo     interfaces.AnswerFeedbackRepository
	messages interfaces.MessageRepository
}

func NewAnswerFeedbackService(repo interfaces.AnswerFeedbackRepository, messages interfaces.MessageRepository) interfaces.AnswerFeedbackService {
	return &answerFeedbackService{repo: repo, messages: messages}
}

func (s *answerFeedbackService) Submit(ctx context.Context, tenantID uint64, reporterID, reporterType string, sess *types.Session, msg *types.Message, req types.AnswerFeedbackRequest) (*types.AnswerFeedback, error) {
	if tenantID == 0 || reporterID == "" || sess == nil || msg == nil || msg.Role != "assistant" || msg.SessionID != sess.ID {
		return nil, ErrFeedbackForbidden
	}
	if !feedbackCategories[req.Category] || len([]rune(strings.TrimSpace(req.Description))) < 10 || len([]rune(req.Description)) > 2000 || len([]rune(req.QuotedText)) > 1000 {
		return nil, ErrFeedbackInvalid
	}
	if old, err := s.repo.GetForReporter(ctx, tenantID, reporterID, msg.ID); err == nil {
		return old, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	question := ""
	if q, err := s.messages.GetFirstMessageOfUser(ctx, sess.ID); err == nil && q != nil {
		question = q.Content
	}
	refs := types.JSONMap{}
	if b, err := json.Marshal(msg.KnowledgeReferences); err == nil {
		_ = json.Unmarshal(b, &refs)
	}
	f := &types.AnswerFeedback{ID: uuid.NewString(), TenantID: tenantID, SessionID: sess.ID, AssistantMessageID: msg.ID, RequestID: msg.RequestID, ReporterID: reporterID, ReporterType: reporterType, Channel: msg.Channel, AgentID: msg.AgentID, ModelID: msg.ModelID, Category: req.Category, Description: strings.TrimSpace(req.Description), ExpectedCorrection: strings.TrimSpace(req.ExpectedCorrection), QuotedText: strings.TrimSpace(req.QuotedText), QuestionSnapshot: question, AnswerSnapshot: msg.Content, ReferencesSnapshot: refs, Status: types.FeedbackStatusPending, Priority: "normal", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, err
	}
	_ = s.repo.AddEvent(ctx, &types.AnswerFeedbackEvent{TenantID: tenantID, FeedbackID: f.ID, ActorID: reporterID, ActorType: reporterType, EventType: "submitted", Comment: f.Description})
	return f, nil
}
func (s *answerFeedbackService) GetMineForMessage(ctx context.Context, t uint64, u, m string) (*types.AnswerFeedback, error) {
	f, e := s.repo.GetForReporter(ctx, t, u, m)
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return f, e
}
func (s *answerFeedbackService) ListMine(ctx context.Context, t uint64, u string, p, z int) ([]*types.AnswerFeedback, int64, error) {
	if p < 1 {
		p = 1
	}
	if z < 1 || z > 100 {
		z = 20
	}
	return s.repo.ListMine(ctx, t, u, p, z)
}
func (s *answerFeedbackService) List(ctx context.Context, t uint64, q types.AnswerFeedbackListQuery) ([]*types.AnswerFeedback, int64, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 || q.PageSize > 100 {
		q.PageSize = 20
	}
	return s.repo.List(ctx, t, q)
}
func (s *answerFeedbackService) Get(ctx context.Context, t uint64, id string) (*types.AnswerFeedback, error) {
	f, e := s.repo.Get(ctx, t, id)
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, ErrFeedbackNotFound
	}
	return f, e
}
func (s *answerFeedbackService) Update(ctx context.Context, t uint64, actor, id string, req types.AnswerFeedbackAdminUpdate) (*types.AnswerFeedback, error) {
	f, e := s.Get(ctx, t, id)
	if e != nil {
		return nil, e
	}
	if req.Status != "" && !feedbackStatuses[req.Status] {
		return nil, ErrFeedbackInvalid
	}
	old := f.Status
	if req.Status != "" {
		f.Status = req.Status
	}
	if req.Priority != "" {
		f.Priority = req.Priority
	}
	if req.RootCause != "" {
		f.RootCause = req.RootCause
	}
	if req.ResolutionAction != "" {
		f.ResolutionAction = req.ResolutionAction
	}
	if req.PublicReply != "" {
		f.PublicReply = req.PublicReply
	}
	if req.InternalNote != "" {
		f.InternalNote = req.InternalNote
	}
	if f.Status == types.FeedbackStatusResolved {
		if strings.TrimSpace(f.RootCause) == "" || strings.TrimSpace(f.ResolutionAction) == "" || strings.TrimSpace(f.PublicReply) == "" {
			return nil, ErrFeedbackInvalid
		}
		now := time.Now()
		f.ResolvedAt = &now
		f.ResolvedBy = actor
	}
	f.UpdatedAt = time.Now()
	if e = s.repo.Update(ctx, f); e != nil {
		return nil, e
	}
	if old != f.Status || req.PublicReply != "" || req.InternalNote != "" {
		_ = s.repo.AddEvent(ctx, &types.AnswerFeedbackEvent{TenantID: t, FeedbackID: f.ID, ActorID: actor, ActorType: "admin", EventType: "admin_update", Comment: req.PublicReply, Metadata: types.JSONMap{"old_status": old, "new_status": f.Status, "internal_note": req.InternalNote}})
	}
	return f, nil
}
func (s *answerFeedbackService) Comment(ctx context.Context, t uint64, actor, actorType, id, comment string) error {
	comment = strings.TrimSpace(comment)
	if len([]rune(comment)) < 2 || len([]rune(comment)) > 2000 {
		return ErrFeedbackInvalid
	}
	f, e := s.Get(ctx, t, id)
	if e != nil {
		return e
	}
	if actorType != "admin" && f.ReporterID != actor {
		return ErrFeedbackForbidden
	}
	if actorType != "admin" && (f.Status == types.FeedbackStatusResolved || f.Status == types.FeedbackStatusDismissed) {
		return ErrFeedbackInvalid
	}
	if actorType != "admin" && f.Status == types.FeedbackStatusNeedsInfo {
		f.Status = types.FeedbackStatusReviewing
		f.UpdatedAt = time.Now()
		_ = s.repo.Update(ctx, f)
	}
	return s.repo.AddEvent(ctx, &types.AnswerFeedbackEvent{TenantID: t, FeedbackID: id, ActorID: actor, ActorType: actorType, EventType: "comment", Comment: comment})
}
func (s *answerFeedbackService) Reopen(ctx context.Context, t uint64, u, id, comment string) error {
	f, e := s.Get(ctx, t, id)
	if e != nil {
		return e
	}
	if f.ReporterID != u {
		return ErrFeedbackForbidden
	}
	if f.Status != types.FeedbackStatusResolved && f.Status != types.FeedbackStatusDismissed {
		return ErrFeedbackInvalid
	}
	if time.Since(f.UpdatedAt) > 7*24*time.Hour {
		return ErrFeedbackInvalid
	}
	f.Status = types.FeedbackStatusReviewing
	f.UpdatedAt = time.Now()
	if e = s.repo.Update(ctx, f); e != nil {
		return e
	}
	return s.repo.AddEvent(ctx, &types.AnswerFeedbackEvent{TenantID: t, FeedbackID: id, ActorID: u, ActorType: "user", EventType: "reopened", Comment: comment})
}
func (s *answerFeedbackService) Events(ctx context.Context, t uint64, id string) ([]*types.AnswerFeedbackEvent, error) {
	if _, e := s.Get(ctx, t, id); e != nil {
		return nil, e
	}
	return s.repo.ListEvents(ctx, t, id)
}

var _ interfaces.AnswerFeedbackService = (*answerFeedbackService)(nil)
