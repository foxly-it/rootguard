package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
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

type Status struct {
	State     string          `json:"state"`
	Message   string          `json:"message"`
	Services  []ServiceStatus `json:"services"`
	UpdatedAt time.Time       `json:"updated_at"`
}

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	return c.do(ctx, http.MethodGet, "/api/control-plane/status")
}

func (c *Client) Check(ctx context.Context) (Status, error) {
	return c.do(ctx, http.MethodPost, "/api/control-plane/check")
}

func (c *Client) Update(ctx context.Context) (Status, error) {
	return c.do(ctx, http.MethodPost, "/api/control-plane/update")
}

func (c *Client) do(ctx context.Context, method, path string) (Status, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return Status{}, err
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
