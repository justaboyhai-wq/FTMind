package web_search

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseArkSearchResponse(t *testing.T) {
	body := []byte("{\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"```json\\n[{\\\"title\\\":\\\"Example\\\",\\\"url\\\":\\\"https://example.com\\\",\\\"snippet\\\":\\\"Example snippet\\\"}]\\n```\"}]}]}")
	results, err := parseArkSearchResponse(body)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "Example", results[0].Title)
	require.Equal(t, "https://example.com", results[0].URL)
	require.Equal(t, "ark", results[0].Source)
}
