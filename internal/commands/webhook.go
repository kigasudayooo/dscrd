package commands

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newWebhookCommand(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "webhook", Short: "Manage webhooks"}
	cmd.AddCommand(
		newWebhookListCommand(g),
		newWebhookCreateCommand(g),
		newWebhookDeleteCommand(g),
		newWebhookExecuteCommand(g),
	)
	return cmd
}

// webhookItem is one rendered webhook.
type webhookItem struct {
	Name      string `json:"name"`
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
}

func (w webhookItem) Concise() string {
	return fmt.Sprintf("%s (%s) — channel:%s", w.Name, w.ID, w.ChannelID)
}

func newWebhookListCommand(g *GlobalFlags) *cobra.Command {
	var channel string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List webhooks for the server or one channel",
		Annotations: map[string]string{
			"discordEndpoint": "GET /guilds/{guild.id}/webhooks",
			"perms":           "MANAGE_WEBHOOKS",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			var path string
			if channel != "" {
				cid, err := channelID(g, client, channel)
				if err != nil {
					return err
				}
				path = "/channels/" + cid + "/webhooks"
			} else {
				gid, err := guildID(g, client)
				if err != nil {
					return err
				}
				path = "/guilds/" + gid + "/webhooks"
			}
			raw, err := client.Do("GET", path, nil)
			if err != nil {
				return err
			}
			if g.Raw {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var hooks []webhookItem
			if err := json.Unmarshal(raw, &hooks); err != nil {
				return err
			}
			return emit(cmd, g, hooks)
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "restrict to one channel (ID or name)")
	return cmd
}

func newWebhookCreateCommand(g *GlobalFlags) *cobra.Command {
	var channel, name string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a webhook on a channel",
		Annotations: map[string]string{
			"discordEndpoint": "POST /channels/{channel.id}/webhooks",
			"write":           "true",
			"perms":           "MANAGE_WEBHOOKS",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cid, err := clientAndChannel(g, channel)
			if err != nil {
				return err
			}
			body := map[string]any{"name": name}
			path := "/channels/" + cid + "/webhooks"
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
			var hook struct {
				ID    string `json:"id"`
				Token string `json:"token"`
			}
			if err := json.Unmarshal(raw, &hook); err != nil {
				return err
			}
			// The token is shown once: it is required to execute the webhook and
			// cannot be retrieved later without MANAGE_WEBHOOKS.
			fmt.Fprintf(cmd.OutOrStdout(), "created webhook %s token:%s\n", hook.ID, hook.Token)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().StringVar(&name, "name", "", "webhook name")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("name")
	return cmd
}

func newWebhookDeleteCommand(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <webhook-id>",
		Short: "Delete a webhook",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			"discordEndpoint": "DELETE /webhooks/{webhook.id}",
			"write":           "true",
			"perms":           "MANAGE_WEBHOOKS",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireSnowflake("webhook-id", args[0]); err != nil {
				return err
			}
			if g.DryRun {
				dryRunPrint(cmd, "DELETE", "/webhooks/"+args[0], nil)
				return nil
			}
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			if _, err := client.Do("DELETE", "/webhooks/"+args[0], nil); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted webhook %s\n", args[0])
			return nil
		},
	}
	return cmd
}

func newWebhookExecuteCommand(g *GlobalFlags) *cobra.Command {
	var id, token, text, textFile string
	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Post a message through a webhook",
		Annotations: map[string]string{
			"discordEndpoint": "POST /webhooks/{webhook.id}/{webhook.token}",
			"write":           "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := readContent(text, textFile, "--text", "--text-file")
			if err != nil {
				return err
			}
			path := "/webhooks/" + id + "/" + token
			// Never echo the webhook token: not in dry-run output, not in errors.
			shown := "/webhooks/" + id + "/<token>"
			body := map[string]any{"content": content}
			if g.DryRun {
				dryRunPrint(cmd, "POST", shown, body)
				return nil
			}
			// The token in the path is the credential; the bot token is neither
			// needed nor sent.
			if _, err := newClient("").DoSecretPath("POST", path, shown, body); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "executed")
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "webhook ID")
	cmd.Flags().StringVar(&token, "token", "", "webhook token")
	cmd.Flags().StringVar(&text, "text", "", "message text")
	cmd.Flags().StringVar(&textFile, "text-file", "", "path to text file (use - for stdin)")
	cmd.MarkFlagRequired("id")
	cmd.MarkFlagRequired("token")
	return cmd
}
