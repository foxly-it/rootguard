package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type decodeJSONTestPayload struct {
	Name string `json:"name"`
}

func TestDecodeJSONDecodesAValidBody(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"rootguard"}`))
	value, err := decodeJSON[decodeJSONTestPayload](response, request, 1<<10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value.Name != "rootguard" {
		t.Fatalf("expected name %q, got %q", "rootguard", value.Name)
	}
}

func TestDecodeJSONRejectsUnknownFields(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"rootguard","surprise":true}`))
	if _, err := decodeJSON[decodeJSONTestPayload](response, request, 1<<10); err == nil {
		t.Fatal("expected an unknown-field error, got nil")
	}
}

func TestDecodeJSONRejectsTrailingData(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"rootguard"}{"name":"ignored-second-document"}`))
	if _, err := decodeJSON[decodeJSONTestPayload](response, request, 1<<10); err == nil {
		t.Fatal("expected a trailing-data error, got nil")
	}
}

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"`+strings.Repeat("a", 1024)+`"}`))
	if _, err := decodeJSON[decodeJSONTestPayload](response, request, 16); err == nil {
		t.Fatal("expected a body-too-large error, got nil")
	}
}
