package commands

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// tier3Server serves fixtures for invite/webhook/audit-log/event/sticker/
// poll/automod verbs against guild 111111111111111111.
func tier3Server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	gid := "111111111111111111"

	// invite
	mux.HandleFunc("GET /guilds/"+gid+"/invites", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"code": "abc123", "uses": 3, "max_uses": 10, "channel": map[string]any{"name": "general"}},
		})
	})
	mux.HandleFunc("POST /channels/222222222222222222/invites", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"code": "fresh1"})
	})
	mux.HandleFunc("DELETE /invites/abc123", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":"abc123"}`))
	})
	mux.HandleFunc("GET /invites/abc123", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"code": "abc123", "guild": map[string]any{"name": "Test Server"},
			"channel": map[string]any{"name": "general"},
		})
	})

	// webhook
	mux.HandleFunc("GET /guilds/"+gid+"/webhooks", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": "121212121212121212", "name": "notifier", "channel_id": "222222222222222222"},
		})
	})
	mux.HandleFunc("POST /channels/222222222222222222/webhooks", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"id": "121212121212121213", "token": "hook-token"})
	})
	mux.HandleFunc("DELETE /webhooks/121212121212121212", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /webhooks/121212121212121212/hook-token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// audit-log
	mux.HandleFunc("GET /guilds/"+gid+"/audit-logs", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"audit_log_entries": []map[string]any{
				{"id": "131313131313131313", "action_type": 12, "user_id": "444444444444444444",
					"target_id": "222222222222222223", "reason": "cleanup"},
			},
		})
	})

	// event
	mux.HandleFunc("GET /guilds/"+gid+"/scheduled-events", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": "141414141414141414", "name": "Meetup",
				"scheduled_start_time": "2026-08-01T11:00:00+00:00",
				"entity_metadata":      map[string]any{"location": "Taipei"}},
		})
	})
	mux.HandleFunc("POST /guilds/"+gid+"/scheduled-events", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(b, &body)
		if body["entity_type"] != float64(3) || body["scheduled_end_time"] == nil {
			t.Errorf("external event body = %v", body)
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "141414141414141415", "name": body["name"]})
	})
	mux.HandleFunc("PATCH /guilds/"+gid+"/scheduled-events/141414141414141414", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"141414141414141414"}`))
	})
	mux.HandleFunc("DELETE /guilds/"+gid+"/scheduled-events/141414141414141414", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// sticker
	mux.HandleFunc("GET /guilds/"+gid+"/stickers", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": "151515151515151515", "name": "wave", "description": "waving"},
		})
	})
	mux.HandleFunc("GET /guilds/"+gid+"/stickers/151515151515151515", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"id": "151515151515151515", "name": "wave"})
	})

	// poll
	mux.HandleFunc("POST /channels/222222222222222222/messages", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(b, &body)
		poll, ok := body["poll"].(map[string]any)
		if !ok {
			t.Errorf("missing poll in body: %v", body)
		} else if answers, ok := poll["answers"].([]any); !ok || len(answers) != 2 {
			t.Errorf("poll answers = %v", poll["answers"])
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "333333333333333339"})
	})
	mux.HandleFunc("GET /channels/222222222222222222/polls/333333333333333339/answers/1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"users": []map[string]any{
			{"id": "444444444444444444", "username": "alice", "global_name": "Alice"},
		}})
	})
	mux.HandleFunc("POST /channels/222222222222222222/polls/333333333333333339/expire", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"333333333333333339"}`))
	})

	// automod
	mux.HandleFunc("GET /guilds/"+gid+"/auto-moderation/rules", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": "161616161616161616", "name": "no-spoilers", "enabled": true},
		})
	})
	mux.HandleFunc("GET /guilds/"+gid+"/auto-moderation/rules/161616161616161616", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"id": "161616161616161616", "name": "no-spoilers"})
	})
	mux.HandleFunc("POST /guilds/"+gid+"/auto-moderation/rules", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), "keyword_filter") {
			t.Errorf("automod create body = %s", b)
		}
		json.NewEncoder(w).Encode(map[string]any{"id": "161616161616161617", "name": "blocklist"})
	})
	mux.HandleFunc("DELETE /guilds/"+gid+"/auto-moderation/rules/161616161616161616", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		http.NotFound(w, r)
	})
	return httptest.NewServer(mux)
}

