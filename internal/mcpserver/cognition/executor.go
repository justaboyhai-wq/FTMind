package cognition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/justaboyhai-wq/fmind/internal/types"
)

const (
	maxKnowledgeResults  = 20
	maxDocumentChunks    = 256
	maxDocumentReadChars = 200_000
	maxWikiReadChars     = 200_000
	defaultSectionTokens = 1_500
	maxSectionTokens     = 8_192
)

type MemoryGateway interface {
	Invoke(context.Context, string, types.BindingContext, string, map[string]any, string) (any, error)
}

type KnowledgeSearcher interface {
	SearchKnowledge(context.Context, []string, []string, []types.TagScope, string) ([]*types.SearchResult, error)
}

type WikiReader interface {
	GetPageByID(context.Context, string) (*types.WikiPage, error)
}

type DocumentReader interface {
	GetKnowledgeByID(context.Context, string) (*types.Knowledge, error)
}

type DocumentChunkReader interface {
	ListChunksByKnowledgeID(context.Context, string) ([]*types.Chunk, error)
}

type DefaultExecutor struct {
	memory    MemoryGateway
	knowledge KnowledgeSearcher
	wiki      WikiReader
	documents DocumentReader
	chunks    DocumentChunkReader
}

func NewDefaultExecutor(memory MemoryGateway, knowledge KnowledgeSearcher, wiki WikiReader, documents DocumentReader, chunks DocumentChunkReader) *DefaultExecutor {
	return &DefaultExecutor{memory: memory, knowledge: knowledge, wiki: wiki, documents: documents, chunks: chunks}
}

type KnowledgeSearchResult struct {
	Items []*types.SearchResult `json:"items"`
}

