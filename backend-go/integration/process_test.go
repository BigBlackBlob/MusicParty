package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMusicPartyProcessLifecycleAndProbes(t *testing.T) {
	if testing.Short() {
		t.Skip("process integration test")
	}
	moduleRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve module root: %v", err)
	}
	binaryName := "musicparty"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(t.TempDir(), binaryName)
	build := exec.Command("go", "build", "-trimpath", "-o", binaryPath, "./cmd/musicparty")
	build.Dir = moduleRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build process binary: %v\n%s", err, output)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release reserved port: %v", err)
	}

	var output bytes.Buffer
	process := exec.Command(binaryPath)
	process.Dir = moduleRoot
	process.Env = append(os.Environ(),
		fmt.Sprintf("SERVER_PORT=%d", port),
		fmt.Sprintf("BASE_URL=http://127.0.0.1:%d", port),
		"DB_PATH="+filepath.Join(t.TempDir(), "process.db"),
		"DB_INIT_SCHEMA=true",
		"AUTH_SECURE_COOKIES=false",
		"BOOTSTRAP_ADMIN_USERNAME=contract-admin",
		"BOOTSTRAP_ADMIN_PASSWORD=Contract-Password-123!",
	)
	process.Stdout = &output
	process.Stderr = &output
	if err := process.Start(); err != nil {
		t.Fatalf("start process: %v", err)
	}
	waited := false
	defer func() {
		if !waited && process.Process != nil {
			_ = process.Process.Kill()
			_ = process.Wait()
		}
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for {
		response, requestErr := client.Get(baseURL + "/actuator/health")
		if requestErr == nil {
			var health struct {
				Status string   `json:"status"`
				Groups []string `json:"groups"`
			}
			decodeErr := json.NewDecoder(response.Body).Decode(&health)
			response.Body.Close()
			if decodeErr == nil && response.StatusCode == http.StatusOK && health.Status == "UP" && len(health.Groups) == 2 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("health did not become ready\n%s", output.String())
		}
		time.Sleep(100 * time.Millisecond)
	}

	for _, path := range []string{"/actuator/health/readiness", "/actuator/prometheus", "/actuator/metrics"} {
		response, err := client.Get(baseURL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d", path, response.StatusCode)
		}
	}

	for _, probe := range []struct {
		path     string
		status   int
		contains string
	}{
		{"/api/config", http.StatusOK, `"authorName"`},
		{"/api/account/status", http.StatusOK, `"requiresSetup":false`},
		{"/api/rooms", http.StatusOK, `"roomId":"lounge"`},
		{"/api/platforms", http.StatusOK, `"local"`},
		{"/api/search/local/contract?offset=0&limit=20", http.StatusOK, "[]"},
		{"/api/search/local/contract?offset=-1&limit=20", http.StatusInternalServerError, "ResponseStatusException"},
	} {
		response, err := client.Get(baseURL + probe.path)
		if err != nil {
			t.Fatalf("GET %s: %v", probe.path, err)
		}
		bodyErr := func() error {
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				return err
			}
			if response.StatusCode != probe.status {
				return fmt.Errorf("status %d body %s", response.StatusCode, body)
			}
			if !bytes.Contains(body, []byte(probe.contains)) {
				return fmt.Errorf("body %s does not contain %s", body, probe.contains)
			}
			return nil
		}()
		if bodyErr != nil {
			t.Fatalf("GET %s: %v", probe.path, bodyErr)
		}
	}

	if runtime.GOOS == "windows" {
		if err := process.Process.Kill(); err != nil {
			t.Fatalf("kill process: %v", err)
		}
	} else if err := process.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("signal process: %v", err)
	}
	waitErr := process.Wait()
	waited = true
	if runtime.GOOS != "windows" && waitErr != nil {
		t.Fatalf("graceful process exit: %v\n%s", waitErr, output.String())
	}
	if !strings.Contains(output.String(), "musicparty Go backend listening") {
		t.Fatalf("startup log missing\n%s", output.String())
	}
}