func TestInviteVerbs(t *testing.T) {
	srv := tier3Server(t)
	defer srv.Close()
	gid := "--guild=111111111111111111"

	out, err := runCmd(t, srv, "invite", "list", gid)
	if err != nil || !strings.Contains(out, "abc123 — #general uses:3/10") {
		t.Fatalf("invite list: %v (%s)", err, out)
	}
	out, err = runCmd(t, srv, "invite", "create", "--channel", "222222222222222222")
	if err != nil || !strings.Contains(out, "created https://discord.gg/fresh1") {
		t.Fatalf("invite create: %v (%s)", err, out)
	}
	out, err = runCmd(t, srv, "invite", "delete", "abc123")
	if err != nil || !strings.Contains(out, "revoked abc123") {
		t.Fatalf("invite delete: %v (%s)", err, out)
	}
	out, err = runCmd(t, srv, "invite", "info", "abc123")
	if err != nil || !strings.Contains(out, "server:Test Server") {
		t.Fatalf("invite info: %v (%s)", err, out)
	}
}

func TestWebhookVerbs(t *testing.T) {
	srv := tier3Server(t)
	defer srv.Close()
	gid := "--guild=111111111111111111"

	out, err := runCmd(t, srv, "webhook", "list", gid)
	if err != nil || !strings.Contains(out, "notifier (121212121212121212)") {
		t.Fatalf("webhook list: %v (%s)", err, out)
	}
	out, err = runCmd(t, srv, "webhook", "create", "--channel", "222222222222222222", "--name", "alerts")
	if err != nil || !strings.Contains(out, "created webhook 121212121212121213") {
		t.Fatalf("webhook create: %v (%s)", err, out)
	}
	out, err = runCmd(t, srv, "webhook", "delete", "121212121212121212")
	if err != nil || !strings.Contains(out, "deleted webhook") {
		t.Fatalf("webhook delete: %v (%s)", err, out)
	}
	out, err = runCmd(t, srv, "webhook", "execute", "--id", "121212121212121212", "--token", "hook-token", "--text", "ping")
	if err != nil || !strings.Contains(out, "executed") {
		t.Fatalf("webhook execute: %v (%s)", err, out)
	}
	// A failing execute must not leak the token through the API error, nor
	// send the bot token to an endpoint that does not need it.
	var gotAuth string
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Unknown Webhook","code":10015}`))
	}))
	defer failing.Close()
	_, err = runCmd(t, failing, "webhook", "execute", "--id", "121212121212121212", "--token", "secret-hook-token", "--text", "x")
	if err == nil || strings.Contains(err.Error(), "secret-hook-token") || !strings.Contains(err.Error(), "<token>") {
		t.Fatalf("webhook execute error leaked token: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("webhook execute sent Authorization %q", gotAuth)
	}
	// So must a transport error, which quotes the request URL.
	failing.Close()
	_, err = runCmd(t, failing, "webhook", "execute", "--id", "121212121212121212", "--token", "secret-hook-token", "--text", "x")
	if err == nil || strings.Contains(err.Error(), "secret-hook-token") {
		t.Fatalf("webhook execute transport error leaked token: %v", err)
	}
	// Dry-run must not leak the token.
	out, err = runCmd(t, nil, "webhook", "execute", "--id", "121212121212121212", "--token", "secret-hook-token", "--text", "x", "--dry-run")
	if err != nil || strings.Contains(out, "secret-hook-token") {
		t.Fatalf("webhook execute dry-run leaked token: %v (%s)", err, out)
	}
}

func TestAuditLogRead(t *testing.T) {
	srv := tier3Server(t)
	defer srv.Close()
	out, err := runCmd(t, srv, "audit-log", "read", "--guild", "111111111111111111")
	if err != nil || !strings.Contains(out, "channel-delete by 444444444444444444 on 222222222222222223 — cleanup") {
		t.Fatalf("audit-log read: %v (%s)", err, out)
	}
}

