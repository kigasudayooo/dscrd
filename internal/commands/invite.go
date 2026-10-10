package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newInviteCommand(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "invite", Short: "Manage invite links"}
	cmd.AddCommand(
		newInviteListCommand(g),
		newInviteCreateCommand(g),
		newInviteDeleteCommand(g),
		newInviteInfoCommand(g),
	)
	return cmd
}

// inviteItem is one rendered invite.
type inviteItem struct {
	Code    string `json:"code"`
	Channel string `json:"channel"`
	Uses    int    `json:"uses"`
	MaxUses int    `json:"max_uses"`
}

func (i inviteItem) Concise() string {
	s := fmt.Sprintf("%s — #%s uses:%d", i.Code, i.Channel, i.Uses)
	if i.MaxUses > 0 {
		s += fmt.Sprintf("/%d", i.MaxUses)
	}
	return s
}

// rawInvite is the wire shape of an invite.
type rawInvite struct {
	Code    string `json:"code"`
	Uses    int    `json:"uses"`
	MaxUses int    `json:"max_uses"`
	Channel struct {
		Name string `json:"name"`
	} `json:"channel"`
	Guild struct {
		Name string `json:"name"`
	} `json:"guild"`
}

func renderInvite(in rawInvite) inviteItem {
	return inviteItem{Code: in.Code, Channel: in.Channel.Name, Uses: in.Uses, MaxUses: in.MaxUses}
}

func newInviteListCommand(g *GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the server's invites",
		Annotations: map[string]string{
			"discordEndpoint": "GET /guilds/{guild.id}/invites",
			"perms":           "MANAGE_GUILD",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, gid, err := clientAndGuild(g)
			if err != nil {
				return err
			}
			raw, err := client.Do("GET", "/guilds/"+gid+"/invites", nil)
			if err != nil {
				return err
			}
			if g.Raw {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var invites []rawInvite
			if err := json.Unmarshal(raw, &invites); err != nil {
				return err
			}
			items := make([]inviteItem, len(invites))
			for i, in := range invites {
				items[i] = renderInvite(in)
			}
			return emit(cmd, g, items)
		},
	}
}

func newInviteCreateCommand(g *GlobalFlags) *cobra.Command {
	var channel string
	var maxAge, maxUses int
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an invite for a channel",
		Annotations: map[string]string{
			"discordEndpoint": "POST /channels/{channel.id}/invites",
			"write":           "true",
			"perms":           "CREATE_INSTANT_INVITE",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cid, err := clientAndChannel(g, channel)
			if err != nil {
				return err
			}
			body := map[string]any{"max_age": maxAge, "max_uses": maxUses}
			path := "/channels/" + cid + "/invites"
			if g.DryRun {
				dryRunPrint(cmd, "POST", path, body)
				return nil
			}
			raw, err := client.Do("POST", path, body)
			if err != nil {
				return err
			}
			if g.Raw {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var in rawInvite
			if err := json.Unmarshal(raw, &in); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created https://discord.gg/%s\n", in.Code)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().IntVar(&maxAge, "max-age", 86400, "expiry in seconds (0 = never)")
	cmd.Flags().IntVar(&maxUses, "max-uses", 0, "max uses (0 = unlimited)")
	cmd.MarkFlagRequired("channel")
	return cmd
}

func newInviteDeleteCommand(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <code>",
		Short: "Revoke an invite",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			"discordEndpoint": "DELETE /invites/{invite.code}",
			"write":           "true",
			"perms":           "MANAGE_CHANNELS",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInviteCode(args[0]); err != nil {
				return err
			}
			if g.DryRun {
				dryRunPrint(cmd, "DELETE", "/invites/"+args[0], nil)
				return nil
			}
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			if _, err := client.Do("DELETE", "/invites/"+args[0], nil); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "revoked %s\n", args[0])
			return nil
		},
	}
	return cmd
}

func newInviteInfoCommand(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <code>",
		Short: "Show an invite's details",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			"discordEndpoint": "GET /invites/{invite.code}",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInviteCode(args[0]); err != nil {
				return err
			}
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			raw, err := client.Do("GET", "/invites/"+args[0], nil)
			if err != nil {
				return err
			}
			if g.Raw {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var in rawInvite
			if err := json.Unmarshal(raw, &in); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — server:%s channel:#%s\n", in.Code, in.Guild.Name, in.Channel.Name)
			return nil
		},
	}
	return cmd
}
