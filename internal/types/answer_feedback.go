package types

import "time"

const (
	FeedbackStatusPending   = "pending"
	FeedbackStatusReviewing = "reviewing"
	FeedbackStatusNeedsInfo = "needs_info"
	FeedbackStatusFixing    = "fixing"
	FeedbackStatusResolved  = "resolved"
	FeedbackStatusDismissed = "dismissed"
)

type AnswerFeedback struct {
	ID                 string      `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID           uint64      `json:"tenant_id" gorm:"index"`
	SessionID          string      `json:"session_id" gorm:"type:varchar(36);index"`
	AssistantMessageID string      `json:"assistant_message_id" gorm:"type:varchar(36);index"`
	RequestID          string      `json:"request_id,omitempty" gorm:"type:varchar(128)"`
	ReporterID         string      `json:"reporter_id" gorm:"type:varchar(512);index"`
	ReporterType       string      `json:"reporter_type" gorm:"type:varchar(32)"`
	Channel            string      `json:"channel,omitempty" gorm:"type:varchar(50)"`
	AgentID            string      `json:"agent_id,omitempty" gorm:"type:varchar(36)"`
	ModelID            string      `json:"model_id,omitempty" gorm:"type:varchar(64)"`
	Category           string      `json:"category" gorm:"type:varchar(40)"`
	Description        string      `json:"description" gorm:"type:text"`
	ExpectedCorrection string      `json:"expected_correction,omitempty" gorm:"type:text"`
	QuotedText         string      `json:"quoted_text,omitempty" gorm:"type:text"`
	QuestionSnapshot   string      `json:"question_snapshot" gorm:"type:text"`
	AnswerSnapshot     string      `json:"answer_snapshot" gorm:"type:text"`
	ReferencesSnapshot JSONMap     `json:"references_snapshot,omitempty" gorm:"type:jsonb"`
	KnowledgeBaseIDs   StringArray `json:"knowledge_base_ids,omitempty" gorm:"type:jsonb"`
	Status             string      `json:"status" gorm:"type:varchar(20);index"`
	Priority           string      `json:"priority" gorm:"type:varchar(20);default:'normal'"`
	RootCause          string      `json:"root_cause,omitempty" gorm:"type:varchar(40)"`
	ResolutionAction   string      `json:"resolution_action,omitempty" gorm:"type:varchar(40)"`
	PublicReply        string      `json:"public_reply,omitempty" gorm:"type:text"`
	InternalNote       string      `json:"internal_note,omitempty" gorm:"type:text"`
	AssignedTo         string      `json:"assigned_to,omitempty" gorm:"type:varchar(36)"`
	ResolvedBy         string      `json:"resolved_by,omitempty" gorm:"type:varchar(36)"`
	ResolvedAt         *time.Time  `json:"resolved_at,omitempty"`
	CreatedAt          time.Time   `json:"created_at"`
	UpdatedAt          time.Time   `json:"updated_at"`
	DeletedAt          *time.Time  `json:"-" gorm:"index"`
}

func (AnswerFeedback) TableName() string { return "answer_feedbacks" }

type AnswerFeedbackEvent struct {
	ID         string    `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID   uint64    `json:"tenant_id" gorm:"index"`
	FeedbackID string    `json:"feedback_id" gorm:"type:varchar(36);index"`
	ActorID    string    `json:"actor_id" gorm:"type:varchar(512)"`
	ActorType  string    `json:"actor_type" gorm:"type:varchar(32)"`
	EventType  string    `json:"event_type" gorm:"type:varchar(40)"`
	Comment    string    `json:"comment,omitempty" gorm:"type:text"`
	Metadata   JSONMap   `json:"metadata,omitempty" gorm:"type:jsonb"`
	CreatedAt  time.Time `json:"created_at"`
}

func (AnswerFeedbackEvent) TableName() string { return "answer_feedback_events" }

type FeedbackSummary struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	Category    string `json:"category"`
	PublicReply string `json:"public_reply,omitempty"`
}
type AnswerFeedbackRequest struct {
	Category           string `json:"category"`
	Description        string `json:"description"`
	ExpectedCorrection string `json:"expected_correction,omitempty"`
	QuotedText         string `json:"quoted_text,omitempty"`
}
type AnswerFeedbackCommentRequest struct {
	Comment string `json:"comment"`
}
type AnswerFeedbackAdminUpdate struct {
	Status           string `json:"status"`
	Priority         string `json:"priority,omitempty"`
	RootCause        string `json:"root_cause,omitempty"`
	ResolutionAction string `json:"resolution_action,omitempty"`
	PublicReply      string `json:"public_reply,omitempty"`
	InternalNote     string `json:"internal_note,omitempty"`
}
type AnswerFeedbackListQuery struct {
	Status   string
	Category string
	Page     int
	PageSize int
}
