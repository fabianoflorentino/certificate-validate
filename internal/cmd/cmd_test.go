package cmd

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fabianoflorentino/certificate-validate/internal/certificate"
	"github.com/fabianoflorentino/certificate-validate/internal/checker"
	"github.com/fabianoflorentino/certificate-validate/internal/config"
)

// mockChecker is a local mock implementation of checker.CertChecker.
type mockChecker struct {
	checkFunc    func(ctx context.Context, hostname string, port int) (*certificate.Certificate, error)
	checkAllFunc func(ctx context.Context, hosts []checker.Host, maxParallel int) ([]*certificate.Certificate, []error)
}

func (m *mockChecker) Check(ctx context.Context, hostname string, port int) (*certificate.Certificate, error) {
	if m.checkFunc != nil {
		return m.checkFunc(ctx, hostname, port)
	}
	return nil, nil
}

func (m *mockChecker) CheckAll(ctx context.Context, hosts []checker.Host, maxParallel int) ([]*certificate.Certificate, []error) {
	if m.checkAllFunc != nil {
		return m.checkAllFunc(ctx, hosts, maxParallel)
	}
	return nil, nil
}

// 1. Test buildApp returns a non-nil *checker.Checker with no error.
func TestBuildApp(t *testing.T) {
	c, err := buildApp()
	if err != nil {
		t.Fatalf("buildApp() error = %v; want nil", err)
	}
	if c == nil {
		t.Fatal("buildApp() returned nil; want non-nil *checker.Checker")
	}
}

// 2. Test toCheckerHostsFromConfig converts config.HostConfig to checker.Host slice.
func TestToCheckerHostsFromConfig(t *testing.T) {
	cfgHosts := []config.HostConfig{
		{Name: "github", URL: "github.com", Port: "443"},
		{Name: "gitlab", URL: "gitlab.com", Port: "443"},
	}

	got := toCheckerHostsFromConfig(cfgHosts)
	if len(got) != 2 {
		t.Fatalf("len(toCheckerHostsFromConfig(cfgHosts)) = %d; want 2", len(got))
	}

	if got[0].Hostname != "github.com" || got[0].Port != 443 || got[0].Name != "github" {
		t.Errorf("got[0] = %+v; want Hostname=github.com, Port=443, Name=github", got[0])
	}
	if got[1].Hostname != "gitlab.com" || got[1].Port != 443 || got[1].Name != "gitlab" {
		t.Errorf("got[1] = %+v; want Hostname=gitlab.com, Port=443, Name=gitlab", got[1])
	}
}

// 3. Test toCheckerHostsFromConfig returns empty for nil input.
func TestToCheckerHostsFromConfig_Nil(t *testing.T) {
	got := toCheckerHostsFromConfig(nil)
	if len(got) != 0 {
		t.Fatalf("len(toCheckerHostsFromConfig(nil)) = %d; want 0", len(got))
	}
}

// 4. Test getAPIHost returns host from first AppConfig with non-empty Host.
func TestGetAPIHost_FirstNonEmpty(t *testing.T) {
	cfg := &config.Config{
		AppConfigs: []config.AppConfig{
			{Name: "first", Host: "127.0.0.1", Port: "8080"},
			{Name: "second", Host: "0.0.0.0", Port: "5000"},
		},
	}
	got := getAPIHost(cfg)
	if got != "127.0.0.1" {
		t.Errorf("getAPIHost(cfg) = %q; want %q", got, "127.0.0.1")
	}
}

// 5. Test getAPIHost returns default "0.0.0.0" when no configs or all empty.
func TestGetAPIHost_Default(t *testing.T) {
	cfg := &config.Config{
		AppConfigs: []config.AppConfig{
			{Name: "empty", Host: "", Port: ""},
		},
	}
	got := getAPIHost(cfg)
	if got != "0.0.0.0" {
		t.Errorf("getAPIHost(cfg) = %q; want %q", got, "0.0.0.0")
	}

	cfgEmpty := &config.Config{}
	gotEmpty := getAPIHost(cfgEmpty)
	if gotEmpty != "0.0.0.0" {
		t.Errorf("getAPIHost(empty) = %q; want %q", gotEmpty, "0.0.0.0")
	}
}

