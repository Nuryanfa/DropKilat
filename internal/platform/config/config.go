// Package config validates environment configuration without exposing values.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type XenditConfig struct{ SecretKey, CallbackToken, SplitRuleID string }
type WhatsAppConfig struct{ PhoneNumberID, AccessToken, AppSecret, VerifyToken, BusinessNumber string }
type Config struct {
	Env, HTTPAddr                   string
	BaseURL                         *url.URL
	DatabaseURL, MigrateDatabaseURL string
	EncryptionKey, HashKey          []byte
	UploadDir                       string
	OwnerPhones                     []string
	FeatureXendit                   bool
	Xendit                          XenditConfig
	FeatureWASend                   bool
	WhatsApp                        WhatsAppConfig
}

var phone = regexp.MustCompile(`^\+[1-9][0-9]{1,14}$`)

func Load(getenv func(string) string) (Config, error) {
	var c Config
	var problems []error
	bad := func(name string) { problems = append(problems, fmt.Errorf("%s: missing or invalid", name)) }
	required := func(name string) string {
		v := getenv(name)
		if strings.TrimSpace(v) == "" {
			bad(name)
		}
		return v
	}
	c.Env = required("DK_ENV")
	if c.Env != "" && c.Env != "dev" && c.Env != "prod" {
		bad("DK_ENV")
	}
	c.HTTPAddr = required("DK_HTTP_ADDR")
	if c.HTTPAddr != "" {
		_, port, err := net.SplitHostPort(c.HTTPAddr)
		n, e := strconv.Atoi(port)
		if err != nil || e != nil || n < 1 || n > 65535 {
			bad("DK_HTTP_ADDR")
		}
	}
	base := required("DK_BASE_URL")
	u, err := url.Parse(base)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		if base != "" {
			bad("DK_BASE_URL")
		}
	} else {
		c.BaseURL = u
	}
	database := func(name string) string {
		v := required(name)
		u, e := url.Parse(v)
		if v != "" && (e != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" || u.Path == "" || u.Path == "/") {
			bad(name)
		}
		return v
	}
	c.DatabaseURL = database("DK_DATABASE_URL")
	c.MigrateDatabaseURL = database("DK_MIGRATE_DATABASE_URL")
	key := func(name string) []byte {
		v := required(name)
		b, e := base64.StdEncoding.DecodeString(v)
		if v != "" && (e != nil || len(b) != 32) {
			bad(name)
		}
		return b
	}
	c.EncryptionKey = key("DK_ENCRYPTION_KEY")
	c.HashKey = key("DK_HASH_KEY")
	c.UploadDir = required("DK_UPLOAD_DIR")
	// An empty owner list disables owner access; each configured entry must be E.164.
	if v := getenv("DK_OWNER_PHONES"); v != "" {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if !phone.MatchString(p) {
				bad("DK_OWNER_PHONES")
			}
			c.OwnerPhones = append(c.OwnerPhones, p)
		}
	}
	flag := func(name string) bool {
		v := getenv(name)
		if v == "" {
			return false
		}
		if v != "true" && v != "false" {
			bad(name)
		}
		return v == "true"
	}
	c.FeatureXendit = flag("DK_FEATURE_XENDIT")
	c.FeatureWASend = flag("DK_FEATURE_WA_SEND")
	if c.FeatureXendit {
		c.Xendit = XenditConfig{required("DK_XENDIT_SECRET_KEY"), required("DK_XENDIT_CALLBACK_TOKEN"), required("DK_XENDIT_SPLIT_RULE_ID")}
	}
	if c.FeatureWASend {
		c.WhatsApp = WhatsAppConfig{required("DK_WA_PHONE_NUMBER_ID"), required("DK_WA_ACCESS_TOKEN"), required("DK_WA_APP_SECRET"), required("DK_WA_VERIFY_TOKEN"), required("DK_WA_BUSINESS_NUMBER")}
		if c.WhatsApp.BusinessNumber != "" && !phone.MatchString(c.WhatsApp.BusinessNumber) {
			bad("DK_WA_BUSINESS_NUMBER")
		}
	}
	return c, errors.Join(problems...)
}
