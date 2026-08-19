package archive

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/justaboyhai-wq/fmind/plugins/baoan-policy-collector/internal/model"
)

var safeName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type Package = model.Package
type PublishResult struct {
	SnapshotID string
	Created    bool
}

func Publish(root string, pkg Package) (PublishResult, error) {
	if pkg.ExternalID == "" {
		return PublishResult{}, errors.New("external id is required")
	}
	name := safeName.ReplaceAllString(pkg.ExternalID, "_")
	base := filepath.Join(root, "policies", name)
	if err := os.MkdirAll(filepath.Join(base, "snapshots"), 0o755); err != nil {
		return PublishResult{}, err
	}
	snapshotHash := packageHash(pkg)
	existing, err := findSnapshot(base, snapshotHash)
	if err != nil {
		return PublishResult{}, err
	}
	if existing != "" {
		return PublishResult{SnapshotID: existing}, nil
	}
	snapshotID := time.Now().UTC().Format("20060102T150405Z") + "-" + snapshotHash[:12]
	staging := filepath.Join(root, ".staging", name, snapshotID)
	if err := os.MkdirAll(filepath.Join(staging, "attachments"), 0o755); err != nil {
		return PublishResult{}, err
	}
	files := map[string][]byte{"source-detail.json": pkg.DetailRaw, "source.html": pkg.SourceHTML, "normalized.md": []byte(pkg.Markdown), "structured.json": pkg.Structured, "relations.json": pkg.Relations}
	for path, body := range files {
		if err := writeChecked(staging, path, body); err != nil {
			return PublishResult{}, err
		}
	}
	for i, a := range pkg.Attachments {
		if int64(len(a.Body)) != a.ActualSize {
			return PublishResult{}, fmt.Errorf("attachment %s size mismatch", a.Name)
		}
		if len(a.SHA256) < 12 {
			return PublishResult{}, fmt.Errorf("attachment %s has invalid sha256", a.Name)
		}
		name := fmt.Sprintf("%s-%s", a.SHA256[:12], safeName.ReplaceAllString(filepath.Base(a.Name), "_"))
		path := filepath.Join("attachments", name)
		if err := writeChecked(staging, path, a.Body); err != nil {
			return PublishResult{}, err
		}
		pkg.Attachments[i].StoragePath = path
		files[path] = a.Body
	}
	manifest := model.Manifest{SchemaVersion: "baoan.raw/v1", PackageID: name, ExternalID: pkg.ExternalID, SnapshotID: snapshotID, FetchedAt: time.Now().UTC(), SnapshotSHA256: snapshotHash}
	for path := range files {
		manifest.Files = append(manifest.Files, path)
	}
	sortStrings(manifest.Files)
	manifestBody, _ := json.MarshalIndent(manifest, "", "  ")
	if err := writeChecked(staging, "manifest.json", manifestBody); err != nil {
		return PublishResult{}, err
	}
	files["manifest.json"] = manifestBody
	if err := writeChecked(staging, "checksums.sha256", []byte(checksumFile(files))); err != nil {
		return PublishResult{}, err
	}
	final := filepath.Join(base, "snapshots", snapshotID)
	if err := os.Rename(staging, final); err != nil {
		return PublishResult{}, err
	}
	latest, _ := json.Marshal(map[string]string{"snapshot_id": snapshotID, "snapshot_sha256": snapshotHash})
	if err := atomicWrite(filepath.Join(base, "latest.json"), latest); err != nil {
		return PublishResult{}, err
	}
	return PublishResult{SnapshotID: snapshotID, Created: true}, nil
}

func packageHash(pkg Package) string {
	h := sha256.New()
	h.Write(pkg.DetailRaw)
	h.Write(pkg.SourceHTML)
	h.Write([]byte(pkg.Markdown))
	h.Write(pkg.Structured)
	h.Write(pkg.Relations)
	for _, a := range pkg.Attachments {
		h.Write([]byte(a.URL))
		h.Write([]byte(a.SHA256))
		h.Write(a.Body)
	}
	return hex.EncodeToString(h.Sum(nil))
}
func findSnapshot(base, hash string) (string, error) {
	entries, err := os.ReadDir(filepath.Join(base, "snapshots"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(base, "snapshots", e.Name(), "manifest.json"))
		if err != nil {
			continue
		}
		var m model.Manifest
		if json.Unmarshal(b, &m) == nil && m.SnapshotSHA256 == hash {
			return e.Name(), nil
		}
	}
	return "", nil
}
func writeChecked(root, rel string, body []byte) error {
	if filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		return errors.New("unsafe archive path")
	}
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}
func atomicWrite(path string, body []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func checksumFile(files map[string][]byte) string {
	var names []string
	for n := range files {
		names = append(names, n)
	}
	sortStrings(names)
	var b strings.Builder
	for _, n := range names {
		s := sha256.Sum256(files[n])
		fmt.Fprintf(&b, "%s  %s\n", hex.EncodeToString(s[:]), n)
	}
	return b.String()
}
func sortStrings(a []string) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j] < a[j-1]; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}