func TestEventVerbs(t *testing.T) {
	srv := tier3Server(t)
	defer srv.Close()
	gid := "--guild=111111111111111111"

	out, err := runCmd(t, srv, "event", "list", gid)
	if err != nil || !strings.Contains(out, "Meetup (141414141414141414)") || !strings.Contains(out, "@ Taipei") {
		t.Fatalf("event list: %v (%s)", err, out)
	}
	out, err = runCmd(t, srv, "event", "create", gid, "--name", "Meetup2",
		"--start", "2026-09-01T11:00:00+00:00", "--end", "2026-09-01T13:00:00+00:00", "--location", "Taipei")
	if err != nil || !strings.Contains(out, "created event Meetup2") {
		t.Fatalf("event create: %v (%s)", err, out)
	}
	if _, err := runCmd(t, srv, "event", "create", gid, "--name", "x", "--start", "2026-09-01T11:00:00+00:00"); err == nil {
		t.Fatal("event create without location/channel must error")
	}
	out, err = runCmd(t, srv, "event", "edit", gid, "--id", "141414141414141414", "--name", "Renamed")
	if err != nil || !strings.Contains(out, "edited event") {
		t.Fatalf("event edit: %v (%s)", err, out)
	}
	out, err = runCmd(t, srv, "event", "delete", gid, "--id", "141414141414141414")
	if err != nil || !strings.Contains(out, "deleted event") {
		t.Fatalf("event delete: %v (%s)", err, out)
	}
}

func TestStickerVerbs(t *testing.T) {
	srv := tier3Server(t)
	defer srv.Close()
	gid := "--guild=111111111111111111"

	out, err := runCmd(t, srv, "sticker", "list", gid)
	if err != nil || !strings.Contains(out, "wave (151515151515151515) — waving") {
		t.Fatalf("sticker list: %v (%s)", err, out)
	}
	out, err = runCmd(t, srv, "sticker", "info", gid, "--id", "151515151515151515")
	if err != nil || !strings.Contains(out, "wave") {
		t.Fatalf("sticker info: %v (%s)", err, out)
	}
}

func TestPollVerbs(t *testing.T) {
	srv := tier3Server(t)
	defer srv.Close()

	out, err := runCmd(t, srv, "poll", "create", "--channel", "222222222222222222",
		"--question", "Lunch?", "--answer", "Ramen", "--answer", "Curry")
	if err != nil || !strings.Contains(out, "poll created 333333333333333339") {
		t.Fatalf("poll create: %v (%s)", err, out)
	}
	if _, err := runCmd(t, srv, "poll", "create", "--channel", "222222222222222222",
		"--question", "One?", "--answer", "only"); err == nil {
		t.Fatal("poll with one answer must error")
	}
	out, err = runCmd(t, srv, "poll", "results", "--channel", "222222222222222222",
		"--message", "333333333333333339", "--answer", "1")
	if err != nil || !strings.Contains(out, "Alice (444444444444444444)") {
		t.Fatalf("poll results: %v (%s)", err, out)
	}
	out, err = runCmd(t, srv, "poll", "end", "--channel", "222222222222222222", "--message", "333333333333333339")
	if err != nil || !strings.Contains(out, "poll ended") {
		t.Fatalf("poll end: %v (%s)", err, out)
	}
}

func TestAutomodVerbs(t *testing.T) {
	srv := tier3Server(t)
	defer srv.Close()
	gid := "--guild=111111111111111111"

	out, err := runCmd(t, srv, "automod", "list", gid)
	if err != nil || !strings.Contains(out, "no-spoilers (161616161616161616) — enabled") {
		t.Fatalf("automod list: %v (%s)", err, out)
	}
	out, err = runCmd(t, srv, "automod", "info", gid, "--id", "161616161616161616")
	if err != nil || !strings.Contains(out, "no-spoilers") {
		t.Fatalf("automod info: %v (%s)", err, out)
	}
	out, err = runCmd(t, srv, "automod", "create", gid, "--name", "blocklist", "--keyword", "spoiler*")
	if err != nil || !strings.Contains(out, "created rule blocklist") {
		t.Fatalf("automod create: %v (%s)", err, out)
	}
	if _, err := runCmd(t, srv, "automod", "create", gid, "--name", "empty"); err == nil {
		t.Fatal("automod create without keywords must error")
	}
	out, err = runCmd(t, srv, "automod", "delete", gid, "--id", "161616161616161616")
	if err != nil || !strings.Contains(out, "deleted rule") {
		t.Fatalf("automod delete: %v (%s)", err, out)
	}
}
