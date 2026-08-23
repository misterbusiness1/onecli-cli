package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var paperclipRunHeaderEnv = []struct {
	header string
	env    string
}{
	{"X-Paperclip-OneCLI-Run-Binding", "PAPERCLIP_ONECLI_RUNTIME_BINDING"},
	{"X-Paperclip-Run-Id", "PAPERCLIP_RUN_ID"},
	{"X-Paperclip-Agent-Id", "PAPERCLIP_AGENT_ID"},
	{"X-Paperclip-Company-Id", "PAPERCLIP_COMPANY_ID"},
}

// PaperclipRunHeadersFromEnv returns an immutable snapshot of a complete
// Paperclip run context. With no context it returns (nil, false, nil), allowing
// explicit operator mode. Any partial context is rejected before networking.
func PaperclipRunHeadersFromEnv() (http.Header, bool, error) {
	headers := make(http.Header)
	present := 0
	for _, item := range paperclipRunHeaderEnv {
		if value := strings.TrimSpace(os.Getenv(item.env)); value != "" {
			headers.Set(item.header, value)
			present++
		}
	}
	if present == 0 {
		return nil, false, nil
	}
	if present != len(paperclipRunHeaderEnv) {
		return nil, false, fmt.Errorf("incomplete Paperclip run context: all PAPERCLIP_ONECLI_RUNTIME_BINDING, PAPERCLIP_RUN_ID, PAPERCLIP_AGENT_ID, and PAPERCLIP_COMPANY_ID values are required")
	}
	return headers, true, nil
}

// ContainerConfig is the response from GET /v1/container-config.
// The server controls all env var names, values, and paths.
type ContainerConfig struct {
	Env                        map[string]string `json:"env"`
	CACertificate              string            `json:"caCertificate"`
	CACertificateContainerPath string            `json:"caCertificateContainerPath"`
	Warnings                   []string          `json:"warnings,omitempty"`
}

// GetContainerConfig returns gateway configuration for a local agent process.
// agentIdentifier may be empty, in which case the server uses the default agent.
func (c *Client) GetContainerConfig(ctx context.Context, agentIdentifier string) (*ContainerConfig, error) {
	path := "/v1/container-config"
	if agentIdentifier != "" {
		q := url.Values{}
		q.Set("agent", agentIdentifier)
		path += "?" + q.Encode()
	}
	var cfg ContainerConfig
	if err := c.do(ctx, http.MethodGet, path, nil, &cfg); err != nil {
		return nil, fmt.Errorf("getting container config: %w", err)
	}
	return &cfg, nil
}
