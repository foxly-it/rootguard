package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ServiceStatus struct {
	Name            string    `json:"name"`
	DisplayName     string    `json:"display_name"`
	CurrentImage    string    `json:"current_image,omitempty"`
	TargetImage     string    `json:"target_image"`
	CurrentID       string    `json:"current_id,omitempty"`
	CandidateID     string    `json:"candidate_id,omitempty"`
	UpdateAvailable bool      `json:"update_available"`
	CheckedAt       time.Time `json:"checked_at,omitempty"`
	Error           string    `json:"error,omitempty"`
}

type CleanupResult struct {
	RemovedImages  []string `json:"removed_images,omitempty"`
	RemovedVolumes []string `json:"removed_volumes,omitempty"`
	Skipped        []string `json:"skipped,omitempty"`
}

type HistoryEntry struct {
	Outcome   string            `json:"outcome"`
	FromIDs   map[string]string `json:"from_ids,omitempty"`
	ToIDs     map[string]string `json:"to_ids,omitempty"`
	Message   string            `json:"message"`
	Cleanup   CleanupResult     `json:"cleanup"`
	CreatedAt time.Time         `json:"created_at"`
}

type Status struct {
	State     string          `json:"state"`
	Message   string          `json:"message"`
	Services  []ServiceStatus `json:"services"`
	History   []HistoryEntry  `json:"history,omitempty"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type Client struct {
	baseURL   string
	token     string
	http      *http.Client
	resolvers map[string]func(context.Context) (string, error)
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

// WithTargetResolver registers a live target-image resolver for a
// control-plane component (e.g. "core", "webapp"). Check/Update call it
// right before each request and forward a successful result to the
// updater as an override for that component; the updater's own network
// is deliberately isolated (no outbound internet) so it cannot resolve
// this itself - Core does, and passes the result through. A resolver
// error is not fatal: the component just falls back to the updater's own
// static pin for that run, same as if no resolver were registered.
func (c *Client) WithTargetResolver(component string, resolve func(context.Context) (string, error)) {
	if c.resolvers == nil {
		c.resolvers = map[string]func(context.Context) (string, error){}
	}
	c.resolvers[component] = resolve
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	return c.do(ctx, http.MethodGet, "/api/control-plane/status", nil)
}

func (c *Client) Check(ctx context.Context) (Status, error) {
	return c.do(ctx, http.MethodPost, "/api/control-plane/check", c.resolveTargets(ctx))
}

func (c *Client) Update(ctx context.Context) (Status, error) {
	return c.do(ctx, http.MethodPost, "/api/control-plane/update", c.resolveTargets(ctx))
}

func (c *Client) resolveTargets(ctx context.Context) map[string]string {
	if len(c.resolvers) == 0 {
		return nil
	}
	targets := make(map[string]string, len(c.resolvers))
	for component, resolve := range c.resolvers {
		if image, err := resolve(ctx); err == nil && image != "" {
			targets[component] = image
		}
	}
	return targets
}

func (c *Client) do(ctx context.Context, method, path string, targetImages map[string]string) (Status, error) {
	var body io.Reader
	if len(targetImages) > 0 {
		encoded, err := json.Marshal(struct {
			TargetImages map[string]string `json:"target_images"`
		}{targetImages})
		if err != nil {
			return Status{}, err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return Status{}, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	response, err := c.http.Do(request)
	if err != nil {
		return Status{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var detail map[string]string
		_ = json.NewDecoder(response.Body).Decode(&detail)
		return Status{}, fmt.Errorf("control-plane updater returned %s: %s", response.Status, detail["error"])
	}
	var result Status
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Status{}, err
	}
	return result, nil
}
