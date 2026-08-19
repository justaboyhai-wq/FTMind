package archive

import (
	"encoding/json"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishIsImmutableAndIdempotent(t *testing.T) {
	root := t.TempDir()
	pkg := Package{ExternalID: "post_12846556", DetailRaw: []byte(`{"id":12846556}`), SourceHTML: []byte(`<p>正文</p>`), Markdown: "# 正文\n", Structured: []byte(`{"id":12846556,"title":"政策","official":{}}`), Relations: []byte(`[]`), Attachments: []model.DownloadedAttachment{{Attachment: model.Attachment{Name: "附件.pdf", URL: "https://www.baoan.gov.cn/attachment/a.pdf"}, ActualSize: 3, SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", Body: []byte("pdf")}}}
	a, err := Publish(root, pkg)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Publish(root, pkg)
	if err != nil {
		t.Fatal(err)
	}
	if a.SnapshotID != b.SnapshotID || b.Created {
		t.Fatalf("a=%+v b=%+v", a, b)
	}
}

func TestPublishCopiesCanonicalURLsIntoManifest(t *testing.T) {
	root := t.TempDir()
	pkg := Package{ExternalID: "post_1", DetailRaw: []byte(`{"id":1}`), SourceHTML: []byte(`<p>正文</p>`), Markdown: "正文", Structured: []byte(`{"id":1,"title":"政策","source_url":"https://www.baoan.gov.cn/source.html","final_url":"https://www.baoan.gov.cn/final.html","markdown":"正文","official":{}}`), Relations: []byte(`[]`)}
	result, err := Publish(root, pkg)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(root, "policies", "post_1", "snapshots", result.SnapshotID, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest model.Manifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.CanonicalURL != "https://www.baoan.gov.cn/source.html" || manifest.FinalURL != "https://www.baoan.gov.cn/final.html" {
		t.Fatalf("manifest URLs not populated: %+v", manifest)
	}
}

func TestWriteCheckedRejectsTraversal(t *testing.T) {
	if err := writeChecked(t.TempDir(), "../evil", []byte("x")); err == nil {
		t.Fatal("expected traversal rejection")
	}
}
