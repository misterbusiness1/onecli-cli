package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestRunClientUsesOnlyAssignedAgentToken(t *testing.T) {
	token := "aoc_" + strings.Repeat("1", 64)
	configCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/health" {
			w.Write([]byte(`{"ok":true}`))
			return
		}
		if r.URL.Path != "/v1/container-config" {
			t.Errorf("unexpected management request: %s", r.URL.Path)
			w.WriteHeader(403)
			return
		}
		configCalls++
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("run did not use its assigned agent credential")
			w.WriteHeader(401)
			return
		}
		if r.URL.Query().Get("agent") == "cto" {
			w.WriteHeader(403)
			fmt.Fprint(w, `{"error":"Agent credential scope mismatch"}`)
			return
		}
		fmt.Fprint(w, `{"env":{},"caCertificate":"","caCertificateContainerPath":""}`)
	}))
	defer server.Close()
	t.Setenv("ONECLI_API_HOST", server.URL)
	t.Setenv("ONECLI_API_KEY", "oc_synthetic_management_key_must_not_be_used")
	t.Setenv("ONECLI_AGENT_TOKEN", token)
	client, err := newRunClient()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.GetContainerConfig(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if _, err = client.GetContainerConfig(context.Background(), "cto"); err == nil {
		t.Fatal("identity override should remain rejected by the server")
	}
	if _, err = client.ListAgents(context.Background(), ""); err == nil {
		t.Fatal("agent-only client must reject management requests before networking")
	}
	if configCalls != 2 {
		t.Fatalf("config requests=%d, expected2", configCalls)
	}
}

func TestRunClientInvalidAgentTokenNeverFallsBack(t *testing.T) {
	for _, token := range []string{"", "aoc_bad", "oc_synthetic_admin"} {
		t.Run(fmt.Sprintf("length%d", len(token)), func(t *testing.T) {
			t.Setenv("ONECLI_AGENT_TOKEN", token)
			t.Setenv("ONECLI_API_KEY", "oc_synthetic_management")
			if _, err := newRunClient(); err == nil {
				t.Fatal("invalid supplied token must not fall back to management login")
			}
		})
	}
}

func TestAgentScopedChildDoesNotReceiveManagementKey(t *testing.T) {
	token := "aoc_" + strings.Repeat("1", 64)
	env := buildChildEnv([]string{"ONECLI_AGENT_TOKEN=" + token, "ONECLI_API_KEY=oc_parent_fixture", "PATH=/usr/bin"}, map[string]string{"ONECLI_API_KEY": "oc_server_fixture"}, "")
	for _, entry := range env {
		if strings.HasPrefix(entry, "ONECLI_API_KEY=") {
			t.Fatal("management key leaked into agent child")
		}
	}
	found := false
	for _, entry := range env {
		if entry == "ONECLI_AGENT_TOKEN="+token {
			found = true
		}
	}
	if !found {
		t.Fatal("assigned agent credential was lost")
	}
}

func TestAgentTokenPreservesPaperclipContextAndRejectsPartialContext(t *testing.T) {
	token := "aoc_" + strings.Repeat("2", 64)
	t.Setenv("ONECLI_AGENT_TOKEN", token)
	t.Setenv("ONECLI_API_KEY", "oc_management_must_not_be_used")
	for name, value := range map[string]string{"PAPERCLIP_ONECLI_RUNTIME_BINDING": "binding-fixture", "PAPERCLIP_RUN_ID": "run-fixture", "PAPERCLIP_AGENT_ID": "agent-fixture", "PAPERCLIP_COMPANY_ID": "company-fixture"} {
		t.Setenv(name, value)
	}
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/container-config" || r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("X-Paperclip-OneCLI-Run-Binding") != "binding-fixture" || r.Header.Get("X-Paperclip-Company-Id") != "company-fixture" {
			t.Error("assigned credential or complete run binding was lost")
		}
		fmt.Fprint(w, `{"env":{},"caCertificate":"","caCertificateContainerPath":""}`)
	}))
	defer server.Close()
	t.Setenv("ONECLI_API_HOST", server.URL)
	client, err := newRunClient()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.GetContainerConfig(context.Background(), "worker"); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("requests=%d want exactly one configuration request", calls)
	}
	t.Setenv("PAPERCLIP_AGENT_ID", "")
	if _, err = newRunClient(); err == nil {
		t.Fatal("token must not bypass partial run-context rejection")
	}
}

