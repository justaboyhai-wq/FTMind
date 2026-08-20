package rssserver

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/canonical"
)

type Config struct {
	DataDir  string
	BaseURL  string
	FeedPath string
}

type Server struct{ cfg Config }

type feed struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel channel  `xml:"channel"`
}
type channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []item `xml:"item"`
}
type item struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	GUID        string   `xml:"guid"`
	Categories  []string `xml:"category,omitempty"`
	Description string   `xml:"description"`
	PubDate     string   `xml:"pubDate,omitempty"`
}

func New(cfg Config) *Server {
	if cfg.FeedPath == "" {
		cfg.FeedPath = "/feed.xml"
	}
	return &Server{cfg: cfg}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc(s.cfg.FeedPath, s.feed)
	mux.HandleFunc("/tag-audit.json", s.tagAudit)
	mux.HandleFunc("/packages/", s.packagePage)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	documents, err := s.loadDocuments()
	response := map[string]any{"status": "ok", "raw_schema_version": "baoan.raw/v1", "document_schema_version": canonical.SchemaVersion, "policy_count": len(documents)}
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		response["status"] = "degraded"
		response["error"] = err.Error()
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) feed(w http.ResponseWriter, r *http.Request) {
	documents, err := s.loadDocuments()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	base := strings.TrimRight(s.cfg.BaseURL, "/")
	if base == "" {
		base = scheme(r) + "://" + r.Host
	}
	entries := make([]item, 0, len(documents))
	dimensions, _ := canonical.LoadLatestDictionary(s.cfg.DataDir)
	for _, doc := range documents {
		updated := doc.FetchedAt
		if updated.IsZero() {
			updated = time.Unix(0, 0).UTC()
		}
		tags := doc.OfficialTags(time.Now().UTC())
		if len(dimensions) > 0 {
			tags, _ = canonical.FilterOfficialTags(tags, dimensions)
		}
		entries = append(entries, item{
			Title:       doc.Structured.Title,
			Link:        base + "/packages/" + url.PathEscape(doc.PackageID),
			GUID:        "baoan-policy:" + doc.PackageID,
			Categories:  tags,
			Description: firstNonEmpty(doc.Structured.Abstract, "宝安区政策完整规范化文档"),
			PubDate:     updated.UTC().Format(time.RFC1123Z),
		})
	}
	out, err := xml.Marshal(feed{Version: "2.0", Channel: channel{
		Title: "宝安政策原文（FTMind Canonical）", Link: base + s.cfg.FeedPath,
		Description: "由 baoan.raw/v1 统一组装为 baoan.canonical-md/v1 的政策文档。", Items: entries,
	}})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(append([]byte(xml.Header), out...))
}

func (s *Server) tagAudit(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	documents, err := s.loadDocuments()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dimensions, err := canonical.LoadLatestDictionary(s.cfg.DataDir)
	if err != nil {
		http.Error(w, fmt.Sprintf("load official tag dictionary: %v", err), http.StatusInternalServerError)
		return
	}
	type rejectedTagAudit struct {
		PolicyID     string   `json:"policy_id"`
		Title        string   `json:"title"`
		RejectedTags []string `json:"rejected_tags"`
	}
	rejected := make([]rejectedTagAudit, 0)
	for _, doc := range documents {
		_, tags := canonical.FilterOfficialTags(doc.OfficialTags(time.Now().UTC()), dimensions)
		if len(tags) > 0 {
			rejected = append(rejected, rejectedTagAudit{PolicyID: doc.PackageID, Title: doc.Structured.Title, RejectedTags: tags})
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"rejected_tag_count": len(rejected), "items": rejected})
}

func (s *Server) packagePage(w http.ResponseWriter, r *http.Request) {
	id, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/packages/"))
	if err != nil || id == "" || strings.ContainsAny(id, `/\\`) || id == "." || id == ".." {
		http.NotFound(w, r)
		return
	}
	doc, err := canonical.LoadLatest(s.cfg.DataDir, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("ETag", `"`+doc.SnapshotSHA256+`"`)
	_, _ = w.Write([]byte(doc.HTML()))
}

func (s *Server) loadDocuments() ([]canonical.Document, error) {
	entries, err := os.ReadDir(filepath.Join(s.cfg.DataDir, "policies"))
	if err != nil {
		return nil, err
	}
	documents := make([]canonical.Document, 0, len(entries))
	var failures []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		doc, err := canonical.LoadLatest(s.cfg.DataDir, entry.Name())
		if err != nil {
			failures = append(failures, entry.Name()+": "+err.Error())
			continue
		}
		documents = append(documents, doc)
	}
	sort.Slice(documents, func(i, j int) bool {
		if documents[i].FetchedAt.Equal(documents[j].FetchedAt) {
			return documents[i].PackageID < documents[j].PackageID
		}
		return documents[i].FetchedAt.After(documents[j].FetchedAt)
	})
	if len(failures) > 0 {
		return nil, fmt.Errorf("canonical assembly failed: %s", strings.Join(failures, "; "))
	}
	return documents, nil
}

func scheme(r *http.Request) string {
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") || r.TLS != nil {
		return "https"
	}
	return "http"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
