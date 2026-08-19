package model

import "time"

type IndexRecord struct {
	ID string `json:"id"`
	Title string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	Content string `json:"content,omitempty"`
	Abstract string `json:"abstract,omitempty"`
	Source string `json:"source,omitempty"`
	Date string `json:"date,omitempty"`
	URL string `json:"url,omitempty"`
	DocumentNumber string `json:"wh,omitempty"`
	Theme string `json:"zt,omitempty"`
	CarrierType string `json:"tc,omitempty"`
	ServiceObjects []string `json:"wjlx,omitempty"`
	ApplicationStart int64 `json:"sbks,omitempty"`
	ApplicationEnd int64 `json:"sbjs,omitempty"`
	ExpiredAt int64 `json:"expired_time,omitempty"`
}

type Attachment struct { ID int64 `json:"id"`; Name, Type, MIME, URL string; Size int64 `json:"size,string,omitempty"` }
type DownloadedAttachment struct { Attachment; ActualSize int64; SHA256 string; StoragePath string }
type RelatedPost struct { ID int64 `json:"id"`; Title, Type, TypeLabel, URL, Status string; Rank, OldRank, RelatedClassify int `json:"rank"`; CreatedAt, PublishedAt, FirstPublishedAt, DisplayPublishedAt, CreateTime int64 }
type FieldConflict struct { Field string; TopLevel, Nested string }
type IndexDiff struct { Added, Changed, Missing []string `json:"-"` }

type OfficialFacts struct {
	ServiceObjects []string `json:"service_objects"`
	IssuingAuthority string `json:"issuing_authority"`
	Theme string `json:"theme"`
	CarrierType string `json:"carrier_type"`
	DocumentGenre string `json:"document_genre"`
}

type StructuredPolicy struct {
	ID int64 `json:"id"`
	Title, DocumentNumber, SourceURL, FinalURL string
	Abstract, Markdown string
	Official OfficialFacts `json:"official"`
	PublishedAt, EffectiveAt, ExpiresAt, ApplicationStart, ApplicationEnd string
	OfficialListed bool `json:"official_listed"`
	LocalApplicationStatus string `json:"local_application_status"`
	Conflicts []FieldConflict `json:"conflicts,omitempty"`
}

type Package struct {
	ExternalID string
	DetailRaw, SourceHTML []byte
	Markdown string
	Structured, Relations []byte
	Attachments []DownloadedAttachment
}

type Manifest struct {
	SchemaVersion string `json:"schema_version"`
	PackageID, ExternalID, SnapshotID string `json:"package_id"`
	CanonicalURL, FinalURL string `json:"canonical_url"`
	FetchedAt time.Time `json:"fetched_at"`
	SnapshotSHA256 string `json:"snapshot_sha256"`
	Files []string `json:"files"`
}

type Failure struct { RunID, URL, Stage, Reason string; Attempts int; NextRetryAt time.Time }