// Paperclip's normal model adapters provide these three metadata values without
// minting a separate OneCLI runtime-binding capability.
func TestAssignedAgentAcceptsOrdinaryPaperclipMetadata(t *testing.T) {
	t.Setenv("PAPERCLIP_ONECLI_RUNTIME_BINDING", "")
	if err := os.Unsetenv("PAPERCLIP_ONECLI_RUNTIME_BINDING"); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"PAPERCLIP_RUN_ID": "run-fixture", "PAPERCLIP_AGENT_ID": "agent-fixture", "PAPERCLIP_COMPANY_ID": "company-fixture"} {
		t.Setenv(name, value)
	}
	token := "aoc_" + strings.Repeat("3", 64)
	t.Setenv("ONECLI_AGENT_TOKEN", token)
	t.Setenv("ONECLI_API_KEY", "oc_management_must_not_be_used")
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer "+token {
			t.Error("assigned token was not selected")
		}
		if r.Header.Get("X-Paperclip-OneCLI-Run-Binding") != "" {
			t.Error("runtime binding was fabricated")
		}
		if r.URL.Path != "/v1/container-config" {
			t.Error("unexpected management request")
		}
		fmt.Fprint(w, `{"env":{},"caCertificate":"","caCertificateContainerPath":""}`)
	}))
	defer server.Close()
	t.Setenv("ONECLI_API_HOST", server.URL)
	client, err := newRunClient()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.GetContainerConfig(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if _, err = client.ListAgents(context.Background(), ""); err == nil {
		t.Fatal("agent client must refuse management")
	}
	if calls != 1 {
		t.Fatalf("requests=%d want1", calls)
	}
}
func TestAssignedAgentExplicitEmptyRuntimeBindingFailsClosed(t *testing.T) {
	t.Setenv("ONECLI_AGENT_TOKEN", "aoc_"+strings.Repeat("4", 64))
	for _, name := range []string{"PAPERCLIP_RUN_ID", "PAPERCLIP_AGENT_ID", "PAPERCLIP_COMPANY_ID"} {
		t.Setenv(name, "")
	}
	for _, value := range []string{"", "   "} {
		t.Setenv("PAPERCLIP_ONECLI_RUNTIME_BINDING", value)
		if _, err := newRunClient(); err == nil {
			t.Fatal("explicit empty runtime binding must fail closed")
		}
	}
}
func TestOrdinaryPaperclipMetadataWithoutAssignedTokenCannotUseAdmin(t *testing.T) {
	t.Setenv("ONECLI_AGENT_TOKEN", "")
	if err := os.Unsetenv("ONECLI_AGENT_TOKEN"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PAPERCLIP_ONECLI_RUNTIME_BINDING", "")
	if err := os.Unsetenv("PAPERCLIP_ONECLI_RUNTIME_BINDING"); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"PAPERCLIP_RUN_ID": "run-fixture", "PAPERCLIP_AGENT_ID": "agent-fixture", "PAPERCLIP_COMPANY_ID": "company-fixture"} {
		t.Setenv(name, value)
	}
	t.Setenv("ONECLI_API_KEY", "oc_management_must_not_be_used")
	if _, err := newRunClient(); err == nil {
		t.Fatal("missing assigned token must not fall back to admin")
	}
}

func TestExplicitEmptyRuntimeBindingCannotUseOperatorFallback(t *testing.T) {
	t.Setenv("ONECLI_AGENT_TOKEN", "")
	if err := os.Unsetenv("ONECLI_AGENT_TOKEN"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"PAPERCLIP_RUN_ID", "PAPERCLIP_AGENT_ID", "PAPERCLIP_COMPANY_ID"} {
		t.Setenv(name, "")
	}
	t.Setenv("PAPERCLIP_ONECLI_RUNTIME_BINDING", "")
	t.Setenv("ONECLI_API_KEY", "oc_management_must_not_be_used")
	if _, err := newRunClient(); err == nil {
		t.Fatal("explicit empty binding must not select operator credentials")
	}
}
