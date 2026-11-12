package config

import "log/slog"

// LogValue reports presence, never the contents of sensitive configuration.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("Env", c.Env), slog.String("HTTPAddr", c.HTTPAddr),
		slog.Bool("DK_BASE_URL", c.BaseURL != nil), slog.Bool("DK_DATABASE_URL", c.DatabaseURL != ""), slog.Bool("DK_MIGRATE_DATABASE_URL", c.MigrateDatabaseURL != ""),
		slog.Bool("DK_ENCRYPTION_KEY", len(c.EncryptionKey) > 0), slog.Bool("DK_HASH_KEY", len(c.HashKey) > 0), slog.Bool("DK_UPLOAD_DIR", c.UploadDir != ""), slog.Bool("DK_OWNER_PHONES", len(c.OwnerPhones) > 0),
		slog.Bool("DK_FEATURE_XENDIT", c.FeatureXendit), slog.Bool("DK_XENDIT_SECRET_KEY", c.Xendit.SecretKey != ""), slog.Bool("DK_XENDIT_CALLBACK_TOKEN", c.Xendit.CallbackToken != ""), slog.Bool("DK_XENDIT_SPLIT_RULE_ID", c.Xendit.SplitRuleID != ""),
		slog.Bool("DK_FEATURE_WA_SEND", c.FeatureWASend), slog.Bool("DK_WA_PHONE_NUMBER_ID", c.WhatsApp.PhoneNumberID != ""), slog.Bool("DK_WA_ACCESS_TOKEN", c.WhatsApp.AccessToken != ""), slog.Bool("DK_WA_APP_SECRET", c.WhatsApp.AppSecret != ""), slog.Bool("DK_WA_VERIFY_TOKEN", c.WhatsApp.VerifyToken != ""), slog.Bool("DK_WA_BUSINESS_NUMBER", c.WhatsApp.BusinessNumber != ""))
}
