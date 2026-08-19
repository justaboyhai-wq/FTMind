package discovery

import (
	"os"
	"testing"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
)

func TestDiscoverIndexScript(t *testing.T) {
	b, err := os.ReadFile("../../testdata/seed.html")
	if err != nil {
		t.Fatal(err)
	}
	u, err := DiscoverIndexScript("https://www.baoan.gov.cn/xxgk/fgk/", b)
	if err != nil || u != "https://www.baoan.gov.cn/zcfg.js" {
		t.Fatalf("url=%q err=%v", u, err)
	}
}

func TestParseAllDataWithoutExecution(t *testing.T) {
	b, err := os.ReadFile("../../testdata/zcfg.js")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ParseAllData(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 3 || r[1].Title != "含'单引号" || r[0].ID != "12846556" {
		t.Fatalf("records=%+v", r)
	}
}

func TestParserRejectsExecutableToken(t *testing.T) {
	if _, err := ParseAllData([]byte(`var allData=[{ "id":"1", "title": evil() }]`)); err == nil {
		t.Fatal("expected rejection")
	}
}
func TestDiffIgnoresOrdering(t *testing.T) {
	a := []model.IndexRecord{{ID: "1", Title: "one"}, {ID: "2", Title: "two"}}
	b := []model.IndexRecord{{ID: "2", Title: "two"}, {ID: "1", Title: "one"}}
	d, err := Diff(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Added) != 0 || len(d.Changed) != 0 || len(d.Missing) != 0 {
		t.Fatalf("diff=%+v", d)
	}
}
