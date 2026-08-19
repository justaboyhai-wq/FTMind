package interfaces

import (
	"context"
	"github.com/justaboyhai-wq/fmind/internal/types"
)

type AnswerFeedbackRepository interface {
	Create(context.Context, *types.AnswerFeedback) error
	Get(context.Context, uint64, string) (*types.AnswerFeedback, error)
	GetForReporter(context.Context, uint64, string, string) (*types.AnswerFeedback, error)
	ListMine(context.Context, uint64, string, int, int) ([]*types.AnswerFeedback, int64, error)
	List(context.Context, uint64, types.AnswerFeedbackListQuery) ([]*types.AnswerFeedback, int64, error)
	Update(context.Context, *types.AnswerFeedback) error
	AddEvent(context.Context, *types.AnswerFeedbackEvent) error
	ListEvents(context.Context, uint64, string) ([]*types.AnswerFeedbackEvent, error)
}
type AnswerFeedbackService interface {
	Submit(context.Context, uint64, string, string, *types.Session, *types.Message, types.AnswerFeedbackRequest) (*types.AnswerFeedback, error)
	GetMineForMessage(context.Context, uint64, string, string) (*types.AnswerFeedback, error)
	ListMine(context.Context, uint64, string, int, int) ([]*types.AnswerFeedback, int64, error)
	List(context.Context, uint64, types.AnswerFeedbackListQuery) ([]*types.AnswerFeedback, int64, error)
	Get(context.Context, uint64, string) (*types.AnswerFeedback, error)
	Update(context.Context, uint64, string, string, types.AnswerFeedbackAdminUpdate) (*types.AnswerFeedback, error)
	Comment(context.Context, uint64, string, string, string, string) error
	Reopen(context.Context, uint64, string, string, string) error
	Events(context.Context, uint64, string) ([]*types.AnswerFeedbackEvent, error)
}
