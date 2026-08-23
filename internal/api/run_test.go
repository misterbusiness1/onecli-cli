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

	got := make(http.Header)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"env":{},"caCertificate":"","caCertificateContainerPath":""}`))
	}))
	defer srv.Close()

	client := newWithPrefix(srv.URL, "", "/v1")
	if _, err := client.GetContainerConfig(context.Background(), "occ-plugin-engineer"); err != nil {
		t.Fatal(err)
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

	client := newWithPrefix(srv.URL, "", "/v1")
	if _, err := client.GetContainerConfig(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	for _, item := range paperclipRunHeaderEnv {
		if got.Get(item.header) != "" {
			t.Errorf("unexpected %s header", item.header)
		}
	}
}
