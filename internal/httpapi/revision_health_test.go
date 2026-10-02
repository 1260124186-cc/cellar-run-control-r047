package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/1260124186-cc/solo-0016-cellar-run-control/internal/service"
	"github.com/1260124186-cc/solo-0016-cellar-run-control/internal/storage"
)

func TestHealthRevisionChangesOnlyForSuccessfulWrite(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("storage.Open() error = %v", err)
	}
	app := service.New(store, nil)
	handler := NewHandler(app)

	postVessel := func(body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/v1/vessels", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	healthRevision := func() int64 {
		request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("health status = %d, want %d", response.Code, http.StatusOK)
		}
		var body struct {
			Status   string `json:"status"`
			Revision int64  `json:"revision"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode health response: %v", err)
		}
		return body.Revision
	}

	if revision := healthRevision(); revision != 0 {
		t.Fatalf("initial health revision = %d, want 0", revision)
	}

	invalid := postVessel(`{"code":"BAD","capacity_liters":1}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid write status = %d, want %d", invalid.Code, http.StatusBadRequest)
	}
	if revision := healthRevision(); revision != 0 {
		t.Fatalf("health revision after invalid write = %d, want 0", revision)
	}

	first := postVessel(`{"code":"VESSEL-1","capacity_liters":100}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("first successful write status = %d, body = %s", first.Code, first.Body.String())
	}
	if revision := healthRevision(); revision != 1 {
		t.Fatalf("health revision after success = %d, want 1", revision)
	}

	duplicate := postVessel(`{"code":"VESSEL-1","capacity_liters":100}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate write status = %d, want %d", duplicate.Code, http.StatusConflict)
	}
	if revision := healthRevision(); revision != 1 {
		t.Fatalf("health revision after duplicate write = %d, want 1", revision)
	}

	second := postVessel(`{"code":"VESSEL-2","capacity_liters":100}`)
	if second.Code != http.StatusCreated {
		t.Fatalf("second successful write status = %d, body = %s", second.Code, second.Body.String())
	}
	if revision := healthRevision(); revision != 2 {
		t.Fatalf("health revision after second success = %d, want 2", revision)
	}
}
