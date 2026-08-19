package detail

import (
	"os"
	"testing"
)

func TestURLForID(t *testing.T) {
	got, err := URLForID("https://www.baoan.gov.cn", "12846556")
	want := "https://www.baoan.gov.cn/postmeta/p/12/12846/12846556.json"
	if err != nil || got != want {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestDecodeLayeredDetail(t *testing.T) {
	b, err := os.ReadFile("../../testdata/detail-12846556.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != 12846556 || len(d.Attachments) != 4 || len(d.RelatedPosts) != 4 {
		t.Fatalf("detail=%+v", d)
	}
	if d.GKML.DocumentNumber != "深宝福规〔2026〕1号" || d.Extension.Theme != "公安、安全、司法" {
		t.Fatalf("gkml=%+v extension=%+v", d.GKML, d.Extension)
	}
	if d.RelatedPosts[1].RelatedClassify != 3 {
		t.Fatalf("related=%+v", d.RelatedPosts[1])
	}
}

func TestURLForIDRejectsInvalid(t *testing.T) {
	if _, err := URLForID("https://www.baoan.gov.cn", "abc"); err == nil {
		t.Fatal("expected invalid id")
	}
	if _, err := URLForID("file:///tmp", "1"); err == nil {
		t.Fatal("expected invalid base")
	}
}
