package anki

//go:generate mockgen -source ankiconnect_client.go -destination mock/ankiconnect_client_mock.go

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/shredd0r/anki-card-creator/config"
)

const (
	defaultBaseUrl     = "http://127.0.0.1:8765"
	ankiConnectVersion = 6
)

// newAnkiConnectHTTPClient builds an *http.Client with keep-alives disabled.
// AnkiConnect's built-in Python HTTP server doesn't reliably support
// persistent connections - it can close a connection the OS/Go transport
// still considers idle-but-reusable, which surfaces as a spurious "EOF" on
// whichever request next tries to reuse it. Since every call here is to
// localhost, paying for a fresh TCP handshake per request is negligible.
func newAnkiConnectHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{DisableKeepAlives: true},
	}
}

// Client is a thin transport seam over AnkiConnect's HTTP API, kept
// interface-shaped so implService stays mockable per the repo's convention.
type Client interface {
	// Invoke calls the given AnkiConnect action with params and unmarshals
	// the response's "result" field into result (pass a pointer, or nil to
	// discard it). Returns an error if the transport fails or the response's
	// "error" field is non-null.
	Invoke(ctx context.Context, action string, params any, result any) error
}

type implClient struct {
	httpClient *http.Client
	baseUrl    string
	token      string
}

// NewClient builds an AnkiConnect client. cfg.BaseUrl defaults to
// defaultBaseUrl when empty; cfg.Token, when non-empty, is sent as the
// AnkiConnect "key" field on every request.
func NewClient(httpClient *http.Client, cfg config.AnkiConnectConfig) Client {
	baseUrl := cfg.BaseUrl
	if baseUrl == "" {
		baseUrl = defaultBaseUrl
	}

	return &implClient{
		httpClient: httpClient,
		baseUrl:    baseUrl,
		token:      cfg.Token,
	}
}

type ankiConnectRequest struct {
	Action  string `json:"action"`
	Version int    `json:"version"`
	Params  any    `json:"params,omitempty"`
	Key     string `json:"key,omitempty"`
}

type ankiConnectResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *string         `json:"error"`
}

func (c *implClient) Invoke(ctx context.Context, action string, params any, result any) error {
	reqBody, err := json.Marshal(ankiConnectRequest{
		Action:  action,
		Version: ankiConnectVersion,
		Params:  params,
		Key:     c.token,
	})
	if err != nil {
		return fmt.Errorf("marshal request for action %q: %w", action, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseUrl, bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("build request for action %q: %w", action, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call anki connect action %q: %w", action, err)
	}
	defer resp.Body.Close()

	var respBody ankiConnectResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return fmt.Errorf("decode response for action %q: %w", action, err)
	}

	if respBody.Error != nil {
		return fmt.Errorf("anki connect action %q failed: %s", action, *respBody.Error)
	}

	if result != nil && len(respBody.Result) > 0 {
		if err := json.Unmarshal(respBody.Result, result); err != nil {
			return fmt.Errorf("unmarshal result for action %q: %w", action, err)
		}
	}

	return nil
}
