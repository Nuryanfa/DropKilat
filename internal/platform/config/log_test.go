package config

import (
	"bytes"
	"log/slog"
	"net/url"
	"strings"
	"testing"
)

func TestLogValue(t *testing.T) {
	secret := "SENSITIVE-DO-NOT-LOG"
	u, _ := url.Parse("https://example.test/" + secret)
	c := Config{Env: "prod", HTTPAddr: ":8080", BaseURL: u, DatabaseURL: secret, MigrateDatabaseURL: secret, EncryptionKey: []byte(secret), HashKey: []byte(secret), UploadDir: secret, OwnerPhones: []string{secret}, FeatureXendit: true, Xendit: XenditConfig{secret, secret, secret}, FeatureWASend: true, WhatsApp: WhatsAppConfig{secret, secret, secret, secret, secret}}
	var b bytes.Buffer
	slog.New(slog.NewJSONHandler(&b, nil)).Info("config", "config", c)
	if strings.Contains(b.String(), secret) || !strings.Contains(b.String(), "DK_WA_ACCESS_TOKEN") || !strings.Contains(b.String(), "prod") {
		t.Fatal(b.String())
	}
}
