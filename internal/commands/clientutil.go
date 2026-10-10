package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/howar31/dscrd/internal/api"
	"github.com/howar31/dscrd/internal/auth"
	"github.com/howar31/dscrd/internal/resolve"
	"github.com/spf13/cobra"
)

// profileName returns the effective profile: the --profile flag if set,
// otherwise the DSCRD_PROFILE env var.
func profileName(g *GlobalFlags) string {
	if g.Profile != "" {
		return g.Profile
	}
	return os.Getenv("DSCRD_PROFILE")
}

// buildClient resolves the active bot token and returns a ready API client.
// DSCRD_API_BASE overrides the API base URL (tests, proxies).
func buildClient(g *GlobalFlags) (*api.Client, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	token, err := auth.ResolveToken(cfg, profileName(g), os.Getenv("DSCRD_TOKEN"))
	if err != nil {
		return nil, err
	}
	return newClient(token), nil
}

// newClient returns an API client for token, honoring DSCRD_API_BASE. An empty
// token yields an unauthenticated client (webhook execution).
func newClient(token string) *api.Client {
	c := api.New(token)
	if base := os.Getenv("DSCRD_API_BASE"); base != "" {
		c.BaseURL = base
	}
	return c
}

// loadConfig reads the dscrd config from its resolved path.
func loadConfig() (*auth.Config, error) {
	path, err := auth.ConfigPath()
	if err != nil {
		return nil, err
	}
	return auth.Load(path)
}

// cacheDir returns the resolver cache directory, colocated with the config.
func cacheDir() string {
	path, err := auth.ConfigPath()
	if err != nil {
		return os.TempDir()
	}
	return filepath.Join(filepath.Dir(path), "cache")
}

// guildID resolves the target guild: the --guild flag, falling back to the
// active profile's default_guild. A snowflake passes through; anything else is
// matched (case-insensitively) against the names of the guilds the bot is in.
func guildID(g *GlobalFlags, c *api.Client) (string, error) {
	target := g.Guild
	if target == "" {
		if cfg, err := loadConfig(); err == nil {
			if p, _, ok := auth.ActiveProfile(cfg, profileName(g)); ok {
				target = p.DefaultGuild
			}
		}
	}
	if target == "" {
		return "", &auth.AuthError{Reason: "no guild specified; use --guild <id|name> or set default_guild on the profile"}
	}
	if resolve.IsSnowflake(target) {
		return target, nil
	}

	raw, err := c.Do("GET", "/users/@me/guilds", nil)
	if err != nil {
		return "", err
	}
	var guilds []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &guilds); err != nil {
		return "", err
	}
	for _, gd := range guilds {
		if strings.EqualFold(gd.Name, target) {
			return gd.ID, nil
		}
	}
	return "", fmt.Errorf("guild %q not found among the bot's servers (run 'dscrd guild list')", target)
}

// channelID resolves a channel argument: a snowflake passes through; a name
// (with or without leading '#') is matched against the target guild's channels.
func channelID(g *GlobalFlags, c *api.Client, arg string) (string, error) {
	if resolve.IsSnowflake(arg) {
		return arg, nil
	}
	name := strings.TrimPrefix(arg, "#")
	gid, err := guildID(g, c)
	if err != nil {
		return "", err
	}
	raw, err := c.Do("GET", "/guilds/"+gid+"/channels", nil)
	if err != nil {
		return "", err
	}
	var channels []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &channels); err != nil {
		return "", err
	}
	for _, ch := range channels {
		if strings.EqualFold(ch.Name, name) {
			return ch.ID, nil
		}
	}
	return "", fmt.Errorf("channel %q not found in guild %s (run 'dscrd channel list')", arg, gid)
}

// clientAndChannel resolves the API client and channel ID together. With
// --dry-run and a snowflake channel there is nothing to resolve, so no client
// (and no credential) is required at all; the returned client is nil.
func clientAndChannel(g *GlobalFlags, channel string) (*api.Client, string, error) {
	if g.DryRun && resolve.IsSnowflake(channel) {
		return nil, channel, nil
	}
	client, err := buildClient(g)
	if err != nil {
		return nil, "", err
	}
	cid, err := channelID(g, client, channel)
	if err != nil {
		return nil, "", err
	}
	return client, cid, nil
}

// clientAndGuild resolves the API client and guild ID together. With --dry-run
// and a snowflake --guild there is nothing to resolve, so no client (and no
// credential) is required; the returned client is nil.
func clientAndGuild(g *GlobalFlags) (*api.Client, string, error) {
	if g.DryRun && resolve.IsSnowflake(g.Guild) {
		return nil, g.Guild, nil
	}
	client, err := buildClient(g)
	if err != nil {
		return nil, "", err
	}
	gid, err := guildID(g, client)
	if err != nil {
		return nil, "", err
	}
	return client, gid, nil
}

// apiLookup implements resolve.Lookup over the Discord API.
type apiLookup struct {
	client *api.Client
}

func (l apiLookup) Name(kind, id string) (string, error) {
	switch kind {
	case "user":
		raw, err := l.client.Do("GET", "/users/"+id, nil)
		if err != nil {
			return "", err
		}
		var u struct {
			Username   string `json:"username"`
			GlobalName string `json:"global_name"`
		}
		if err := json.Unmarshal(raw, &u); err != nil {
			return "", err
		}
		if u.GlobalName != "" {
			return u.GlobalName, nil
		}
		return u.Username, nil
	case "channel":
		raw, err := l.client.Do("GET", "/channels/"+id, nil)
		if err != nil {
			return "", err
		}
		var ch struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &ch); err != nil {
			return "", err
		}
		return ch.Name, nil
	default:
		return "", fmt.Errorf("resolve: unsupported kind %q", kind)
	}
}

// newResolver returns a Resolver for pretty output, or nil when --no-resolve.
func newResolver(g *GlobalFlags, c *api.Client) *resolve.Resolver {
	if g.NoResolve {
		return nil
	}
	return resolve.New(filepath.Join(cacheDir(), "resolve.json"), apiLookup{client: c})
}

// dryRunLocal renders a pending local mutation for --dry-run. Commands that
// only touch the config file have no method/path to show, so they cannot use
// dryRunPrint.
func dryRunLocal(cmd *cobra.Command, format string, args ...any) {
	fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] "+format+"\n", args...)
}

// dryRunPrint renders the about-to-fire call for --dry-run.
func dryRunPrint(cmd *cobra.Command, method, path string, body any) {
	if body == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] %s %s\n", method, path)
		return
	}
	b, _ := json.Marshal(body)
	fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] %s %s %s\n", method, path, b)
}
