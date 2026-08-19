package model

import (
	"bytes"
	"encoding/json"
	"strconv"
	"time"
)

type IndexRecord struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Subtitle         string   `json:"subtitle,omitempty"`
	Content          string   `json:"content,omitempty"`
	Abstract         string   `json:"abstract,omitempty"`
	Source           string   `json:"source,omitempty"`
	Date             string   `json:"date,omitempty"`
	URL              string   `json:"url,omitempty"`
	DocumentNumber   string   `json:"wh,omitempty"`
	Theme            string   `json:"zt,omitempty"`
	CarrierType      string   `json:"tc,omitempty"`
	ServiceObjects   []string `json:"wjlx,omitempty"`
	ApplicationStart int64    `json:"sbks,omitempty"`
	ApplicationEnd   int64    `json:"sbjs,omitempty"`
	ExpiredAt        int64    `json:"expired_time,omitempty"`
}

// UnmarshalJSON accepts the site's mixed number/string timestamp representation.
func (r *IndexRecord) UnmarshalJSON(data []byte) error {
	type plain IndexRecord
	var aux struct {
		*plain
		ApplicationStart json.RawMessage `json:"sbks"`
		ApplicationEnd   json.RawMessage `json:"sbjs"`
		ExpiredAt        json.RawMessage `json:"expired_time"`
		ServiceObjects   json.RawMessage `json:"wjlx"`
	}
	aux.plain = (*plain)(r)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	var err error
	if r.ApplicationStart, err = parseInt(aux.ApplicationStart); err != nil {
		return err
	}
	if r.ApplicationEnd, err = parseInt(aux.ApplicationEnd); err != nil {
		return err
	}
	if r.ExpiredAt, err = parseInt(aux.ExpiredAt); err != nil {
		return err
	}
	if len(bytes.TrimSpace(aux.ServiceObjects)) > 0 && !bytes.Equal(bytes.TrimSpace(aux.ServiceObjects), []byte("null")) {
		if err := json.Unmarshal(aux.ServiceObjects, &r.ServiceObjects); err != nil {
			var one string
			if err := json.Unmarshal(aux.ServiceObjects, &one); err != nil {
				return err
			}
			r.ServiceObjects = []string{one}
		}
	}
	return nil
}

func parseInt(raw json.RawMessage) (int64, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, nil
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, err
	}
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

type Attachment struct {
	ID                    int64 `json:"id"`
	Name, Type, MIME, URL string
	Size                  int64 `json:"size,string,omitempty"`
}

func (a *Attachment) UnmarshalJSON(data []byte) error {
	type plain Attachment
	var aux struct {
		*plain
		Size json.RawMessage `json:"size"`
	}
	aux.plain = (*plain)(a)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	n, err := parseInt(aux.Size)
	a.Size = n
	return err
}

type DownloadedAttachment struct {
	Attachment
	ActualSize  int64
	SHA256      string
	StoragePath string
	Body        []byte `json:"-"`
}
type RelatedPost struct {
	ID                                                            int64 `json:"id"`
	Title, Type, TypeLabel, URL, Status                           string
	Rank                                                          int    `json:"rank"`
	OldRank                                                       int    `json:"oldrank"`
	RelatedClassify                                               int    `json:"related_classify"`
	CreatedAt                                                     string `json:"created_at"`
	PublishedAt, FirstPublishedAt, DisplayPublishedAt, CreateTime int64
}
type FieldConflict struct {
	Field            string
	TopLevel, Nested string
}
type IndexDiff struct {
	Added, Changed, Missing []string `json:"-"`
}

type OfficialFacts struct {
	ServiceObjects   []string `json:"service_objects"`
	IssuingAuthority string   `json:"issuing_authority"`
	Theme            string   `json:"theme"`
	CarrierType      string   `json:"carrier_type"`
	DocumentGenre    string   `json:"document_genre"`
}

type StructuredPolicy struct {
	ID                     int64           `json:"id"`
	Title                  string          `json:"title"`
	DocumentNumber         string          `json:"document_number"`
	SourceURL              string          `json:"source_url"`
	FinalURL               string          `json:"final_url"`
	Abstract               string          `json:"abstract"`
	Markdown               string          `json:"markdown"`
	Official               OfficialFacts   `json:"official"`
	PublishedAt            string          `json:"published_at"`
	EffectiveAt            string          `json:"effective_at"`
	ExpiresAt              string          `json:"expires_at"`
	ApplicationStart       string          `json:"application_start"`
	ApplicationEnd         string          `json:"application_end"`
	OfficialListed         bool            `json:"official_listed"`
	LocalApplicationStatus string          `json:"local_application_status"`
	Conflicts              []FieldConflict `json:"conflicts,omitempty"`
}

type Package struct {
	ExternalID            string
	DetailRaw, SourceHTML []byte
	Markdown              string
	Structured, Relations []byte
	Attachments           []DownloadedAttachment
}

type Manifest struct {
	SchemaVersion  string               `json:"schema_version"`
	PackageID      string               `json:"package_id"`
	ExternalID     string               `json:"external_id"`
	SnapshotID     string               `json:"snapshot_id"`
	CanonicalURL   string               `json:"canonical_url"`
	FinalURL       string               `json:"final_url"`
	FetchedAt      time.Time            `json:"fetched_at"`
	SnapshotSHA256 string               `json:"snapshot_sha256"`
	Files          []string             `json:"files"`
	Attachments    []AttachmentManifest `json:"attachments,omitempty"`
}

type AttachmentManifest struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	MIME         string `json:"mime,omitempty"`
	DeclaredSize int64  `json:"declared_size,omitempty"`
	ActualSize   int64  `json:"actual_size"`
	SHA256       string `json:"sha256"`
	StoragePath  string `json:"storage_path"`
}

type Failure struct {
	RunID       string    `json:"run_id"`
	ExternalID  string    `json:"external_id"`
	URL         string    `json:"url"`
	Stage       string    `json:"stage"`
	Reason      string    `json:"reason"`
	Attempts    int       `json:"attempts"`
	NextRetryAt time.Time `json:"next_retry_at,omitempty"`
}
