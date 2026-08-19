package detail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
)

type Extension struct {
	DocumentNumber                         string   `json:"document_number"`
	IssuingAuthority                       string   `json:"issuing_authority"`
	Theme                                  string   `json:"theme"`
	CarrierType                            string   `json:"carrier_type"`
	ServiceObjects                         []string `json:"service_objects"`
	ConsultationAddress, ConsultationPhone string
	ApplicationStart, ApplicationEnd       int64
}

type GKML struct {
	DocumentNumber                                         string `json:"document_number"`
	Publisher                                              string `json:"publisher"`
	ClassifyMainName, ClassifyGenreName, ClassifyThemeName string
	ExpiredAt                                              int64 `json:"expired_time"`
	IsExpired                                              int   `json:"is_expired"`
}

type Decoded struct {
	Raw                                                                   []byte
	ID                                                                    int64
	Title, Abstract, ContentHTML, Source, Date, URL                       string
	OriginURL, GKMLURL                                                    string
	PublishedAt, FirstPublishedAt, DisplayPublishedAt, CreatedAt, Updated string
	ExpiredAt                                                             int64
	Extension                                                             Extension
	GKML                                                                  GKML
	Attachments                                                           []model.Attachment
	RelatedPosts                                                          []model.RelatedPost
	Conflicts                                                             []model.FieldConflict
}

type rawDetail struct {
	ID                                                            int64 `json:"id"`
	Title, Abstract, Content, Source, Date, URL                   string
	OriginURL                                                     string `json:"origin_url"`
	GKMLURL                                                       string `json:"gkml_url"`
	FirstPublishTime, PublishTime, DisplayPublishTime, CreateTime int64
	Updated                                                       string
	Attachment                                                    []model.Attachment  `json:"attachment"`
	RelatedPosts                                                  []model.RelatedPost `json:"related_posts"`
	JSONExt                                                       string              `json:"json_ext"`
	GKMLData                                                      string              `json:"gkml_data"`
	ExpiredAt                                                     int64               `json:"expired_time"`
	EXTWH                                                         string              `json:"EXT_wh"`
	EXTFBJG                                                       string              `json:"EXT_fbjg"`
	EXTZTFL                                                       string              `json:"EXT_ztfl"`
	EXTWJLB                                                       string              `json:"EXT_wjlb"`
	WJLX                                                          []string            `json:"wjlx"`
	SBKS                                                          int64               `json:"sbks"`
	SBJS                                                          int64               `json:"sbjs"`
}

func URLForID(base, id string) (string, error) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return "", fmt.Errorf("invalid policy id %q", id)
	}
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid base URL")
	}
	u.Path = fmt.Sprintf("/postmeta/p/%d/%d/%d.json", n/1000000, n/1000, n)
	u.RawQuery, u.Fragment = "", ""
	return u.String(), nil
}

