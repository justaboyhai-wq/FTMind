package cognition

import (
	"context"
	"errors"
	"testing"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

type memoryGatewayStub struct {
	tool    string
	token   string
	binding types.BindingContext
	result  any
	err     error
}

func (m *memoryGatewayStub) Invoke(_ context.Context, tool string, binding types.BindingContext, token string, _ map[string]any, _ string) (any, error) {
	m.tool, m.token, m.binding = tool, token, binding
	return m.result, m.err
}

type knowledgeSearcherStub struct {
	kbIDs   []string
	query   string
	results []*types.SearchResult
	err     error
}

func (s *knowledgeSearcherStub) SearchKnowledge(_ context.Context, knowledgeBaseIDs []string, _ []string, _ []types.TagScope, query string) ([]*types.SearchResult, error) {
	s.kbIDs, s.query = append([]string(nil), knowledgeBaseIDs...), query
	return s.results, s.err
}

type wikiReaderStub struct {
	pages map[string]*types.WikiPage
	err   error
}

func (s *wikiReaderStub) GetPageByID(_ context.Context, id string) (*types.WikiPage, error) {
	return s.pages[id], s.err
}

type documentReaderStub struct {
	documents map[string]*types.Knowledge
	err       error
}

func (s *documentReaderStub) GetKnowledgeByID(_ context.Context, id string) (*types.Knowledge, error) {
	return s.documents[id], s.err
}

type chunkReaderStub struct {
	chunks map[string][]*types.Chunk
	err    error
}

func (s *chunkReaderStub) ListChunksByKnowledgeID(_ context.Context, id string) ([]*types.Chunk, error) {
	return s.chunks[id], s.err
}

func TestDefaultExecutorForwardsMemoryThroughExplicitDataPlane(t *testing.T) {
	memory := &memoryGatewayStub{result: map[string]any{"items": []any{}}}
	executor := NewDefaultExecutor(memory, &knowledgeSearcherStub{}, &wikiReaderStub{}, &documentReaderStub{}, &chunkReaderStub{})
	binding := *scopedBinding([]string{"memory.recall"}, []string{"team:team-1"})
	result, err := executor.ExecuteCognitionTool(context.Background(), Invocation{
		Tool: ToolMemorySearch, Binding: binding, bindingToken: "signed-token", Arguments: map[string]any{"query": "roadmap"}, TraceID: "trace-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || memory.tool != ToolMemorySearch || memory.token != "signed-token" || memory.binding.BindingID != binding.BindingID {
		t.Fatalf("memory invocation lost authority: %#v", memory)
	}
}

func TestDefaultExecutorKnowledgeSearchIsExactAndDefenseInDepthFiltered(t *testing.T) {
	searcher := &knowledgeSearcherStub{results: []*types.SearchResult{
		{ID: "chunk-1", KnowledgeID: "doc-1", KnowledgeBaseID: "kb-1", Content: "allowed"},
		{ID: "chunk-2", KnowledgeID: "doc-2", KnowledgeBaseID: "kb-2", Content: "must-not-leak"},
	}}
	executor := NewDefaultExecutor(&memoryGatewayStub{}, searcher, &wikiReaderStub{}, &documentReaderStub{}, &chunkReaderStub{})
	result, err := executor.ExecuteCognitionTool(context.Background(), Invocation{
		Tool: ToolKnowledgeSearch, Arguments: map[string]any{"knowledge_base_ids": []any{"kb-1"}, "query": "allowed"},
		Binding: *scopedBinding([]string{"knowledge.search"}, []string{"knowledge_base:kb-1"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	items := result.(KnowledgeSearchResult).Items
	if len(items) != 1 || items[0].KnowledgeBaseID != "kb-1" || len(searcher.kbIDs) != 1 || searcher.kbIDs[0] != "kb-1" {
		t.Fatalf("knowledge scope widened: result=%#v requested=%v", items, searcher.kbIDs)
	}
}

func TestDefaultExecutorReadsExactWikiPageAndDocumentChunks(t *testing.T) {
	wiki := &wikiReaderStub{pages: map[string]*types.WikiPage{
		"page-1": {ID: "page-1", TenantID: 7, KnowledgeBaseID: "kb-1", Title: "Page", Status: types.WikiPageStatusPublished, Content: "Wiki body", Version: 4},
	}}
	documents := &documentReaderStub{documents: map[string]*types.Knowledge{
		"doc-1": {ID: "doc-1", TenantID: 7, KnowledgeBaseID: "kb-1", Title: "Document", FilePath: "secret/internal/path"},
	}}
	chunks := &chunkReaderStub{chunks: map[string][]*types.Chunk{
		"doc-1": {{ID: "chunk-1", TenantID: 7, KnowledgeID: "doc-1", Content: "Document body", IsEnabled: true}},
	}}
	executor := NewDefaultExecutor(&memoryGatewayStub{}, &knowledgeSearcherStub{}, wiki, documents, chunks)

	wikiResult, err := executor.ExecuteCognitionTool(context.Background(), Invocation{Tool: ToolWikiGetPage, Arguments: map[string]any{"wiki_page_id": "page-1"}, Binding: *scopedBinding([]string{"wiki.get"}, []string{"wiki_page:page-1"})})
	if err != nil {
		t.Fatal(err)
	}
	if got := wikiResult.(WikiPageResult).Page; got.ID != "page-1" || got.Version != 4 || got.Content != "Wiki body" {
		t.Fatalf("unexpected wiki view: %#v", got)
	}
	documentResult, err := executor.ExecuteCognitionTool(context.Background(), Invocation{Tool: ToolDocumentRead, Arguments: map[string]any{"document_id": "doc-1"}, Binding: *scopedBinding([]string{"document.read"}, []string{"document:doc-1"})})
	if err != nil {
		t.Fatal(err)
	}
	gotDocument := documentResult.(DocumentReadResult)
	if gotDocument.Document.ID != "doc-1" || len(gotDocument.Chunks) != 1 || gotDocument.Chunks[0].Content != "Document body" {
		t.Fatalf("unexpected document view: %#v", gotDocument)
	}
	if gotDocument.Document.FilePath != "" {
		t.Fatal("internal file path escaped document_read")
	}
}

func TestDefaultExecutorRejectsArchivedWikiPagesFromExactReadAndContext(t *testing.T) {
	wiki := &wikiReaderStub{pages: map[string]*types.WikiPage{
		"page-archived": {
			ID: "page-archived", TenantID: 7, KnowledgeBaseID: "kb-1",
			Title: "Revoked memory", Status: types.WikiPageStatusArchived,
			Content: "content that must no longer reach an external agent", Version: 5,
		},
	}}
	executor := NewDefaultExecutor(&memoryGatewayStub{}, &knowledgeSearcherStub{}, wiki, &documentReaderStub{}, &chunkReaderStub{})
	binding := *scopedBinding(
		[]string{"wiki.get", "context.assemble"},
		[]string{"wiki_page:page-archived"},
	)

	_, err := executor.ExecuteCognitionTool(context.Background(), Invocation{
		Tool: ToolWikiGetPage, Binding: binding,
		Arguments: map[string]any{"wiki_page_id": "page-archived"},
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("archived page exact-read must fail closed, got %v", err)
	}

	result, err := executor.ExecuteCognitionTool(context.Background(), Invocation{
		Tool: ToolContextAssemble, Binding: binding,
		Arguments: map[string]any{"asset_scopes": []any{"wiki_page:page-archived"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	assembled := result.(types.ContextPackage)
	if len(assembled.Wiki.Items) != 0 || !assembled.Partial {
		t.Fatalf("archived page leaked into assembled context: %#v", assembled)
	}
}

func TestDefaultExecutorAssemblesSeparatedContextWithProvenanceAndPartialWarnings(t *testing.T) {
	memory := &memoryGatewayStub{result: []types.ContextItem{{ID: "mem-1", Type: "l1", Content: "Remember this"}}}
	searcher := &knowledgeSearcherStub{results: []*types.SearchResult{{ID: "chunk-1", KnowledgeID: "doc-1", KnowledgeBaseID: "kb-1", Content: "RAG body", Score: 0.9}}}
	wiki := &wikiReaderStub{pages: map[string]*types.WikiPage{"page-1": {ID: "page-1", TenantID: 7, KnowledgeBaseID: "kb-1", Title: "Wiki", Status: types.WikiPageStatusPublished, Content: "Wiki body", Version: 2}}}
	documents := &documentReaderStub{documents: map[string]*types.Knowledge{"doc-1": {ID: "doc-1", TenantID: 7, KnowledgeBaseID: "kb-1", Title: "Doc"}}}
	chunks := &chunkReaderStub{chunks: map[string][]*types.Chunk{"doc-1": {{ID: "raw-1", TenantID: 7, KnowledgeID: "doc-1", Content: "Raw body", IsEnabled: true}}}}
	executor := NewDefaultExecutor(memory, searcher, wiki, documents, chunks)
	binding := *scopedBinding([]string{"context.assemble", "memory.context", "knowledge.search", "wiki.get", "document.read"}, []string{"team:team-1", "knowledge_base:kb-1", "wiki_page:page-1", "document:doc-1"})
	result, err := executor.ExecuteCognitionTool(context.Background(), Invocation{
		Tool: ToolContextAssemble, Binding: binding, bindingToken: "signed-token", TraceID: "trace-1",
		Arguments: map[string]any{"query": "roadmap", "include_memory": true, "asset_scopes": []any{"knowledge_base:kb-1", "wiki_page:page-1", "document:doc-1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	contextPackage := result.(types.ContextPackage)
	if contextPackage.SchemaVersion != types.ContextPackageSchemaVersion || contextPackage.BindingID != binding.BindingID || contextPackage.TraceID != "trace-1" {
		t.Fatalf("unexpected package identity: %#v", contextPackage)
	}
	if len(contextPackage.Memory.Items) != 1 || len(contextPackage.RAG.Items) != 1 || len(contextPackage.Wiki.Items) != 1 || len(contextPackage.Raw.Items) != 1 {
		t.Fatalf("sections were merged or dropped: %#v", contextPackage)
	}
	if len(contextPackage.Provenance) < 4 || len(contextPackage.UsedAssets) < 3 || contextPackage.Partial {
		t.Fatalf("missing lineage or unexpected partial result: %#v", contextPackage)
	}

	wiki.err = errors.New("wiki unavailable")
	partial, err := executor.ExecuteCognitionTool(context.Background(), Invocation{
		Tool: ToolContextAssemble, Binding: binding, bindingToken: "signed-token",
		Arguments: map[string]any{"query": "roadmap", "asset_scopes": []any{"wiki_page:page-1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	partialPackage := partial.(types.ContextPackage)
	if !partialPackage.Partial || len(partialPackage.Warnings) == 0 {
		t.Fatalf("partial failure was hidden: %#v", partialPackage)
	}
}
