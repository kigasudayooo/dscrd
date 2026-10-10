package commands

import (
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// msgServer serves message fixtures for the msg verb tests.
func msgServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /channels/222222222222222222/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") == "" {
			t.Error("missing limit param")
		}
		// Newest first, as Discord returns them.
		json.NewEncoder(w).Encode([]map[string]any{
			{
				"id": "333333333333333334", "content": "second",
				"timestamp": "2026-07-03T04:06:00.000000+00:00", "channel_id": "222222222222222222",
				"author": map[string]any{"username": "bob", "global_name": "Bob"},
			},
			{
				"id": "333333333333333333", "content": "first",
				"timestamp": "2026-07-03T04:05:00.000000+00:00", "channel_id": "222222222222222222",
				"author": map[string]any{"username": "alice", "global_name": "Alice"},
				"attachments": []map[string]any{
					{"filename": "report.txt", "url": "https://cdn.discordapp.com/attachments/1/2/report.txt"},
				},
			},
		})
	})
	mux.HandleFunc("POST /channels/222222222222222222/messages", func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "multipart/form-data") {
			_, params, _ := mime.ParseMediaType(ct)
			form, err := multipart.NewReader(r.Body, params["boundary"]).ReadForm(1 << 20)
			if err != nil {
				t.Errorf("ReadForm: %v", err)
			}
			w.Header().Set("X-Multipart", "yes")
			if v := form.Value["payload_json"]; len(v) != 1 || !strings.Contains(v[0], "with file") {
				t.Errorf("payload_json = %v", v)
			}
			if f := form.File["files[0]"]; len(f) != 1 {
				t.Errorf("files = %v", f)
			}
			json.NewEncoder(w).Encode(map[string]string{"id": "333333333333333336"})
			return
		}
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(b, &body)
		if ref, ok := body["message_reference"].(map[string]any); ok {
			if ref["message_id"] != "333333333333333333" {
				t.Errorf("reply reference = %v", ref)
			}
		}
		json.NewEncoder(w).Encode(map[string]string{"id": "333333333333333335"})
	})
	mux.HandleFunc("PATCH /channels/222222222222222222/messages/333333333333333333", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":"333333333333333333"}`))
	})
	mux.HandleFunc("DELETE /channels/222222222222222222/messages/333333333333333333", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("PUT /channels/222222222222222222/messages/333333333333333333/reactions/{emoji}/@me", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("emoji") != "👍" {
			t.Errorf("emoji path = %q", r.PathValue("emoji"))
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /channels/222222222222222222/messages/333333333333333333/reactions/{emoji}/@me", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /channels/222222222222222222/messages/333333333333333333/reactions/{emoji}", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]string{
			{"id": "444444444444444444", "username": "alice", "global_name": "Alice"},
		})
	})
	mux.HandleFunc("PUT /channels/222222222222222222/pins/333333333333333333", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /channels/222222222222222222/pins/333333333333333333", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /channels/222222222222222222/pins", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{
				"id": "333333333333333333", "content": "pinned note",
				"timestamp": "2026-07-01T00:00:00+00:00",
				"author":    map[string]any{"username": "alice", "global_name": "Alice"},
				"embeds": []map[string]any{
					{"title": "Status", "description": "all systems\n nominal"},
				},
			},
		})
	})
	mux.HandleFunc("GET /users/@me/guilds", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{{"id": "111111111111111111", "name": "Test Server"}})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		http.NotFound(w, r)
	})
	return httptest.NewServer(mux)
}

