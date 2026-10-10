package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/howar31/dscrd/internal/api"
	"github.com/spf13/cobra"
)

func newDMCommand(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dm",
		Short: "Direct messages (sent as the bot; requires a mutual server)",
	}
	cmd.AddCommand(newDMSendCommand(g), newDMReadCommand(g))
	return cmd
}

// openDM creates (or reuses) the DM channel with user and returns its ID.
func openDM(c *api.Client, user string) (string, error) {
	raw, err := c.Do("POST", "/users/@me/channels", map[string]string{"recipient_id": user})
	if err != nil {
		return "", err
	}
	var ch struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &ch); err != nil {
		return "", err
	}
	return ch.ID, nil
}

func newDMSendCommand(g *GlobalFlags) *cobra.Command {
	var user, text, textFile string
	var allowMentions bool
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a DM to a user (as the bot)",
		Annotations: map[string]string{
			"discordEndpoint": "POST /users/@me/channels + POST /channels/{dm.id}/messages",
			"write":           "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := readContent(text, textFile, "--text", "--text-file")
			if err != nil {
				return err
			}
			if g.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] DM to %s: %s\n", user, content)
				return nil
			}
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			dmID, err := openDM(client, user)
			if err != nil {
				return err
			}
			raw, err := client.Do("POST", "/channels/"+dmID+"/messages", messageBody(content, allowMentions))
			if err != nil {
				return err
			}
			if g.Raw {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var sent struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(raw, &sent); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "sent %s (dm %s)\n", sent.ID, dmID)
			return nil
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "recipient user ID")
	bindMentionsFlag(cmd, &allowMentions)
	cmd.Flags().StringVar(&text, "text", "", "message text")
	cmd.Flags().StringVar(&textFile, "text-file", "", "path to text file (use - for stdin)")
	cmd.MarkFlagRequired("user")
	return cmd
}

func newDMReadCommand(g *GlobalFlags) *cobra.Command {
	var user string
	var limit int
	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read the bot's DM history with a user",
		Annotations: map[string]string{
			"discordEndpoint": "POST /users/@me/channels + GET /channels/{dm.id}/messages",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			dmID, err := openDM(client, user)
			if err != nil {
				return err
			}
			q := url.Values{}
			q.Set("limit", strconv.Itoa(limit))
			raw, err := client.Do("GET", "/channels/"+dmID+"/messages?"+q.Encode(), nil)
			if err != nil {
				return err
			}
			if g.Raw {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var msgs []rawMessage
			if err := json.Unmarshal(raw, &msgs); err != nil {
				return err
			}
			r := newResolver(g, client)
			items := make([]msgItem, 0, len(msgs))
			for i := len(msgs) - 1; i >= 0; i-- {
				items = append(items, renderMessage(r, msgs[i]))
			}
			return emit(cmd, g, items)
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "user ID")
	cmd.Flags().IntVar(&limit, "limit", 5, "max messages")
	cmd.MarkFlagRequired("user")
	return cmd
}
