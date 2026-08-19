package config

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	c := Default()
	if c.SeedURL != "https://www.baoan.gov.cn/xxgk/fgk/" {
		t.Fatalf("SeedURL=%q", c.SeedURL)
	}
	if c.RequestInterval != time.Second || c.HTMLMaxBytes != 10<<20 || c.AttachmentMaxBytes != 100<<20 {
		t.Fatalf("limits=%+v", c)
	}
	if len(c.AllowedHosts) != 1 || c.AllowedHosts[0] != "www.baoan.gov.cn" {
		t.Fatalf("hosts=%v", c.AllowedHosts)
	}
}