func TestMsgRead(t *testing.T) {
	srv := msgServer(t)
	defer srv.Close()
	out, err := runCmd(t, srv, "msg", "read", "--channel", "222222222222222222", "--no-resolve")
	if err != nil {
		t.Fatalf("msg read: %v (%s)", err, out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %v", lines)
	}
	// Oldest first, attachment rendered, ID present.
	if !strings.HasPrefix(lines[0], "Alice: first [file:report.txt") || !strings.Contains(lines[0], "(333333333333333333)") {
		t.Errorf("line0 = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "Bob: second") {
		t.Errorf("line1 = %q", lines[1])
	}
}

func TestMsgSend(t *testing.T) {
	srv := msgServer(t)
	defer srv.Close()
	out, err := runCmd(t, srv, "msg", "send", "--channel", "222222222222222222", "--text", "hello")
	if err != nil || strings.TrimSpace(out) != "sent 333333333333333335" {
		t.Fatalf("send: %v (%s)", err, out)
	}
}

func TestMsgSendReply(t *testing.T) {
	srv := msgServer(t)
	defer srv.Close()
	out, err := runCmd(t, srv, "msg", "send", "--channel", "222222222222222222",
		"--text", "re", "--reply-to", "333333333333333333")
	if err != nil || !strings.Contains(out, "sent") {
		t.Fatalf("reply: %v (%s)", err, out)
	}
}

func TestMsgSendFile(t *testing.T) {
	srv := msgServer(t)
	defer srv.Close()
	path := t.TempDir() + "/attach.txt"
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runCmd(t, srv, "msg", "send", "--channel", "222222222222222222",
		"--text", "with file", "--file", path)
	if err != nil || strings.TrimSpace(out) != "sent 333333333333333336" {
		t.Fatalf("send file: %v (%s)", err, out)
	}
}

func TestMsgSendRequiresContent(t *testing.T) {
	if _, err := runCmd(t, nil, "msg", "send", "--channel", "222222222222222222"); err == nil {
		t.Fatal("send without text/file must error")
	}
}

func TestMsgSendDryRun(t *testing.T) {
	out, err := runCmd(t, nil, "msg", "send", "--channel", "222222222222222222", "--text", "hi", "--dry-run")
	if err != nil || !strings.Contains(out, "[dry-run] POST /channels/222222222222222222/messages") {
		t.Fatalf("dry-run: %v (%s)", err, out)
	}
}

func TestMsgEdit(t *testing.T) {
	srv := msgServer(t)
	defer srv.Close()
	out, err := runCmd(t, srv, "msg", "edit", "--channel", "222222222222222222",
		"--message", "333333333333333333", "--text", "fixed")
	if err != nil || !strings.Contains(out, "edited 333333333333333333") {
		t.Fatalf("edit: %v (%s)", err, out)
	}
}

func TestMsgDelete(t *testing.T) {
	srv := msgServer(t)
	defer srv.Close()
	out, err := runCmd(t, srv, "msg", "delete", "--channel", "222222222222222222", "--message", "333333333333333333")
	if err != nil || !strings.Contains(out, "deleted 333333333333333333") {
		t.Fatalf("delete: %v (%s)", err, out)
	}
}

func TestMsgReactUnicodeEscaping(t *testing.T) {
	srv := msgServer(t)
	defer srv.Close()
	out, err := runCmd(t, srv, "msg", "react", "--channel", "222222222222222222",
		"--message", "333333333333333333", "--emoji", "👍")
	if err != nil || !strings.Contains(out, "reacted") {
		t.Fatalf("react: %v (%s)", err, out)
	}
}

func TestMsgUnreact(t *testing.T) {
	srv := msgServer(t)
	defer srv.Close()
	out, err := runCmd(t, srv, "msg", "unreact", "--channel", "222222222222222222",
		"--message", "333333333333333333", "--emoji", "👍")
	if err != nil || !strings.Contains(out, "unreacted") {
		t.Fatalf("unreact: %v (%s)", err, out)
	}
}

func TestMsgReactions(t *testing.T) {
	srv := msgServer(t)
	defer srv.Close()
	out, err := runCmd(t, srv, "msg", "reactions", "--channel", "222222222222222222",
		"--message", "333333333333333333", "--emoji", "👍")
	if err != nil || strings.TrimSpace(out) != "Alice (444444444444444444)" {
		t.Fatalf("reactions: %v (%s)", err, out)
	}
}

func TestMsgPinUnpinPins(t *testing.T) {
	srv := msgServer(t)
	defer srv.Close()
	if out, err := runCmd(t, srv, "msg", "pin", "--channel", "222222222222222222", "--message", "333333333333333333"); err != nil || !strings.Contains(out, "pinned") {
		t.Fatalf("pin: %v (%s)", err, out)
	}
	if out, err := runCmd(t, srv, "msg", "unpin", "--channel", "222222222222222222", "--message", "333333333333333333"); err != nil || !strings.Contains(out, "unpinned") {
		t.Fatalf("unpin: %v (%s)", err, out)
	}
	out, err := runCmd(t, srv, "msg", "pins", "--channel", "222222222222222222", "--no-resolve")
	if err != nil || !strings.Contains(out, "Alice: pinned note [embed: Status — all systems nominal]") {
		t.Fatalf("pins: %v (%s)", err, out)
	}
}

func TestMsgPermalink(t *testing.T) {
	srv := msgServer(t)
	defer srv.Close()
	out, err := runCmd(t, srv, "msg", "permalink", "--guild", "111111111111111111",
		"--channel", "222222222222222222", "--message", "333333333333333333")
	if err != nil {
		t.Fatalf("permalink: %v (%s)", err, out)
	}
	want := "https://discord.com/channels/111111111111111111/222222222222222222/333333333333333333"
	if strings.TrimSpace(out) != want {
		t.Fatalf("out = %q", out)
	}
}

func TestPrettifyMentionsPattern(t *testing.T) {
	// Regex behavior only; resolver plumbing is covered via the resolve package.
	got := mentionPattern.FindAllString("<@444444444444444444> <@!444444444444444444> <#222222222222222222> <@&555555555555555555>", -1)
	if len(got) != 3 {
		t.Fatalf("matches = %v (role mentions must not match)", got)
	}
}

// Text relayed by an agent is untrusted: every posting verb must suppress
// mention pings unless --allow-mentions is passed.
func TestPostingVerbsSuppressMentionsByDefault(t *testing.T) {
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Write([]byte(`{"id":"333333333333333333","name":"t"}`))
	}))
	defer srv.Close()

	const suppressed = `"allowed_mentions":{"parse":[]}`
	ch, id := "222222222222222222", "333333333333333333"
	verbs := [][]string{
		{"msg", "send", "--channel", ch, "--text", "@everyone hi"},
		{"dm", "send", "--user", "444444444444444444", "--text", "@everyone hi"},
		{"thread", "reply", "--thread", id, "--text", "@everyone hi"},
		{"thread", "create", "--channel", ch, "--name", "t", "--text", "@everyone hi"},
		{"webhook", "execute", "--id", id, "--token", "hook-token", "--text", "@everyone hi"},
	}
	for _, args := range verbs {
		if _, err := runCmd(t, srv, args...); err != nil || !strings.Contains(body, suppressed) {
			t.Errorf("%v: mentions not suppressed: %v (%s)", args, err, body)
		}
		if _, err := runCmd(t, srv, append(args, "--allow-mentions")...); err != nil || strings.Contains(body, "allowed_mentions") {
			t.Errorf("%v --allow-mentions: still suppressed: %v (%s)", args, err, body)
		}
	}
}
