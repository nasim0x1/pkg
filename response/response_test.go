package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponseHelpers(t *testing.T) {
	// Success
	rec := httptest.NewRecorder()
	Success(rec, "operation successful", map[string]string{"key": "value"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var res Response
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !res.Success || res.Message != "operation successful" {
		t.Errorf("unexpected response body: %+v", res)
	}

	// BadRequest
	rec = httptest.NewRecorder()
	BadRequest(rec, "missing required field")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	// Paginated
	rec = httptest.NewRecorder()
	Paginated(rec, "users list", []string{"a", "b"}, &Meta{Page: 1, PerPage: 10, TotalItems: 2, TotalPages: 1})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}
