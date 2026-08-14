package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/rogeecn/id-ledger/internal/database"
)

func TestHealthAndBearerAuth(t *testing.T) {
	server := newTestServer(t, time.Unix(1_700_000_000, 0))

	response := doRequest(t, server.App(), http.MethodGet, "/healthz", nil, "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", response.StatusCode)
	}
	response.Body.Close()

	response = doRequest(t, server.App(), http.MethodPost, "/v1/projects/demo/ids", []byte(`{"ids":["one"]}`), "")
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d", response.StatusCode)
	}
	response.Body.Close()
}

func TestBatchWriteIsIdempotentAndServerTimestamped(t *testing.T) {
	createdAt := time.Date(2026, 8, 14, 14, 0, 0, 0, time.UTC)
	server := newTestServer(t, createdAt)

	response := doRequest(t, server.App(), http.MethodPost, "/v1/projects/demo/ids", []byte(`{"ids":["A","a","A"]}`), "secret")
	var first writeResponse
	decodeResponse(t, response, http.StatusOK, &first)
	if first.Received != 3 || first.Inserted != 2 {
		t.Fatalf("unexpected first result: %#v", first)
	}

	server.now = func() time.Time { return createdAt.Add(time.Hour) }
	response = doRequest(t, server.App(), http.MethodPost, "/v1/projects/demo/ids", []byte(`{"ids":["A","a"]}`), "secret")
	var second writeResponse
	decodeResponse(t, response, http.StatusOK, &second)
	if second.Inserted != 0 {
		t.Fatalf("duplicate insert count = %d", second.Inserted)
	}

	path := fmt.Sprintf("/v1/projects/demo/ids?since=%s&until=%s", url.QueryEscape(createdAt.Add(-time.Second).Format(time.RFC3339)), url.QueryEscape(createdAt.Add(time.Second).Format(time.RFC3339)))
	response = doRequest(t, server.App(), http.MethodGet, path, nil, "secret")
	var listed listResponse
	decodeResponse(t, response, http.StatusOK, &listed)
	if len(listed.Items) != 2 || listed.Items[0].CreatedAt != createdAt.Format(time.RFC3339) || listed.Items[1].CreatedAt != createdAt.Format(time.RFC3339) {
		t.Fatalf("unexpected records: %#v", listed.Items)
	}

	response = doRequest(t, server.App(), http.MethodPost, "/v1/projects/demo/ids", []byte(`{"ids":["x"],"created_at":"2020-01-01T00:00:00Z"}`), "secret")
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("client-created timestamp status = %d", response.StatusCode)
	}
}

func TestTimeCursorPagination(t *testing.T) {
	createdAt := time.Date(2026, 8, 14, 14, 0, 0, 0, time.UTC)
	server := newTestServer(t, createdAt)

	response := doRequest(t, server.App(), http.MethodPost, "/v1/projects/demo/ids", []byte(`{"ids":["a","b","c"]}`), "secret")
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("write status = %d", response.StatusCode)
	}

	path := fmt.Sprintf("/v1/projects/demo/ids?since=%s&until=%s&limit=2", url.QueryEscape(createdAt.Add(-time.Second).Format(time.RFC3339)), url.QueryEscape(createdAt.Add(time.Second).Format(time.RFC3339)))
	response = doRequest(t, server.App(), http.MethodGet, path, nil, "secret")
	var first listResponse
	decodeResponse(t, response, http.StatusOK, &first)
	if len(first.Items) != 2 || first.Items[0].ID != "c" || first.Items[1].ID != "b" || first.NextCursor == "" {
		t.Fatalf("unexpected first page: %#v", first)
	}

	response = doRequest(t, server.App(), http.MethodGet, "/v1/projects/demo/ids?limit=2&cursor="+url.QueryEscape(first.NextCursor), nil, "secret")
	var second listResponse
	decodeResponse(t, response, http.StatusOK, &second)
	if len(second.Items) != 1 || second.Items[0].ID != "a" || second.NextCursor != "" {
		t.Fatalf("unexpected second page: %#v", second)
	}
}

func newTestServer(t *testing.T, now time.Time) *Server {
	t.Helper()
	connection, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.Close() })
	server := New(connection, "secret")
	server.now = func() time.Time { return now }
	return server
}

func doRequest(t *testing.T, app *fiber.App, method, target string, body []byte, token string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func decodeResponse(t *testing.T, response *http.Response, status int, value any) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != status {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("status = %d, body = %s", response.StatusCode, body)
	}
	if err := json.NewDecoder(response.Body).Decode(value); err != nil {
		t.Fatal(err)
	}
}
