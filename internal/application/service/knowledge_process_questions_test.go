package service

import (
	"reflect"
	"testing"
)

func TestParseGeneratedQuestionLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		limit   int
		want    []string
	}{
		{
			name:    "skip control token",
			content: "SKIP",
			limit:   3,
			want:    []string{},
		},
		{
			name:    "skip token is case insensitive",
			content: "  skip  ",
			limit:   3,
			want:    []string{},
		},
		{
			name:    "numbering is removed and limit is enforced",
			content: "1. 如何配置知识库的向量模型？\n2) 如何重新生成推荐问题？\n3. 如何检查向量维度？",
			limit:   2,
			want: []string{
				"如何配置知识库的向量模型？",
				"如何重新生成推荐问题？",
			},
		},
		{
			name:    "skip line does not hide valid questions",
			content: "SKIP\n如何确认文档已经完成向量化？",
			limit:   3,
			want:    []string{"如何确认文档已经完成向量化？"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGeneratedQuestionLines(tt.content, tt.limit)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseGeneratedQuestionLines() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
