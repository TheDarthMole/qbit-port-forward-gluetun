//go:build container

package main

import (
	"bufio"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// These tests execute /app/main inside the container-test Docker target.
func TestContainerRuntime(t *testing.T) {
	if os.Getuid() == 0 || os.Getgid() == 0 {
		t.Fatal("the runtime must use a non-root user and group")
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		t.Fatal(err)
	}
	if len(roots.Subjects()) == 0 {
		t.Fatal("the runtime has no trusted CA certificates")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/app/main")
	cmd.Env = []string{}
	output, err := cmd.CombinedOutput()
	if err == nil || cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 1 {
		t.Fatalf("missing API key: exit error = %v, output = %s", err, output)
	}
	if !strings.Contains(string(output), "QBT_API_KEY is required") {
		t.Fatalf("unexpected startup output: %s", output)
	}
}

func TestContainerSync(t *testing.T) {
	for _, https := range []bool{false, true} {
		t.Run(fmt.Sprintf("HTTPS=%t", https), func(t *testing.T) {
			var port, updates atomic.Int64
			port.Store(12345)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v1/portforward" && r.Method == http.MethodGet {
					fmt.Fprint(w, `{"port":51413}`)
					return
				}
				if r.Header.Get("Authorization") != "Bearer container_test_key" {
					http.Error(w, "invalid API key", http.StatusForbidden)
					return
				}
				switch {
				case r.URL.Path == "/api/v2/app/preferences" && r.Method == http.MethodGet:
					fmt.Fprintf(w, `{"listen_port":%d}`, port.Load())
				case r.URL.Path == "/api/v2/app/setPreferences" && r.Method == http.MethodPost:
					if err := r.ParseForm(); err != nil || r.PostForm.Get("json") != `{"listen_port": 51413}` {
						http.Error(w, "invalid preferences", http.StatusBadRequest)
						return
					}
					port.Store(51413)
					updates.Add(1)
				default:
					http.NotFound(w, r)
				}
			})
			server := httptest.NewUnstartedServer(handler)
			if https {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()

			env := []string{
				"QBT_API_KEY=container_test_key",
				"QBT_ADDR=" + server.URL,
				"GTN_ADDR=" + server.URL,
			}
			if https {
				runContainerApp(t, env, "certificate signed by unknown authority")
				if updates.Load() != 0 {
					t.Fatal("updated preferences despite an untrusted certificate")
				}

				// Add a private CA through Go's standard certificate-directory support.
				// The distribution's public bundle remains available at its default path.
				certDir := t.TempDir()
				cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
				if err := os.WriteFile(filepath.Join(certDir, "private-ca.crt"), cert, 0644); err != nil {
					t.Fatal(err)
				}
				env = append(env, "SSL_CERT_DIR="+certDir)
			}

			runContainerApp(t, env, "Successfully updated port")
			if port.Load() != 51413 || updates.Load() != 1 {
				t.Fatalf("port = %d, updates = %d; want 51413 and 1", port.Load(), updates.Load())
			}
			runContainerApp(t, env, "Port already set, skipping...")
			if updates.Load() != 1 {
				t.Fatal("changed preferences when the port was already correct")
			}
		})
	}
}

func runContainerApp(t *testing.T, env []string, want string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/app/main")
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		_ = cmd.Wait()
	}()

	var output strings.Builder
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		fmt.Fprintln(&output, line)
		if strings.Contains(line, want) {
			return
		}
	}
	t.Fatalf("application never logged %q (context: %v, read: %v):\n%s", want, ctx.Err(), scanner.Err(), output.String())
}
