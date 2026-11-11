package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func valid() map[string]string {
	return map[string]string{
		"DK_ENV": "dev", "DK_HTTP_ADDR": ":8081", "DK_BASE_URL": "http://localhost:8081",
		"DK_DATABASE_URL": "postgres://app@localhost/db", "DK_MIGRATE_DATABASE_URL": "postgres://owner@localhost/db",
		"DK_ENCRYPTION_KEY": base64.StdEncoding.EncodeToString(make([]byte, 32)), "DK_HASH_KEY": base64.StdEncoding.EncodeToString(make([]byte, 32)),
		"DK_UPLOAD_DIR": "./uploads", "DK_OWNER_PHONES": "+628123456789,+628987654321",
	}
}
func TestLoad(t *testing.T) {
	for _, tc := range []struct{ name, variable, value string }{
		{"missing", "DK_UPLOAD_DIR", ""}, {"base64", "DK_ENCRYPTION_KEY", "secret%%%"},
		{"short key", "DK_HASH_KEY", "c2VjcmV0"}, {"owner", "DK_OWNER_PHONES", "08123456789"},
		{"environment", "DK_ENV", "stage"}, {"address", "DK_HTTP_ADDR", ":bad"},
		{"base URL", "DK_BASE_URL", "ftp://example.com"}, {"database", "DK_DATABASE_URL", "not-a-url"},
		{"flag", "DK_FEATURE_XENDIT", "maybe"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := valid()
			env[tc.variable] = tc.value
			_, err := Load(func(k string) string { return env[k] })
			if err == nil || !strings.Contains(err.Error(), tc.variable) {
				t.Fatalf("expected error for %s", tc.variable)
			}
			if tc.value != "" && strings.Contains(err.Error(), tc.value) {
				t.Fatal("configuration value leaked")
			}
		})
	}
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			env := valid()
			if enabled {
				env["DK_FEATURE_XENDIT"] = "true"
				env["DK_FEATURE_WA_SEND"] = "true"
			}
			_, err := Load(func(k string) string { return env[k] })
			if !enabled {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			names := []string{"DK_XENDIT_SECRET_KEY", "DK_XENDIT_CALLBACK_TOKEN", "DK_XENDIT_SPLIT_RULE_ID", "DK_WA_PHONE_NUMBER_ID", "DK_WA_ACCESS_TOKEN", "DK_WA_APP_SECRET", "DK_WA_VERIFY_TOKEN", "DK_WA_BUSINESS_NUMBER"}
			if err == nil {
				t.Fatal("expected provider errors")
			}
			for _, name := range names {
				if !strings.Contains(err.Error(), name) {
					t.Errorf("missing %s", name)
				}
				env[name] = "provider-secret"
			}
			env["DK_WA_BUSINESS_NUMBER"] = "+628123456789"
			cfg, err := Load(func(k string) string { return env[k] })
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Xendit.SecretKey != "provider-secret" || cfg.WhatsApp.AccessToken != "provider-secret" {
				t.Fatal("provider values not loaded")
			}
		})
	}
}
func TestLoadReportsAllProblems(t *testing.T) {
	_, err := Load(func(string) string { return "" })
	if err == nil {
		t.Fatal("expected errors")
	}
	for _, name := range []string{"DK_ENV", "DK_HTTP_ADDR", "DK_BASE_URL", "DK_DATABASE_URL", "DK_MIGRATE_DATABASE_URL", "DK_ENCRYPTION_KEY", "DK_HASH_KEY", "DK_UPLOAD_DIR"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("missing %s", name)
		}
	}
}
