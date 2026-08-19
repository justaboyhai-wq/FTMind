package config

import "time"

type Config struct {
	SeedURL                   string
	SourceBaseURL             string
	DataDir                   string
	AllowedHosts              []string
	RequestInterval           time.Duration
	HTMLMaxBytes              int64
	AttachmentMaxBytes        int64
	PolicyAttachmentsMaxBytes int64
	RequestTimeout            time.Duration
	RetryCount                int
	IncrementalCron           string
	FullCron                  string
}

func Default() Config {
	return Config{
		SeedURL:                   "https://www.baoan.gov.cn/xxgk/fgk/",
		SourceBaseURL:             "https://www.baoan.gov.cn",
		DataDir:                   "./baoan-policy-data",
		AllowedHosts:              []string{"www.baoan.gov.cn"},
		RequestInterval:           time.Second,
		HTMLMaxBytes:              10 << 20,
		AttachmentMaxBytes:        100 << 20,
		PolicyAttachmentsMaxBytes: 500 << 20,
		RequestTimeout:            30 * time.Second,
		RetryCount:                3,
		IncrementalCron:           "0 2 * * *",
		FullCron:                  "0 3 * * 0",
	}
}
