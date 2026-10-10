// Package commands defines the dscrd command tree.
package commands

import (
	"github.com/howar31/dscrd/internal/output"
	"github.com/spf13/cobra"
)

// emit renders items to the command's stdout in the selected format.
func emit(cmd *cobra.Command, g *GlobalFlags, items any) error {
	return output.Emit(cmd.OutOrStdout(), g.Format, items)
}

// NewRootCommand builds the dscrd root command for the given build version.
func NewRootCommand(version string) *cobra.Command {
	g := &GlobalFlags{}
	root := &cobra.Command{
		Use:           "dscrd",
		Short:         "Agent-facing Discord CLI",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// Keep `dscrd --version` and the `version` subcommand byte-for-byte
	// consistent ("dscrd <version>"); Cobra's default template prints
	// "dscrd version <version>".
	root.SetVersionTemplate("dscrd {{.Version}}\n")
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return validateIDFlags(cmd)
	}
	bindGlobalFlags(root, g)
	root.AddCommand(
		newAPICommand(g),
		newAuthCommand(g),
		newGuildCommand(g),
		newChannelCommand(g),
		newMsgCommand(g),
		newThreadCommand(g),
		newSearchCommand(g),
		newMemberCommand(g),
		newRoleCommand(g),
		newEmojiCommand(g),
		newFileCommand(g),
		newDMCommand(g),
		newUserCommand(g),
		newInviteCommand(g),
		newWebhookCommand(g),
		newAuditLogCommand(g),
		newEventCommand(g),
		newStickerCommand(g),
		newPollCommand(g),
		newAutomodCommand(g),
		newVersionCommand(g, version),
		newGenerateSkillsCommand(version),
	)
	return root
}
