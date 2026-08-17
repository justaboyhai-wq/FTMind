package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/justaboyhai-wq/fmind/internal/types/interfaces"
)

type wikiIssueToolServiceStub struct {
	interfaces.WikiPageService
	calls int
	kbID  string
}

func (s *wikiIssueToolServiceStub) UpdateIssueStatus(_ context.Context, kbID, _, _ string) error {
	s.calls++
	s.kbID = kbID
	return nil
}

func TestWikiUpdateIssueToolRejectsKnowledgeBaseOutsideServerScope(t *testing.T) {
	service := &wikiIssueToolServiceStub{}
	tool := NewWikiUpdateIssueTool(service, []string{"kb-allowed"})
	result, err := tool.Execute(context.Background(), json.RawMessage(`{
		"knowledge_base_id":"kb-secret","issue_id":"issue-1","status":"resolved"
	}`))
	if err != nil || result.Success || service.calls != 0 {
		t.Fatalf("result=%#v err=%v calls=%d", result, err, service.calls)
	}

	result, err = tool.Execute(context.Background(), json.RawMessage(`{
		"knowledge_base_id":"kb-allowed","issue_id":"issue-1","status":"resolved"
	}`))
	if err != nil || !result.Success || service.calls != 1 || service.kbID != "kb-allowed" {
		t.Fatalf("result=%#v err=%v calls=%d kb=%s", result, err, service.calls, service.kbID)
	}
}
