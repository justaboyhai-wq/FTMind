// Package search implements the `keystone search` command tree:
// chunks / kb / docs / sessions.
package search

import (
	"github.com/spf13/cobra"

	"github.com/justaboyhai-wq/keystone/cli/internal/cmdutil"
)

// NewCmdSearch builds the `keystone search` parent. Pure dispatcher to the
// four subcommands - users must pick a verb (chunks / kb / docs / sessions).
func NewCmdSearch(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search across chunks, knowledge bases, documents, or sessions",
		Long: `Verb-noun search tree:

  search chunks   "<q>" --kb X     hybrid retrieval (RAG search)
  search kb       "<q>"            find KBs by name / description
  search docs     "<q>" --kb X     find documents inside a KB
  search sessions "<q>"            find chat sessions by title / description`,
		Example: `  keystone search chunks "what is RAG?" --kb engineering
  keystone search kb     "marketing"
  keystone search docs   "Q3 forecast" --kb finance
  keystone search sessions "onboarding"`,
	}

	cmd.AddCommand(NewCmdChunks(f))
	cmd.AddCommand(NewCmdKB(f))
	cmd.AddCommand(NewCmdDocs(f))
	cmd.AddCommand(NewCmdSessions(f))
	return cmd
}

// emptyContentSearchHint returns an actionable note when a KB-scoped content
// search (chunks / docs) yields zero results, so an agent can distinguish
// "no match" from "the KB has no indexed content". Empty when n > 0 so it
// never adds noise to real results.
func emptyContentSearchHint(n int) string {
	if n > 0 {
		return ""
	}
	return "0 results: this may be no match, OR the KB has no indexed chunks. " +
		"Check `keystone kb status <kb>` (chunk_count) and `keystone doc list --kb <kb>` (parse_status); " +
		"documents in parse_status=draft are not indexed — run `keystone doc reparse <doc-id>`."
}