type WikiPageView struct {
	ID              string `json:"id"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	Summary         string `json:"summary,omitempty"`
	Content         string `json:"content"`
	Version         int    `json:"version"`
}

type WikiPageResult struct {
	Page WikiPageView `json:"page"`
}

type DocumentView struct {
	ID              string `json:"id"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
	Title           string `json:"title"`
	Description     string `json:"description,omitempty"`
	Type            string `json:"type,omitempty"`
	Source          string `json:"source,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
	FilePath        string `json:"-"`
}

type DocumentChunkView struct {
	ID      string `json:"id"`
	Index   int    `json:"index"`
	Content string `json:"content"`
	Version string `json:"version,omitempty"`
}

type DocumentReadResult struct {
	Document  DocumentView        `json:"document"`
	Chunks    []DocumentChunkView `json:"chunks"`
	Truncated bool                `json:"truncated"`
}

func (e *DefaultExecutor) ExecuteCognitionTool(ctx context.Context, invocation Invocation) (any, error) {
	switch invocation.Tool {
	case ToolMemoryGetContext, ToolMemorySearch, ToolMemoryCaptureTurn, ToolMemoryConfirmCandidate:
		if e.memory == nil {
			return nil, errors.New("memory gateway is unavailable")
		}
		return e.memory.Invoke(ctx, invocation.Tool, invocation.Binding, invocation.bindingToken, cloneArguments(invocation.Arguments), invocation.TraceID)
	case ToolKnowledgeSearch:
		return e.searchKnowledge(ctx, invocation)
	case ToolWikiGetPage:
		return e.readWikiPage(ctx, invocation)
	case ToolDocumentRead:
		return e.readDocument(ctx, invocation)
	case ToolContextAssemble:
		return e.assembleContext(ctx, invocation)
	default:
		return nil, fmt.Errorf("%w: unsupported executor tool", ErrInvalidRequest)
	}
}

func (e *DefaultExecutor) searchKnowledge(ctx context.Context, invocation Invocation) (KnowledgeSearchResult, error) {
	if e.knowledge == nil {
		return KnowledgeSearchResult{}, errors.New("knowledge search is unavailable")
	}
	ids, err := stringSlice(invocation.Arguments["knowledge_base_ids"])
	if err != nil || len(ids) == 0 {
		return KnowledgeSearchResult{}, fmt.Errorf("%w: knowledge_base_ids is required", ErrInvalidRequest)
	}
	query, _ := invocation.Arguments["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return KnowledgeSearchResult{}, fmt.Errorf("%w: query is required", ErrInvalidRequest)
	}
	results, err := e.knowledge.SearchKnowledge(ctx, ids, nil, nil, query)
	if err != nil {
		return KnowledgeSearchResult{}, err
	}
	allowed := stringSet(ids)
	filtered := make([]*types.SearchResult, 0, minInt(len(results), maxKnowledgeResults))
	for _, result := range results {
		if result == nil {
			continue
		}
		if _, ok := allowed[result.KnowledgeBaseID]; !ok {
			continue
		}
		copyValue := *result
		copyValue.Content, _ = truncateRunes(copyValue.Content, maxDocumentReadChars/maxKnowledgeResults)
		filtered = append(filtered, &copyValue)
		if len(filtered) == maxKnowledgeResults {
			break
		}
	}
	return KnowledgeSearchResult{Items: filtered}, nil
}

func (e *DefaultExecutor) readWikiPage(ctx context.Context, invocation Invocation) (WikiPageResult, error) {
	if e.wiki == nil {
		return WikiPageResult{}, errors.New("wiki reader is unavailable")
	}
	id, _ := invocation.Arguments["wiki_page_id"].(string)
	page, err := e.wiki.GetPageByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return WikiPageResult{}, err
	}
	if page == nil || page.ID != id || page.TenantID != invocation.Binding.TenantID {
		return WikiPageResult{}, ErrForbidden
	}
	content, _ := truncateRunes(page.Content, maxWikiReadChars)
	return WikiPageResult{Page: WikiPageView{
		ID: page.ID, KnowledgeBaseID: page.KnowledgeBaseID, Slug: page.Slug,
		Title: page.Title, Summary: page.Summary, Content: content, Version: page.Version,
	}}, nil
}

func (e *DefaultExecutor) readDocument(ctx context.Context, invocation Invocation) (DocumentReadResult, error) {
	if e.documents == nil || e.chunks == nil {
		return DocumentReadResult{}, errors.New("document reader is unavailable")
	}
	id, _ := invocation.Arguments["document_id"].(string)
	id = strings.TrimSpace(id)
	document, err := e.documents.GetKnowledgeByID(ctx, id)
	if err != nil {
		return DocumentReadResult{}, err
	}
	if document == nil || document.ID != id || document.TenantID != invocation.Binding.TenantID {
		return DocumentReadResult{}, ErrForbidden
	}
	chunks, err := e.chunks.ListChunksByKnowledgeID(ctx, id)
	if err != nil {
		return DocumentReadResult{}, err
	}
	maxChars := integerArgument(invocation.Arguments, "max_chars", maxDocumentReadChars, 1, maxDocumentReadChars)
	views := make([]DocumentChunkView, 0, minInt(len(chunks), maxDocumentChunks))
	remaining := maxChars
	truncated := false
	for _, chunk := range chunks {
		if chunk == nil || chunk.KnowledgeID != id || chunk.TenantID != invocation.Binding.TenantID || !chunk.IsEnabled {
			continue
		}
		if len(views) == maxDocumentChunks || remaining <= 0 {
			truncated = true
			break
		}
		content, wasTruncated := truncateRunes(chunk.Content, remaining)
		remaining -= len([]rune(content))
		truncated = truncated || wasTruncated
		views = append(views, DocumentChunkView{
			ID: chunk.ID, Index: chunk.ChunkIndex, Content: content,
			Version: chunk.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		})
	}
	return DocumentReadResult{
		Document: DocumentView{
			ID: document.ID, KnowledgeBaseID: document.KnowledgeBaseID, Title: document.Title,
			Description: document.Description, Type: document.Type, Source: document.Source,
			UpdatedAt: document.UpdatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		},
		Chunks: views, Truncated: truncated,
	}, nil
}

func (e *DefaultExecutor) assembleContext(ctx context.Context, invocation Invocation) (types.ContextPackage, error) {
	assetScopes, err := stringSlice(invocation.Arguments["asset_scopes"])
	if err != nil || len(assetScopes) == 0 {
		return types.ContextPackage{}, fmt.Errorf("%w: asset_scopes is required", ErrInvalidRequest)
	}
	packageValue := newContextPackage(invocation)
	query, _ := invocation.Arguments["query"].(string)
	query = strings.TrimSpace(query)
	includeMemory, _ := invocation.Arguments["include_memory"].(bool)

	if includeMemory {
		if e.memory == nil {
			markPartial(&packageValue, "memory context unavailable")
		} else {
			result, callErr := e.memory.Invoke(ctx, ToolMemoryGetContext, invocation.Binding, invocation.bindingToken, map[string]any{"query": query}, invocation.TraceID)
			if callErr != nil {
				markPartial(&packageValue, "memory context unavailable")
			} else {
				for _, item := range memoryContextItems(result) {
					appendContextItem(&packageValue.Memory, item, packageValue.Budgets["memory"].MaxTokens)
					packageValue.Provenance = append(packageValue.Provenance, types.ContextProvenance{Section: "memory", Asset: "team:" + invocation.Binding.TeamID, SourceID: item.ID, Version: item.Version})
				}
			}
		}
		packageValue.PermissionDecisions = append(packageValue.PermissionDecisions, types.ContextPermissionDecision{Tool: ToolMemoryGetContext, AssetScope: "team:" + invocation.Binding.TeamID, Allowed: true})
	}

	for _, scope := range assetScopes {
		parts := strings.SplitN(scope, ":", 2)
		if len(parts) != 2 {
			markPartial(&packageValue, "invalid asset scope skipped")
			continue
		}
		kind, id := parts[0], parts[1]
		packageValue.PermissionDecisions = append(packageValue.PermissionDecisions, types.ContextPermissionDecision{Tool: ToolContextAssemble, AssetScope: scope, Allowed: true})
		switch kind {
		case "knowledge_base":
			result, callErr := e.searchKnowledge(ctx, Invocation{Tool: ToolKnowledgeSearch, Binding: invocation.Binding, Arguments: map[string]any{"knowledge_base_ids": []string{id}, "query": query}})
			if callErr != nil {
				markPartial(&packageValue, "knowledge search unavailable for "+scope)
				continue
			}
			for _, item := range result.Items {
				contextItem := types.ContextItem{ID: item.ID, Type: "rag_chunk", Title: item.KnowledgeTitle, Content: item.Content, Score: item.Score}
				appendContextItem(&packageValue.RAG, contextItem, packageValue.Budgets["rag"].MaxTokens)
				packageValue.Provenance = append(packageValue.Provenance, types.ContextProvenance{Section: "rag", Asset: scope, SourceID: item.KnowledgeID})
			}
			packageValue.UsedAssets = appendUniqueAsset(packageValue.UsedAssets, types.ContextAssetVersion{AssetScope: scope})
		case "wiki_page":
			result, callErr := e.readWikiPage(ctx, Invocation{Tool: ToolWikiGetPage, Binding: invocation.Binding, Arguments: map[string]any{"wiki_page_id": id}})
			if callErr != nil {
				markPartial(&packageValue, "wiki page unavailable for "+scope)
				continue
			}
			version := strconv.Itoa(result.Page.Version)
			item := types.ContextItem{ID: result.Page.ID, Type: "wiki_page", Title: result.Page.Title, Content: result.Page.Content, Version: version}
			appendContextItem(&packageValue.Wiki, item, packageValue.Budgets["wiki"].MaxTokens)
			packageValue.Provenance = append(packageValue.Provenance, types.ContextProvenance{Section: "wiki", Asset: scope, SourceID: result.Page.ID, Version: version})
			packageValue.UsedAssets = appendUniqueAsset(packageValue.UsedAssets, types.ContextAssetVersion{AssetScope: scope, Version: version})
		case "document":
			result, callErr := e.readDocument(ctx, Invocation{Tool: ToolDocumentRead, Binding: invocation.Binding, Arguments: map[string]any{"document_id": id, "max_chars": packageValue.Budgets["raw"].MaxTokens * 4}})
			if callErr != nil {
				markPartial(&packageValue, "document unavailable for "+scope)
				continue
			}
			for _, chunk := range result.Chunks {
				item := types.ContextItem{ID: chunk.ID, Type: "document_chunk", Title: result.Document.Title, Content: chunk.Content, Version: chunk.Version}
				appendContextItem(&packageValue.Raw, item, packageValue.Budgets["raw"].MaxTokens)
				packageValue.Provenance = append(packageValue.Provenance, types.ContextProvenance{Section: "raw", Asset: scope, SourceID: chunk.ID, Version: chunk.Version})
			}
			packageValue.UsedAssets = appendUniqueAsset(packageValue.UsedAssets, types.ContextAssetVersion{AssetScope: scope, Version: result.Document.UpdatedAt})
		}
	}
	updateContextBudgets(&packageValue)
	return packageValue, nil
}

func newContextPackage(invocation Invocation) types.ContextPackage {
	budgets := map[string]types.ContextBudget{
		"memory": {MaxTokens: sectionBudget(invocation.Arguments, "memory")},
		"rag":    {MaxTokens: sectionBudget(invocation.Arguments, "rag")},
		"wiki":   {MaxTokens: sectionBudget(invocation.Arguments, "wiki")},
		"raw":    {MaxTokens: sectionBudget(invocation.Arguments, "raw")},
		"skill":  {MaxTokens: sectionBudget(invocation.Arguments, "skill")},
	}
	return types.ContextPackage{
		SchemaVersion: types.ContextPackageSchemaVersion, BindingID: invocation.Binding.BindingID,
		PolicyVersion: invocation.Binding.PolicyVersion, TraceID: invocation.TraceID,
		Memory: types.ContextSection{Items: []types.ContextItem{}}, RAG: types.ContextSection{Items: []types.ContextItem{}},
		Wiki: types.ContextSection{Items: []types.ContextItem{}}, Raw: types.ContextSection{Items: []types.ContextItem{}},
		Skill: types.ContextSection{Items: []types.ContextItem{}}, Budgets: budgets,
		Provenance: []types.ContextProvenance{}, Conflicts: []types.ContextConflict{}, Warnings: []string{},
		PermissionDecisions: []types.ContextPermissionDecision{}, UsedAssets: []types.ContextAssetVersion{},
	}
}

func sectionBudget(arguments map[string]any, section string) int {
	raw, ok := arguments["budgets"].(map[string]any)
	if !ok {
		return defaultSectionTokens
	}
	return integerArgument(raw, section, defaultSectionTokens, 1, maxSectionTokens)
}

func appendContextItem(section *types.ContextSection, item types.ContextItem, maxTokens int) {
	remaining := maxTokens - section.UsedTokens
	if remaining <= 0 {
		return
	}
	item.Content, _ = truncateRunes(item.Content, remaining*4)
	used := approximateTokens(item.Content)
	if used == 0 && item.Content != "" {
		used = 1
	}
	section.Items = append(section.Items, item)
	section.UsedTokens += used
}

func updateContextBudgets(value *types.ContextPackage) {
	used := map[string]int{"memory": value.Memory.UsedTokens, "rag": value.RAG.UsedTokens, "wiki": value.Wiki.UsedTokens, "raw": value.Raw.UsedTokens, "skill": value.Skill.UsedTokens}
	for section, budget := range value.Budgets {
		budget.UsedTokens = used[section]
		value.Budgets[section] = budget
	}
}

func memoryContextItems(value any) []types.ContextItem {
	switch typed := value.(type) {
	case []types.ContextItem:
		return append([]types.ContextItem(nil), typed...)
	case types.ContextSection:
		return append([]types.ContextItem(nil), typed.Items...)
	default:
		encoded, err := json.Marshal(value)
		if err != nil || string(encoded) == "null" {
			return nil
		}
		return []types.ContextItem{{ID: "memory-context", Type: "memory_context", Content: string(encoded)}}
	}
}

func markPartial(value *types.ContextPackage, warning string) {
	value.Partial = true
	value.Warnings = append(value.Warnings, warning)
}

func appendUniqueAsset(values []types.ContextAssetVersion, candidate types.ContextAssetVersion) []types.ContextAssetVersion {
	for _, value := range values {
		if value.AssetScope == candidate.AssetScope && value.Version == candidate.Version {
			return values
		}
	}
	return append(values, candidate)
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func integerArgument(arguments map[string]any, key string, fallback, minimum, maximum int) int {
	value := fallback
	switch raw := arguments[key].(type) {
	case int:
		value = raw
	case int64:
		value = int(raw)
	case float64:
		value = int(raw)
	case json.Number:
		if parsed, err := raw.Int64(); err == nil {
			value = int(parsed)
		}
	}
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func truncateRunes(value string, maximum int) (string, bool) {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value, false
	}
	return string(runes[:maximum]), true
}

func approximateTokens(value string) int {
	return (len([]rune(value)) + 3) / 4
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