// 6. Test getAPIPort returns port from first AppConfig with non-empty Port.
func TestGetAPIPort_FirstNonEmpty(t *testing.T) {
	cfg := &config.Config{
		AppConfigs: []config.AppConfig{
			{Name: "first", Host: "127.0.0.1", Port: "8080"},
			{Name: "second", Host: "0.0.0.0", Port: "5000"},
		},
	}
	got := getAPIPort(cfg)
	if got != "8080" {
		t.Errorf("getAPIPort(cfg) = %q; want %q", got, "8080")
	}
}

// 7. Test getAPIPort returns default "5000" when no configs or all empty.
func TestGetAPIPort_Default(t *testing.T) {
	cfg := &config.Config{
		AppConfigs: []config.AppConfig{
			{Name: "empty", Host: "", Port: ""},
		},
	}
	got := getAPIPort(cfg)
	if got != "5000" {
		t.Errorf("getAPIPort(cfg) = %q; want %q", got, "5000")
	}

	cfgEmpty := &config.Config{}
	gotEmpty := getAPIPort(cfgEmpty)
	if gotEmpty != "5000" {
		t.Errorf("getAPIPort(empty) = %q; want %q", gotEmpty, "5000")
	}
}

// 8. Test runWatchLoop calls CheckAll.
func TestRunWatchLoop_CallsCheckAll(t *testing.T) {
	called := make(chan struct{}, 1)
	mc := &mockChecker{
		checkAllFunc: func(ctx context.Context, hosts []checker.Host, maxParallel int) ([]*certificate.Certificate, []error) {
			select {
			case called <- struct{}{}:
			default:
			}
			return nil, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runWatchLoop(ctx, mc, []checker.Host{{Hostname: "example.com", Port: 443}}, 10*time.Millisecond)

	select {
	case <-called:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("CheckAll was not called within timeout")
	}
}

// 9. Test runWatchLoop stops when context is cancelled.
func TestRunWatchLoop_StopsOnCancel(t *testing.T) {
	mc := &mockChecker{
		checkAllFunc: func(ctx context.Context, hosts []checker.Host, maxParallel int) ([]*certificate.Certificate, []error) {
			return nil, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		runWatchLoop(ctx, mc, nil, 10*time.Millisecond)
		close(done)
	}()

	// Allow the loop to start.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("runWatchLoop did not stop after context cancellation")
	}
}

// 10. Test runWatchLoop prints certificate JSON (use formatted cert data from CheckAll).
func TestRunWatchLoop_PrintsCertificateJSON(t *testing.T) {
	cert := &certificate.Certificate{
		CommonName: "test.example.com",
		Hostname:   "test.example.com",
		Port:       443,
		DaysLeft:   100,
	}

	mc := &mockChecker{
		checkAllFunc: func(ctx context.Context, hosts []checker.Host, maxParallel int) ([]*certificate.Certificate, []error) {
			return []*certificate.Certificate{cert}, nil
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		runWatchLoop(ctx, mc, []checker.Host{{Hostname: "test.example.com", Port: 443}}, 50*time.Millisecond)
		close(done)
	}()

	// Wait for at least one iteration to print.
	time.Sleep(150 * time.Millisecond)

	cancel()
	<-done

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	os.Stdout = oldStdout

	output, _ := io.ReadAll(r)

	expected, _ := json.MarshalIndent(cert, "", "  ")
	if !strings.Contains(string(output), string(expected)) {
		t.Errorf("stdout did not contain expected JSON.\nGot:\n%s\nExpected to contain:\n%s", string(output), string(expected))
	}
}