func Decode(body []byte) (Decoded, error) {
	var raw rawDetail
	if err := json.Unmarshal(body, &raw); err != nil {
		return Decoded{}, fmt.Errorf("decode detail: %w", err)
	}
	var ext map[string]json.RawMessage
	if raw.JSONExt != "" {
		if err := json.Unmarshal([]byte(raw.JSONExt), &ext); err != nil {
			return Decoded{}, fmt.Errorf("decode json_ext: %w", err)
		}
	}
	var g map[string]json.RawMessage
	if raw.GKMLData != "" {
		if err := json.Unmarshal([]byte(raw.GKMLData), &g); err != nil {
			return Decoded{}, fmt.Errorf("decode gkml_data: %w", err)
		}
	}
	d := Decoded{Raw: append([]byte(nil), body...), ID: raw.ID, Title: raw.Title, Abstract: raw.Abstract, ContentHTML: raw.Content, Source: raw.Source, Date: raw.Date, URL: raw.URL, OriginURL: raw.OriginURL, GKMLURL: raw.GKMLURL, Attachments: raw.Attachment, RelatedPosts: raw.RelatedPosts, ExpiredAt: raw.ExpiredAt, Updated: raw.Updated}
	d.PublishedAt = unixString(raw.PublishTime)
	d.FirstPublishedAt = unixString(raw.FirstPublishTime)
	d.DisplayPublishedAt = unixString(raw.DisplayPublishTime)
	d.CreatedAt = unixString(raw.CreateTime)
	d.Extension = Extension{DocumentNumber: pickStringFallback(ext, "EXT_wh", raw.EXTWH), IssuingAuthority: pickStringFallback(ext, "EXT_fbjg", raw.EXTFBJG), Theme: pickStringFallback(ext, "EXT_ztfl", raw.EXTZTFL), CarrierType: pickStringFallback(ext, "EXT_wjlb", raw.EXTWJLB), ServiceObjects: pickStringsFallback(ext, "wjlx", raw.WJLX), ConsultationAddress: pickString(ext, "zxdz"), ConsultationPhone: pickString(ext, "zxdh"), ApplicationStart: pickIntFallback(ext, "sbks", raw.SBKS), ApplicationEnd: pickIntFallback(ext, "sbjs", raw.SBJS)}
	d.GKML = GKML{DocumentNumber: pickString(g, "document_number"), Publisher: pickString(g, "publisher"), ClassifyMainName: pickString(g, "classify_main_name"), ClassifyGenreName: pickString(g, "classify_genre_name"), ClassifyThemeName: pickString(g, "classify_theme_name"), ExpiredAt: pickInt(g, "expired_time"), IsExpired: int(pickInt(g, "is_expired"))}
	d.Conflicts = conflicts(ext, raw)
	return d, nil
}

func unixString(seconds int64) string {
	if seconds <= 0 {
		return ""
	}
	return time.Unix(seconds, 0).UTC().Format(time.RFC3339)
}
func pickString(m map[string]json.RawMessage, key string) string {
	var s string
	if err := json.Unmarshal(m[key], &s); err == nil {
		return s
	}
	return ""
}
func pickStringFallback(m map[string]json.RawMessage, key, fallback string) string {
	if s := pickString(m, key); s != "" {
		return s
	}
	return fallback
}
func pickInt(m map[string]json.RawMessage, key string) int64 {
	var n int64
	if err := json.Unmarshal(m[key], &n); err == nil {
		return n
	}
	var s string
	if err := json.Unmarshal(m[key], &s); err == nil {
		n, _ = strconv.ParseInt(s, 10, 64)
	}
	return n
}
func pickIntFallback(m map[string]json.RawMessage, key string, fallback int64) int64 {
	if n := pickInt(m, key); n != 0 {
		return n
	}
	return fallback
}
func pickStrings(m map[string]json.RawMessage, key string) []string {
	var s []string
	_ = json.Unmarshal(m[key], &s)
	return s
}
func pickStringsFallback(m map[string]json.RawMessage, key string, fallback []string) []string {
	if s := pickStrings(m, key); len(s) > 0 {
		return s
	}
	return fallback
}
func conflicts(ext map[string]json.RawMessage, raw rawDetail) []model.FieldConflict {
	fields := []string{"EXT_wh", "EXT_fbjg", "EXT_ztfl", "EXT_wjlb", "wjlx", "sbks", "sbjs"}
	out := []model.FieldConflict{}
	for _, k := range fields {
		nested := ext[k]
		if len(nested) == 0 {
			continue
		}
		top := topField(raw, k)
		if len(top) > 0 && !bytes.Equal(bytes.TrimSpace(top), bytes.TrimSpace(nested)) {
			out = append(out, model.FieldConflict{Field: k, TopLevel: string(top), Nested: string(nested)})
		}
	}
	return out
}
func topField(r rawDetail, key string) []byte {
	var v any
	switch key {
	case "EXT_wh":
		v = r.EXTWH
	case "EXT_fbjg":
		v = r.EXTFBJG
	case "EXT_ztfl":
		v = r.EXTZTFL
	case "EXT_wjlb":
		v = r.EXTWJLB
	case "wjlx":
		v = r.WJLX
	case "sbks":
		v = r.SBKS
	case "sbjs":
		v = r.SBJS
	default:
		return nil
	}
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	if string(b) == `""` || string(b) == "null" || string(b) == "0" || string(b) == "[]" {
		return nil
	}
	return b
}
