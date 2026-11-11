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

func TestLoad_QAEdgeCasesAndSecretNonLeakage(t *testing.T) {
	key31 := base64.StdEncoding.EncodeToString(make([]byte, 31))
	key33 := base64.StdEncoding.EncodeToString(make([]byte, 33))
	urlSafeKey := strings.Repeat("-", 43) + "="

	cases := []struct {
		name     string
		mutate   func(map[string]string)
		wantVars []string
		secret   string
	}{
		{
			name:     "31-byte encryption key rejected",
			mutate:   func(e map[string]string) { e["DK_ENCRYPTION_KEY"] = key31 },
			wantVars: []string{"DK_ENCRYPTION_KEY"},
			secret:   key31,
		},
		{
			name:     "33-byte hash key rejected",
			mutate:   func(e map[string]string) { e["DK_HASH_KEY"] = key33 },
			wantVars: []string{"DK_HASH_KEY"},
			secret:   key33,
		},
		{
			name:     "URL-safe base64 rejected by StdEncoding",
			mutate:   func(e map[string]string) { e["DK_ENCRYPTION_KEY"] = urlSafeKey },
			wantVars: []string{"DK_ENCRYPTION_KEY"},
			secret:   urlSafeKey,
		},
		{
			name:     "base URL with userinfo rejected without leaking credentials",
			mutate:   func(e map[string]string) { e["DK_BASE_URL"] = "https://admin:supersecret-url-pass@dropkilat.id" },
			wantVars: []string{"DK_BASE_URL"},
			secret:   "supersecret-url-pass",
		},
		{
			name: "database URLs with invalid scheme or empty path do not leak password",
			mutate: func(e map[string]string) {
				e["DK_DATABASE_URL"] = "mysql://app:db-secret-pass-1@localhost:3306/dropkilat"
				e["DK_MIGRATE_DATABASE_URL"] = "postgres://owner:db-secret-pass-2@localhost:5432/"
			},
			wantVars: []string{"DK_DATABASE_URL", "DK_MIGRATE_DATABASE_URL"},
			secret:   "db-secret-pass",
		},
		{
			name:     "port 0 rejected",
			mutate:   func(e map[string]string) { e["DK_HTTP_ADDR"] = ":0" },
			wantVars: []string{"DK_HTTP_ADDR"},
		},
		{
			name:     "port 65536 rejected",
			mutate:   func(e map[string]string) { e["DK_HTTP_ADDR"] = ":65536" },
			wantVars: []string{"DK_HTTP_ADDR"},
		},
		{
			name: "non-E.164 WhatsApp business number rejected when feature enabled",
			mutate: func(e map[string]string) {
				e["DK_FEATURE_WA_SEND"] = "true"
				e["DK_WA_PHONE_NUMBER_ID"] = "123"
				e["DK_WA_ACCESS_TOKEN"] = "wa-token-secret"
				e["DK_WA_APP_SECRET"] = "wa-app-secret"
				e["DK_WA_VERIFY_TOKEN"] = "wa-verify-secret"
				e["DK_WA_BUSINESS_NUMBER"] = "081234567890"
			},
			wantVars: []string{"DK_WA_BUSINESS_NUMBER"},
			secret:   "081234567890",
		},
		{
			name:     "owner phones trailing comma rejected",
			mutate:   func(e map[string]string) { e["DK_OWNER_PHONES"] = "+628123456789," },
			wantVars: []string{"DK_OWNER_PHONES"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := valid()
			tc.mutate(env)
			_, err := Load(func(k string) string { return env[k] })
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			for _, v := range tc.wantVars {
				if !strings.Contains(err.Error(), v) {
					t.Errorf("expected error to mention %s, got %q", v, err.Error())
				}
			}
			if tc.secret != "" && strings.Contains(err.Error(), tc.secret) {
				t.Fatalf("secret leaked in error output: %q", err.Error())
			}
		})
	}

	t.Run("owner phones trimmed when surrounded by spaces", func(t *testing.T) {
		env := valid()
		env["DK_OWNER_PHONES"] = " +628123456789 , +628987654321 "
		cfg, err := Load(func(k string) string { return env[k] })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.OwnerPhones) != 2 || cfg.OwnerPhones[0] != "+628123456789" || cfg.OwnerPhones[1] != "+628987654321" {
			t.Fatalf("unexpected OwnerPhones: %v", cfg.OwnerPhones)
		}
	})
}
