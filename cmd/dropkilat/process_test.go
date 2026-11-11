package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestInvalidConfigProcess(t *testing.T) {
	if os.Getenv("DK_TEST_INVALID_CHILD") == "1" {
		main()
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G204 -- executable is the current test binary from os.Executable, never user input.
	cmd := exec.Command(executable, "-test.run=^TestInvalidConfigProcess$")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "DK_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "DK_TEST_INVALID_CHILD=1", "DK_ENCRYPTION_KEY=do-not-log-this-secret")
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("expected exit 1, got %v", err)
	}
	for _, name := range []string{"DK_ENV", "DK_HTTP_ADDR", "DK_BASE_URL", "DK_DATABASE_URL", "DK_MIGRATE_DATABASE_URL", "DK_ENCRYPTION_KEY", "DK_HASH_KEY", "DK_UPLOAD_DIR"} {
		if !strings.Contains(string(output), name) {
			t.Errorf("missing variable %s", name)
		}
	}
	if strings.Contains(string(output), "do-not-log-this-secret") {
		t.Fatal("configuration value leaked")
	}
}
