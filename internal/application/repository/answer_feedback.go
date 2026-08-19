package repository

import (
	"context"
	"github.com/google/uuid"
	"github.com/justaboyhai-wq/fmind/internal/types"
	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
	"gorm.io/gorm"
)

type answerFeedbackRepository struct{ db *gorm.DB }

func NewAnswerFeedbackRepository(db *gorm.DB) interfaces.AnswerFeedbackRepository {
	return &answerFeedbackRepository{db}
}
func (r *answerFeedbackRepository) Create(ctx context.Context, f *types.AnswerFeedback) error {
	if f.ID == "" {
		f.ID = uuid.NewString()
	}
	return r.db.WithContext(ctx).Create(f).Error
}
func (r *answerFeedbackRepository) Get(ctx context.Context, t uint64, id string) (*types.AnswerFeedback, error) {
	var f types.AnswerFeedback
	err := r.db.WithContext(ctx).Where("tenant_id=? AND id=?", t, id).First(&f).Error
	return &f, err
}
func (r *answerFeedbackRepository) GetForReporter(ctx context.Context, t uint64, u, m string) (*types.AnswerFeedback, error) {
	var f types.AnswerFeedback
	err := r.db.WithContext(ctx).Where("tenant_id=? AND reporter_id=? AND assistant_message_id=?", t, u, m).First(&f).Error
	return &f, err
}
func (r *answerFeedbackRepository) ListMine(ctx context.Context, t uint64, u string, p, s int) ([]*types.AnswerFeedback, int64, error) {
	var a []*types.AnswerFeedback
	var n int64
	q := r.db.WithContext(ctx).Model(&types.AnswerFeedback{}).Where("tenant_id=? AND reporter_id=?", t, u)
	q.Count(&n)
	err := q.Order("created_at DESC").Offset((p - 1) * s).Limit(s).Find(&a).Error
	return a, n, err
}
func (r *answerFeedbackRepository) List(ctx context.Context, t uint64, q types.AnswerFeedbackListQuery) ([]*types.AnswerFeedback, int64, error) {
	var a []*types.AnswerFeedback
	var n int64
	db := r.db.WithContext(ctx).Model(&types.AnswerFeedback{}).Where("tenant_id=?", t)
	if q.Status != "" {
		db = db.Where("status=?", q.Status)
	}
	if q.Category != "" {
		db = db.Where("category=?", q.Category)
	}
	db.Count(&n)
	err := db.Order("created_at DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&a).Error
	return a, n, err
}
func (r *answerFeedbackRepository) Update(ctx context.Context, f *types.AnswerFeedback) error {
	return r.db.WithContext(ctx).Save(f).Error
}
func (r *answerFeedbackRepository) AddEvent(ctx context.Context, e *types.AnswerFeedbackEvent) error {
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	return r.db.WithContext(ctx).Create(e).Error
}
func (r *answerFeedbackRepository) ListEvents(ctx context.Context, t uint64, id string) ([]*types.AnswerFeedbackEvent, error) {
	var a []*types.AnswerFeedbackEvent
	err := r.db.WithContext(ctx).Where("tenant_id=? AND feedback_id=?", t, id).Order("created_at ASC").Find(&a).Error
	return a, err
}
