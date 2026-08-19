package types

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBaoanPolicyQABuiltinAgentContract(t *testing.T) {
	configDir := filepath.Join("..", "..", "config")
	data, err := os.ReadFile(filepath.Join(configDir, "builtin_agents.yaml"))
	require.NoError(t, err)

	var file builtinAgentsFile
	require.NoError(t, yaml.Unmarshal(data, &file))

	var policyAgent *BuiltinAgentEntry
	for i := range file.BuiltinAgents {
		if file.BuiltinAgents[i].ID == "builtin-baoan-policy-qa" {
			policyAgent = &file.BuiltinAgents[i]
			break
		}
	}
	require.NotNil(t, policyAgent)
	require.True(t, policyAgent.IsBuiltin)
	require.Equal(t, "宝安政策问答", policyAgent.I18n["zh-CN"].Name)
	require.Equal(t, "baoan_policy_qa", policyAgent.Config.SystemPromptID)
	require.Contains(t, GetBuiltinAgentIDs(), BuiltinBaoanPolicyQAID)
	require.Equal(t, "selected", policyAgent.Config.KBSelectionMode)
	require.False(t, policyAgent.Config.WebSearchEnabled)

	allowed := make(map[string]bool, len(policyAgent.Config.AllowedTools))
	for _, tool := range policyAgent.Config.AllowedTools {
		allowed[tool] = true
	}
	for _, tool := range []string{
		"wiki_search", "wiki_read_page", "wiki_read_source_doc",
		"knowledge_search", "grep_chunks", "list_knowledge_chunks", "get_document_info",
	} {
		require.Truef(t, allowed[tool], "required read tool %q is missing", tool)
	}
	for _, tool := range []string{
		"wiki_write_page", "wiki_replace_text", "wiki_rename_page", "wiki_delete_page",
	} {
		require.Falsef(t, allowed[tool], "write tool %q must not be granted", tool)
	}

	prompts, err := os.ReadFile(filepath.Join(configDir, "prompt_templates", "agent_system_prompt.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(prompts), `id: "baoan_policy_qa"`)
	require.Contains(t, string(prompts), "当前可申报")
	require.NotContains(t, string(prompts), "波粒二象")
	require.True(t, strings.Contains(string(prompts), "official") && strings.Contains(string(prompts), "computed") && strings.Contains(string(prompts), "derived_ai"))
}
