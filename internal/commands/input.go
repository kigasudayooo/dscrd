package commands

import (
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/howar31/dscrd/internal/resolve"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// idFlags are the flags that only ever carry a Discord snowflake. Their values
// are spliced into API paths, so anything else ("../", "?x=1") would redirect
// the call to a different endpoint. Flags that also accept a name (--channel,
// --guild) are resolved through channelID/guildID instead.
var idFlags = map[string]bool{
	"message": true, "user": true, "role": true, "thread": true, "id": true,
	"from-message": true, "reply-to": true, "before": true, "after": true,
	"around": true, "parent": true, "author": true,
}

// validateIDFlags rejects a non-snowflake value in any explicitly set ID flag.
// It runs before every command, so it also guards --dry-run output.
func validateIDFlags(cmd *cobra.Command) error {
	var err error
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if err == nil && idFlags[f.Name] {
			err = requireSnowflake("--"+f.Name, f.Value.String())
		}
	})
	return err
}

// requireSnowflake reports an error unless v is a Discord snowflake ID. label
// names the flag or argument in the message.
func requireSnowflake(label, v string) error {
	if !resolve.IsSnowflake(v) {
		return fmt.Errorf("%s must be a Discord ID (17-20 digits), got %q", label, v)
	}
	return nil
}

// inviteCodePattern is the shape of an invite code (vanity codes included).
var inviteCodePattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

// requireInviteCode reports an error unless code is a bare invite code.
func requireInviteCode(code string) error {
	if !inviteCodePattern.MatchString(code) {
		return fmt.Errorf("invalid invite code %q: pass the bare code, not a URL", code)
	}
	return nil
}

// readContent returns message text from either an inline flag value or a file
// ("-" reads stdin). Exactly one of the two must be provided; flagName and
// fileFlagName appear in error messages.
func readContent(text, file, flagName, fileFlagName string) (string, error) {
	switch {
	case text != "" && file != "":
		return "", fmt.Errorf("use %s or %s, not both", flagName, fileFlagName)
	case text != "":
		return text, nil
	case file == "":
		return "", fmt.Errorf("provide %s or %s", flagName, fileFlagName)
	case file == "-":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(data), nil
	default:
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}
