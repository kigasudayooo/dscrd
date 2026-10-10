package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func newTestClient(srv *httptest.Server) *Client {
	c := New("tok")
	c.BaseURL = srv.URL
	return c
}

func TestDoSuccessPassthrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bot tok" {
			t.Errorf("Authorization = %q, want %q", got, "Bot tok")
		}
		if r.Method != "GET" || r.URL.Path != "/channels/222222222222222222" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"id":"222222222222222222","name":"general"}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).Do("GET", "/channels/222222222222222222", nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if !strings.Contains(string(raw), "general") {
		t.Fatalf("raw = %s", raw)
	}
}

func TestDoJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		b, _ := io.ReadAll(r.Body)
		var body map[string]string
		if err := json.Unmarshal(b, &body); err != nil || body["content"] != "hello" {
			t.Errorf("body = %s", b)
		}
		w.Write([]byte(`{"id":"333333333333333333"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).Do("POST", "/channels/2/messages", map[string]string{"content": "hello"})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
}

func TestDoNoContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).Do("DELETE", "/channels/2/messages/3", nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("raw = %q, want empty", raw)
	}
}

func TestDoErrorMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"code":10003,"message":"Unknown Channel"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).Do("GET", "/channels/9", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Status != 404 || apiErr.Code != 10003 || apiErr.Message != "Unknown Channel" {
		t.Fatalf("apiErr = %+v", apiErr)
	}
	if apiErr.ExitCode() != 4 {
		t.Fatalf("ExitCode = %d, want 4", apiErr.ExitCode())
	}
}

func TestDoErrorHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"code":50001,"message":"Missing Access"}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).Do("GET", "/guilds/1/channels", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.Hint == "" || !strings.Contains(apiErr.Error(), "hint:") {
		t.Fatalf("expected hint in %q", apiErr.Error())
	}
}

func TestExitCodes(t *testing.T) {
	cases := []struct {
		e    APIError
		want int
	}{
		{APIError{Status: 401}, 3},
		{APIError{Status: 404}, 4},
		{APIError{Status: 403, Code: 10008}, 4},
		{APIError{Status: 429}, 5},
		{APIError{Status: 403, Code: 50013}, 1},
		{APIError{Status: 500}, 1},
	}
	for _, c := range cases {
		if got := c.e.ExitCode(); got != c.want {
			t.Errorf("ExitCode(%+v) = %d, want %d", c.e, got, c.want)
		}
	}
}

func TestDoRateLimitRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"retry_after":0.01,"message":"You are being rate limited."}`))
			return
		}
		w.Write([]byte(`{"id":"1"}`))
	}))
	defer srv.Close()

	if _, err := newTestClient(srv).Do("GET", "/users/@me", nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestDoRateLimitExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"retry_after":0.001}`))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	c.MaxRetries = 2
	_, err := c.Do("GET", "/users/@me", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.ExitCode() != 5 {
		t.Fatalf("err = %v, want rate-limit APIError (exit 5)", err)
	}
}

func TestDoSearchIndexWarmupRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusAccepted)
			w.Write([]byte(`{"code":110000,"retry_after":0.01,"documents_indexed":0}`))
			return
		}
		w.Write([]byte(`{"total_results":0,"messages":[]}`))
	}))
	defer srv.Close()

	raw, err := newTestClient(srv).Do("GET", "/guilds/1/messages/search?content=x", nil)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if attempts != 2 || !strings.Contains(string(raw), "total_results") {
		t.Fatalf("attempts = %d raw = %s", attempts, raw)
	}
}

func TestDoMultipart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mt != "multipart/form-data" {
			t.Fatalf("Content-Type = %q (%v)", r.Header.Get("Content-Type"), err)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		form, err := mr.ReadForm(1 << 20)
		if err != nil {
			t.Fatalf("ReadForm: %v", err)
		}
		if got := form.Value["payload_json"]; len(got) != 1 || !strings.Contains(got[0], "hello") {
			t.Errorf("payload_json = %v", got)
		}
		if got := form.File["files[0]"]; len(got) != 1 || got[0].Filename != "note.txt" {
			t.Errorf("files[0] = %v", got)
		}
		w.Write([]byte(`{"id":"333333333333333333"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := dir + "/note.txt"
	if err := writeFile(path, "attachment body"); err != nil {
		t.Fatal(err)
	}
	_, err := newTestClient(srv).DoMultipart("POST", "/channels/2/messages", []byte(`{"content":"hello"}`), path)
	if err != nil {
		t.Fatalf("DoMultipart: %v", err)
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func TestDoRejectsRetargetingPaths(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
	}))
	defer srv.Close()

	for _, path := range []string{
		"/channels/1/messages/../../../guilds/2",
		"/channels/1/messages/%2e%2e/x",
		"/channels/1/./messages",
		"/channels/1/messages/2#frag",
		"/channels/1/messages/2\r\nX-Injected: 1",
	} {
		if _, err := newTestClient(srv).Do("DELETE", path, nil); err == nil {
			t.Errorf("Do(%q) succeeded, want refusal", path)
		}
	}
	if hit {
		t.Fatal("a refused path reached the server")
	}
	// Dots inside a segment and in the query are legitimate.
	if _, err := newTestClient(srv).Do("GET", "/users/@me?after=1.5&x=..", nil); err != nil {
		t.Fatalf("legitimate path refused: %v", err)
	}
}

func TestDoRateLimitWaitIsCapped(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"retry_after":86400}`))
	}))
	defer srv.Close()

	_, err := newTestClient(srv).Do("GET", "/users/@me", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.ExitCode() != 5 {
		t.Fatalf("err = %v, want rate-limit APIError (exit 5)", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (no sleep on an over-long retry_after)", attempts)
	}
}
