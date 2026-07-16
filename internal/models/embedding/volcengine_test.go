package embedding

import "testing"

func TestVolcengineMultimodalEmbeddingURL(t *testing.T) {
	tests := map[string]string{
		"standard Ark API base": "https://ark.cn-beijing.volces.com/api/v3",
		"AgentPlan API base":    "https://ark.cn-beijing.volces.com/api/plan/v3",
		"Coding Plan API base":  "https://ark.cn-beijing.volces.com/api/coding/v3",
		"legacy host-only base": "https://ark.cn-beijing.volces.com",
	}

	for name, baseURL := range tests {
		t.Run(name, func(t *testing.T) {
			got := volcengineMultimodalEmbeddingURL(baseURL)
			want := baseURL + VolcengineMultimodalEmbeddingPath
			if name == "legacy host-only base" {
				want = baseURL + volcengineStandardAPIPath + VolcengineMultimodalEmbeddingPath
			}
			if got != want {
				t.Fatalf("endpoint = %q, want %q", got, want)
			}
		})
	}
}
