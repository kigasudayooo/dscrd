package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGlobalFlagsRegistered(t *testing.T) {
	root := NewRootCommand("test")
	for _, name := range []string{"format", "profile", "guild", "raw", "dry-run", "no-resolve"} {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("missing global flag --%s", name)
		}
	}
}

func TestVersionFlag(t *testing.T) {
	out, err := runCmd(t, nil, "--version")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.TrimSpace(out) != "dscrd test" {
		t.Fatalf("version output = %q, want %q", strings.TrimSpace(out), "dscrd test")
	}
}

// guildListServer serves GET /users/@me/guilds with two fixture guilds.
func guildListServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/@me/guilds":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": "111111111111111111", "name": "Test Server", "owner": true},
				{"id": "111111111111111112", "name": "Other Server", "owner": false},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestGuildIDSnowflakePassthrough(t *testing.T) {
	g := &GlobalFlags{Guild: "111111111111111111"}
	gid, err := guildID(g, nil) // nil client: snowflake must not hit the API
	if err != nil || gid != "111111111111111111" {
		t.Fatalf("guildID = %q, %v", gid, err)
	}
}

func TestGuildIDNameResolution(t *testing.T) {
	srv := guildListServer(t)
	defer srv.Close()
	t.Setenv("DSCRD_CONFIG", t.TempDir()+"/config.toml")

	g := &GlobalFlags{Guild: "test server"} // case-insensitive
	c := testAPIClient(srv)
	gid, err := guildID(g, c)
	if err != nil || gid != "111111111111111111" {
		t.Fatalf("guildID = %q, %v", gid, err)
	}
}

func TestGuildIDNotFound(t *testing.T) {
	srv := guildListServer(t)
	defer srv.Close()
	t.Setenv("DSCRD_CONFIG", t.TempDir()+"/config.toml")

	g := &GlobalFlags{Guild: "Nope"}
	if _, err := guildID(g, testAPIClient(srv)); err == nil {
		t.Fatal("unknown guild name must error")
	}
}

func TestGuildIDUnset(t *testing.T) {
	t.Setenv("DSCRD_CONFIG", t.TempDir()+"/config.toml")
	g := &GlobalFlags{}
	if _, err := guildID(g, nil); err == nil {
		t.Fatal("missing guild must error with guidance")
	}
}

// ID flags are spliced into API paths; a value that is not a snowflake must be
// refused before any request (or dry-run preview) is built.
func TestIDFlagsRejectPathInjection(t *testing.T) {
	ch := "222222222222222222"
	cases := [][]string{
		{"msg", "delete", "--channel", ch, "--message", "../../../guilds/111111111111111111"},
		{"msg", "delete", "--channel", ch, "--message", "333333333333333333?x=1", "--dry-run"},
		{"msg", "read", "--channel", ch, "--before", "abc"},
		{"role", "assign", "--guild", "111111111111111111", "--user", "444444444444444444/..", "--role", "555555555555555555"},
		{"thread", "read", "--thread", "../guilds"},
		{"webhook", "delete", "../guilds/111111111111111111"},
		{"invite", "delete", "abc/../../guilds/111111111111111111"},
		{"invite", "info", "https://example.invalid/abc"},
		{"poll", "results", "--channel", ch, "--message", "333333333333333333", "--answer", "1/../.."},
	}
	for _, args := range cases {
		hit := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hit = true
			w.Write([]byte(`{}`))
		}))
		out, err := runCmd(t, srv, args...)
		srv.Close()
		if err == nil {
			t.Errorf("%v: accepted (%s)", args, out)
		}
		if hit {
			t.Errorf("%v: reached the API", args)
		}
	}
}
