// Command dscrd is an agent-facing Discord CLI.
package main

import (
	"errors"
	"fmt"
	"os"

	dscrd "github.com/howar31/dscrd"
	"github.com/howar31/dscrd/internal/api"
	"github.com/howar31/dscrd/internal/auth"
	"github.com/howar31/dscrd/internal/commands"
	"github.com/howar31/dscrd/internal/output"
)

func main() {
	root := commands.NewRootCommand(dscrd.Version)
	// Remote text (names, message content, API error messages) must not be
	// able to drive the terminal.
	stdout, stderr := output.SafeWriter(os.Stdout), output.SafeWriter(os.Stderr)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(stderr, "dscrd:", err)
		var apiErr *api.APIError
		if errors.As(err, &apiErr) {
			os.Exit(apiErr.ExitCode())
		}
		var authErr *auth.AuthError
		if errors.As(err, &authErr) {
			os.Exit(authErr.ExitCode())
		}
		os.Exit(1)
	}
}
