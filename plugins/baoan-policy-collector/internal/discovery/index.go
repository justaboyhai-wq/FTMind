package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
)

var scriptPattern = regexp.MustCompile(`(?is)<script\b[^>]*?\bsrc\s*=\s*["']([^"']+)["'][^>]*>`)

func DiscoverIndexScript(seedURL string, body []byte) (string, error) {
	base, err := url.Parse(seedURL)
	if err != nil {
		return "", fmt.Errorf("parse seed URL: %w", err)
	}
	var found string
	for _, match := range scriptPattern.FindAllSubmatch(body, -1) {
		src := html.UnescapeString(string(match[1]))
		parsed, err := url.Parse(src)
		if err != nil {
			continue
		}
		if strings.EqualFold(parsed.Path, "/zcfg.js") || strings.HasSuffix(strings.ToLower(parsed.Path), "/zcfg.js") {
			if found != "" && found != base.ResolveReference(parsed).String() {
				return "", fmt.Errorf("multiple zcfg.js scripts")
			}
			found = base.ResolveReference(parsed).String()
		}
	}
	if found == "" {
		return "", fmt.Errorf("zcfg.js script not found")
	}
	return found, nil
}

func RecordHash(record model.IndexRecord) (string, error) {
	b, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

func Diff(previous, current []model.IndexRecord) (model.IndexDiff, error) {
	oldHashes := make(map[string]string, len(previous))
	newHashes := make(map[string]string, len(current))
	for _, r := range previous {
		h, err := RecordHash(r)
		if err != nil {
			return model.IndexDiff{}, err
		}
		oldHashes[r.ID] = h
	}
	for _, r := range current {
		h, err := RecordHash(r)
		if err != nil {
			return model.IndexDiff{}, err
		}
		newHashes[r.ID] = h
	}
	var d model.IndexDiff
	for id, h := range newHashes {
		old, ok := oldHashes[id]
		if !ok {
			d.Added = append(d.Added, id)
		} else if old != h {
			d.Changed = append(d.Changed, id)
		}
	}
	for id := range oldHashes {
		if _, ok := newHashes[id]; !ok {
			d.Missing = append(d.Missing, id)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Changed)
	sort.Strings(d.Missing)
	return d, nil
}
