package normalize

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/detail"
)

func loadSampleDetail(t *testing.T) detail.Decoded {
	t.Helper()
	b, err := os.ReadFile("../../testdata/detail-12846556.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := detail.Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestNormalizeSample(t *testing.T) {
	d := loadSampleDetail(t)
	p, err := Policy(d, time.Date(2026, 8, 19, 0, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p.Markdown, "第一章") || !strings.Contains(p.Markdown, "第三十条") {
		t.Fatal("policy structure lost")
	}
	if p.Structured.Official.Theme != "公安、安全、司法" || p.Structured.Official.ServiceObjects[0] != "其他" {
		t.Fatalf("official=%+v", p.Structured.Official)
	}
	if len(p.Relations) != 4 || p.Relations[1].RelationType != "text_interpretation" || p.Relations[2].SourceLabel != "意见征集" {
		t.Fatalf("relations=%+v", p.Relations)
	}
}

func TestApplicationStatus(t *testing.T) {
	n := time.Unix(1000, 0)
	if applicationStatus(2000, 3000, 0, n) != "not_started" {
		t.Fatal("not_started")
	}
	if applicationStatus(500, 1500, 0, n) != "open" {
		t.Fatal("open")
	}
	if applicationStatus(1, 2, 0, n) != "closed" {
		t.Fatal("closed")
	}
}
