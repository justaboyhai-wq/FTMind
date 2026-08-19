package normalize

import (
	"fmt"
	"strings"
	"time"

	htmltomd "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/detail"
	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
)

type Relation struct {
	SourceID         int64  `json:"source_id"`
	RelationType     string `json:"relation_type"`
	TargetID         int64  `json:"target_id,omitempty"`
	TargetURL        string `json:"target_url"`
	SourceCode       string `json:"source_code"`
	SourceLabel      string `json:"source_label"`
	TargetTitle      string `json:"target_title"`
	AssignmentSource string `json:"assignment_source"`
	RuleVersion      string `json:"rule_version"`
}

type Normalized struct {
	Structured model.StructuredPolicy
	Markdown   string
	Relations  []Relation
}

func Policy(d detail.Decoded, now time.Time) (Normalized, error) {
	md, err := htmltomd.ConvertString(d.ContentHTML)
	if err != nil {
		return Normalized{}, fmt.Errorf("convert policy HTML: %w", err)
	}
	md = strings.TrimSpace(md)
	if md == "" {
		return Normalized{}, fmt.Errorf("policy %d has empty content", d.ID)
	}
	start := time.Unix(d.Extension.ApplicationStart, 0)
	end := time.Unix(d.Extension.ApplicationEnd, 0)
	status := applicationStatus(d.Extension.ApplicationStart, d.Extension.ApplicationEnd, d.GKML.ExpiredAt, now)
	structured := model.StructuredPolicy{ID: d.ID, Title: d.Title, DocumentNumber: first(d.Extension.DocumentNumber, d.GKML.DocumentNumber), SourceURL: d.URL, FinalURL: d.URL, Abstract: d.Abstract, Markdown: md, PublishedAt: d.PublishedAt, EffectiveAt: formatTime(start), ExpiresAt: formatTime(time.Unix(d.GKML.ExpiredAt, 0)), Official: model.OfficialFacts{ServiceObjects: d.Extension.ServiceObjects, IssuingAuthority: first(d.Extension.IssuingAuthority, d.GKML.Publisher), Theme: first(d.Extension.Theme, d.GKML.ClassifyThemeName), CarrierType: first(d.Extension.CarrierType, d.GKML.ClassifyMainName), DocumentGenre: d.GKML.ClassifyGenreName}, ApplicationStart: formatTime(start), ApplicationEnd: formatTime(end), OfficialListed: true, LocalApplicationStatus: status, Conflicts: d.Conflicts}
	return Normalized{Structured: structured, Markdown: md, Relations: relations(d)}, nil
}

func relations(d detail.Decoded) []Relation {
	out := make([]Relation, 0, len(d.RelatedPosts))
	for _, p := range d.RelatedPosts {
		r := Relation{SourceID: d.ID, TargetID: p.ID, TargetURL: p.URL, TargetTitle: p.Title, SourceCode: fmt.Sprintf("related_classify:%d", p.RelatedClassify), SourceLabel: p.TypeLabel, AssignmentSource: "website", RuleVersion: "baoan-related-v1"}
		switch p.RelatedClassify {
		case 2:
			r.RelationType = "related_document"
		case 3:
			r.RelationType = "text_interpretation"
		case 4:
			r.RelationType = "graphic_interpretation"
		case 5:
			r.RelationType = "video_interpretation"
		case 7:
			r.RelationType = "website_related"
			if strings.Contains(p.Title, "意见") || strings.Contains(p.Title, "征集") || strings.Contains(p.Title, "征求") {
				r.SourceLabel = "意见征集"
			} else if strings.Contains(p.Title, "申报") || strings.Contains(p.Title, "申请") {
				r.SourceLabel = "申报公告"
			} else if strings.Contains(p.Title, "操作规程") {
				r.SourceLabel = "操作规程"
			}
		default:
			r.RelationType = "unknown"
		}
		out = append(out, r)
	}
	return out
}

func applicationStatus(start, end, expiry int64, now time.Time) string {
	if start > 0 && end > 0 {
		n := now.Unix()
		if n < start {
			return "not_started"
		}
		if n <= end {
			return "open"
		}
		return "closed"
	}
	if expiry == 0 {
		return "official_open_date_unknown"
	}
	if now.Unix() <= expiry {
		return "rolling"
	}
	return "closed"
}
func formatTime(t time.Time) string {
	if t.IsZero() || t.Unix() <= 0 {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
func first(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
