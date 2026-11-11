//go:build unix

package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

// The subprocess uses the same signal setup and server lifecycle as main.
func TestSIGTERMDrainsProcess(t *testing.T) {
	if os.Getenv("DK_TEST_SIGNAL_CHILD") == "1" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		listener, err := net.Listen("tcp", os.Getenv("DK_TEST_SIGNAL_ADDR"))
		if err != nil {
			t.Fatal(err)
		}
		mux := http.NewServeMux()
		mux.Handle("GET /healthz", healthHandler())
		mux.HandleFunc("GET /slow", func(w http.ResponseWriter, _ *http.Request) {
			// #nosec G703 -- marker is a parent-test-generated path inside t.TempDir.
			if err := os.WriteFile(os.Getenv("DK_TEST_SIGNAL_MARKER"), []byte("entered"), 0600); err != nil {
				t.Error(err)
			}
			time.Sleep(300 * time.Millisecond)
			w.WriteHeader(http.StatusNoContent)
		})
		if err := serve(ctx, listener, mux); err != nil {
			t.Fatal(err)
		}
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	marker := t.TempDir() + "/entered"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// #nosec G204 -- executable is the current test binary from os.Executable, never user input.
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestSIGTERMDrainsProcess$")
	cmd.Env = append(os.Environ(), "DK_TEST_SIGNAL_CHILD=1", "DK_TEST_SIGNAL_ADDR="+addr, "DK_TEST_SIGNAL_MARKER="+marker)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			if err := cmd.Process.Kill(); err != nil {
				t.Log(err)
			}
			if err := cmd.Wait(); err != nil {
				t.Log(err)
			}
		}
	}()
	client := &http.Client{Timeout: time.Second}
	for {
		resp, e := client.Get("http://" + addr + "/healthz")
		if e == nil {
			if err := resp.Body.Close(); err != nil {
				t.Fatal(err)
			}
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("child never became healthy")
		case <-time.After(10 * time.Millisecond):
		}
	}
	request := make(chan error, 1)
	go func() {
		resp, e := client.Get("http://" + addr + "/slow")
		if e == nil {
			e = resp.Body.Close()
			if resp.StatusCode != 204 {
				t.Error("in-flight request failed")
			}
		}
		request <- e
	}()
	for {
		if _, e := os.Stat(marker); e == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("request never entered")
		case <-time.After(5 * time.Millisecond):
		}
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := <-request; err != nil {
		t.Fatal(err)
	}
	err = cmd.Wait()
	waited = true
	if err != nil {
		t.Fatalf("child did not exit cleanly: %v", err)
	}
}
