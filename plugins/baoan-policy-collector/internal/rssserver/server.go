package rssserver

import (
	"encoding/json"
	"encoding/xml"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Config controls the read-only RSS gateway over a baoan.raw/v1 data directory.
type Config struct {
	DataDir  string
	BaseURL  string
	FeedPath string
}

type Server struct{ cfg Config }

type policy struct {
	ID, Title, SourceURL, Markdown, Abstract string
	Snapshot                                 string
	Updated                                  time.Time
}

type structured struct {
	Title, SourceURL, FinalURL, Abstract, PublishedAt, EffectiveAt, ExpiresAt string
}

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
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate,omitempty"`
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
	mux.HandleFunc("/packages/", s.packagePage)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok","schema_version":"baoan.raw/v1"}`))
}

func (s *Server) feed(w http.ResponseWriter, r *http.Request) {
	items, err := s.loadPolicies()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	base := strings.TrimRight(s.cfg.BaseURL, "/")
	if base == "" {
		base = scheme(r) + "://" + r.Host
	}
	entries := make([]item, 0, len(items))
	for _, p := range items {
		entries = append(entries, item{
			Title:       p.Title,
			Link:        base + "/packages/" + url.PathEscape(p.ID),
			GUID:        "baoan-policy:" + p.ID + ":" + p.Snapshot,
			Description: firstNonEmpty(p.Abstract, "宝安区政策原文；详情页包含标准化原文、来源 URL 和结构化字段。"),
			PubDate:     p.Updated.UTC().Format(time.RFC1123Z),
		})
	}
	channelLink := base + s.cfg.FeedPath
	out, err := xml.Marshal(feed{Version: "2.0", Channel: channel{Title: "宝安政策原文（FMind Raw）", Link: channelLink, Description: "来源于宝安区政策法规库的 baoan.raw/v1 政策包。", Items: entries}})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(append([]byte(xml.Header), out...))
}

func (s *Server) packagePage(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/packages/")
	id, err := url.PathUnescape(id)
	if err != nil || id == "" || strings.ContainsAny(id, `/\\`) || id == "." || id == ".." {
		http.NotFound(w, r)
		return
	}
	base := filepath.Join(s.cfg.DataDir, "policies", id)
	latestBody, err := os.ReadFile(filepath.Join(base, "latest.json"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var latest struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if json.Unmarshal(latestBody, &latest) != nil || latest.SnapshotID == "" || strings.ContainsAny(latest.SnapshotID, `/\\`) {
		http.NotFound(w, r)
		return
	}
	snapshot := filepath.Join(base, "snapshots", latest.SnapshotID)
	structuredBody, _ := os.ReadFile(filepath.Join(snapshot, "structured.json"))
	var p structured
	_ = json.Unmarshal(structuredBody, &p)
	markdown, err := os.ReadFile(filepath.Join(snapshot, "normalized.md"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	title := firstNonEmpty(p.Title, id)
	// Keep the original markdown intact inside a readable HTML document. The
	// FMind RSS connector extracts the article page and converts it to Markdown.
	body := "<!doctype html><html><head><meta charset=\"utf-8\"><title>" + html.EscapeString(title) + "</title></head><body><article><h1>" + html.EscapeString(title) + "</h1><p>来源：" + html.EscapeString(firstNonEmpty(p.SourceURL, p.FinalURL)) + "</p><pre>" + html.EscapeString(string(markdown)) + "</pre></article></body></html>"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(body))
}

func (s *Server) loadPolicies() ([]policy, error) {
	root := filepath.Join(s.cfg.DataDir, "policies")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	out := make([]policy, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		base := filepath.Join(root, entry.Name())
		latestBody, err := os.ReadFile(filepath.Join(base, "latest.json"))
		if err != nil {
			continue
		}
		var latest struct {
			SnapshotID string `json:"snapshot_id"`
		}
		if json.Unmarshal(latestBody, &latest) != nil || latest.SnapshotID == "" || strings.ContainsAny(latest.SnapshotID, `/\\`) {
			continue
		}
		snapshot := filepath.Join(base, "snapshots", latest.SnapshotID)
		body, err := os.ReadFile(filepath.Join(snapshot, "structured.json"))
		if err != nil {
			continue
		}
		var p structured
		if json.Unmarshal(body, &p) != nil {
			continue
		}
		markdown, err := os.ReadFile(filepath.Join(snapshot, "normalized.md"))
		if err != nil {
			continue
		}
		updated := time.Time{}
		if manifest, err := os.ReadFile(filepath.Join(snapshot, "manifest.json")); err == nil {
			var m struct {
				FetchedAt time.Time `json:"fetched_at"`
			}
			_ = json.Unmarshal(manifest, &m)
			updated = m.FetchedAt
		}
		if updated.IsZero() {
			updated = time.Now().UTC()
		}
		out = append(out, policy{ID: entry.Name(), Title: firstNonEmpty(p.Title, entry.Name()), SourceURL: p.SourceURL, Markdown: string(markdown), Abstract: p.Abstract, Snapshot: latest.SnapshotID, Updated: updated})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out, nil
}

func scheme(r *http.Request) string {
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") || r.TLS != nil {
		return "https"
	}
	return "http"
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
