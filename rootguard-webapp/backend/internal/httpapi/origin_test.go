package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireSameOriginWrites(t *testing.T) {
	handler := RequireSameOriginWrites(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, test := range []struct {
		name   string
		origin string
		want   int
	}{
		{name: "same origin", origin: "http://rootguard.local:8080", want: http.StatusNoContent},
		{name: "cross origin", origin: "https://attacker.example", want: http.StatusForbidden},
		{name: "non-browser client", want: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://rootguard.local:8080/adguard-ui/control/filtering", nil)
			request.Host = "rootguard.local:8080"
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Fatalf("got %d, want %d", recorder.Code, test.want)
			}
		})
	}
}
