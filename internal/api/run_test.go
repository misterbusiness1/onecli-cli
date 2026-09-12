package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetContainerConfigTransportsPaperclipRunContext(t *testing.T) {
	t.Setenv("PAPERCLIP_ONECLI_RUNTIME_BINDING", "binding-proof")
	t.Setenv("PAPERCLIP_RUN_ID", "run-proof")
	t.Setenv("PAPERCLIP_AGENT_ID", "agent-proof")
	t.Setenv("PAPERCLIP_COMPANY_ID", "company-proof")

	requests := 0
	got := make(http.Header)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/v1/container-config" {
			t.Errorf("request path = %q, want /v1/container-config", r.URL.Path)
		}
		got = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"env":{},"caCertificate":"","caCertificateContainerPath":""}`))
	}))
	defer srv.Close()

	headers, bound, err := PaperclipRunHeadersFromEnv()
	if err != nil || !bound {
		t.Fatalf("PaperclipRunHeadersFromEnv() = bound %v, err %v", bound, err)
	}
	client := NewPaperclipRun(srv.URL, headers)
	if _, err := client.GetContainerConfig(context.Background(), "occ-plugin-engineer"); err != nil {
		t.Fatal(err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want exactly one capability-bearing request", requests)
	}

	want := map[string]string{
		"X-Paperclip-OneCLI-Run-Binding": "binding-proof",
		"X-Paperclip-Run-Id":             "run-proof",
		"X-Paperclip-Agent-Id":           "agent-proof",
		"X-Paperclip-Company-Id":         "company-proof",
	}
	for name, value := range want {
		if got.Get(name) != value {
			t.Errorf("%s = %q, want %q", name, got.Get(name), value)
		}
	}
}

func TestGetContainerConfigOmitsPaperclipHeadersWhenUnbound(t *testing.T) {
	for _, item := range paperclipRunHeaderEnv {
		t.Setenv(item.env, "")
	}

	got := make(http.Header)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"env":{},"caCertificate":"","caCertificateContainerPath":""}`))
	}))
	defer srv.Close()

	headers, bound, err := PaperclipRunHeadersFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if bound || headers != nil {
		t.Fatalf("unbound context = headers %v, bound %v", headers, bound)
	}
	if len(got) != 0 {
		t.Fatalf("unexpected request before client construction: %v", got)
	}
}

func TestPaperclipRunHeadersRejectPartialContext(t *testing.T) {
	for _, item := range paperclipRunHeaderEnv {
		t.Setenv(item.env, "")
	}
	t.Setenv("PAPERCLIP_RUN_ID", "run-only")
	if _, _, err := PaperclipRunHeadersFromEnv(); err == nil {
		t.Fatal("expected partial Paperclip context to fail")
	}
}

func TestPaperclipRunClientRejectsManagementEndpointBeforeNetwork(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer srv.Close()
	client := NewPaperclipRun(srv.URL, http.Header{
		"X-Paperclip-Onecli-Run-Binding": {"binding"},
		"X-Paperclip-Run-Id":             {"run"},
		"X-Paperclip-Agent-Id":           {"agent"},
		"X-Paperclip-Company-Id":         {"company"},
	})
	if _, err := client.ListProjects(context.Background()); err == nil {
		t.Fatal("expected management endpoint rejection")
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want zero", requests)
	}
}
