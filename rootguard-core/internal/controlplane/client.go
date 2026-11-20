package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
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

	// lookupNotice explains why the last check or update compared against
	// the installed pins only. It is kept until the next lookup succeeds, so
	// the status the WebGUI polls after a check carries it too.
	mu           sync.Mutex
	lookupNotice string
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
// error is not fatal, but it drops every override for that run, not just
// the failed one: Core and WebApp then both stay on the updater's own
// static pins, same as if no resolver were registered.
func (c *Client) WithTargetResolver(component string, resolve func(context.Context) (string, error)) {
	if c.resolvers == nil {
		c.resolvers = map[string]func(context.Context) (string, error){}
	}
	c.resolvers[component] = resolve
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	status, err := c.do(ctx, http.MethodGet, "/api/control-plane/status", nil)
	return c.withLookupNotice(status), err
}

func (c *Client) Check(ctx context.Context) (Status, error) {
	targets := c.resolveTargets(ctx)
	status, err := c.do(ctx, http.MethodPost, "/api/control-plane/check", targets)
	return c.withLookupNotice(status), err
}

func (c *Client) Update(ctx context.Context) (Status, error) {
	targets := c.resolveTargets(ctx)
	status, err := c.do(ctx, http.MethodPost, "/api/control-plane/update", targets)
	return c.withLookupNotice(status), err
}

// withLookupNotice appends why the newest release is unknown to the
// updater's own message. Found in the upgrade journey of 2026-11-20: with
// GitHub's rate limit reached, Core 1.0.15 logged the reason, but the WebGUI
// only said "Control-Plane-Prüfung abgeschlossen." and showed no update,
// which reads as "up to date" although 1.0.16 was out. The data-plane check
// already says so (updater.Manager's check message); this is the same text.
func (c *Client) withLookupNotice(status Status) Status {
	c.mu.Lock()
	notice := c.lookupNotice
	c.mu.Unlock()
	if notice != "" && !strings.Contains(status.Message, notice) {
		status.Message = strings.TrimSpace(status.Message + " " + notice)
	}
	return status
}

func (c *Client) setLookupNotice(err error) {
	notice := ""
	if err != nil {
		notice = "Die neueste RootGuard-Version ließ sich nicht bei GitHub abfragen. " +
			"Verglichen wurde nur mit den installierten Versionen (" + err.Error() + ")."
	}
	c.mu.Lock()
	c.lookupNotice = notice
	c.mu.Unlock()
}

func (c *Client) resolveTargets(ctx context.Context) map[string]string {
	if len(c.resolvers) == 0 {
		return nil
	}
	// All or nothing (found live 2026-11-13): passing on only the overrides
	// that resolved moved Core to the new release and left WebApp on the
	// installed one, and the updater reported that as a successful update.
	targets := make(map[string]string, len(c.resolvers))
	for component, resolve := range c.resolvers {
		image, err := resolve(ctx)
		if err == nil && image == "" {
			err = fmt.Errorf("no image reference")
		}
		if err != nil {
			log.Printf("control-plane update target for %s unavailable, keeping the updater's own pins for every component: %v", component, err)
			c.setLookupNotice(err)
			return nil
		}
		targets[component] = image
	}
	c.setLookupNotice(nil)
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
