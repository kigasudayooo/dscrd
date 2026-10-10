package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

func newThreadCommand(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "thread", Short: "Work with threads and forum posts"}
	cmd.AddCommand(
		newThreadListCommand(g),
		newThreadCreateCommand(g),
		newThreadReadCommand(g),
		newThreadReplyCommand(g),
		newThreadArchiveCommand(g),
	)
	return cmd
}

// threadItem is one row of `thread list`.
type threadItem struct {
	Name     string `json:"name"`
	ID       string `json:"id"`
	ParentID string `json:"parent_id"`
	Archived bool   `json:"archived"`
}

func (t threadItem) Concise() string {
	s := fmt.Sprintf("%s (%s) — in %s", t.Name, t.ID, t.ParentID)
	if t.Archived {
		s += ", archived"
	}
	return s
}

// rawThread is the wire shape of a thread channel.
type rawThread struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	ParentID       string `json:"parent_id"`
	ThreadMetadata struct {
		Archived bool `json:"archived"`
	} `json:"thread_metadata"`
}

func threadItems(threads []rawThread) []threadItem {
	items := make([]threadItem, len(threads))
	for i, th := range threads {
		items[i] = threadItem{Name: th.Name, ID: th.ID, ParentID: th.ParentID, Archived: th.ThreadMetadata.Archived}
	}
	return items
}

func newThreadListCommand(g *GlobalFlags) *cobra.Command {
	var channel string
	var archived bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active threads in the server (or archived ones in a channel)",
		Annotations: map[string]string{
			"discordEndpoint": "GET /guilds/{guild.id}/threads/active",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			var path string
			if archived {
				if channel == "" {
					return fmt.Errorf("--archived requires --channel")
				}
				cid, err := channelID(g, client, channel)
				if err != nil {
					return err
				}
				path = "/channels/" + cid + "/threads/archived/public"
			} else {
				gid, err := guildID(g, client)
				if err != nil {
					return err
				}
				path = "/guilds/" + gid + "/threads/active"
			}
			raw, err := client.Do("GET", path, nil)
			if err != nil {
				return err
			}
			if g.Raw {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var resp struct {
				Threads []rawThread `json:"threads"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				return err
			}
			return emit(cmd, g, threadItems(resp.Threads))
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name (for --archived)")
	cmd.Flags().BoolVar(&archived, "archived", false, "list archived public threads of --channel")
	return cmd
}

func newThreadCreateCommand(g *GlobalFlags) *cobra.Command {
	var channel, name, fromMessage, text, textFile string
	var allowMentions bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a thread (from a message, standalone, or a forum post with --text)",
		Annotations: map[string]string{
			"discordEndpoint": "POST /channels/{channel.id}/threads",
			"write":           "true",
			"perms":           "CREATE_PUBLIC_THREADS",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			cid, err := channelID(g, client, channel)
			if err != nil {
				return err
			}
			var path string
			body := map[string]any{"name": name}
			switch {
			case fromMessage != "":
				path = "/channels/" + cid + "/messages/" + fromMessage + "/threads"
			case text != "" || textFile != "":
				// Forum/media channels require an initial message with the thread.
				content, err := readContent(text, textFile, "--text", "--text-file")
				if err != nil {
					return err
				}
				path = "/channels/" + cid + "/threads"
				body["message"] = messageBody(content, allowMentions)
			default:
				path = "/channels/" + cid + "/threads"
				body["type"] = 11 // public thread
			}
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
			var th rawThread
			if err := json.Unmarshal(raw, &th); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created thread %s (%s)\n", th.Name, th.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "parent channel ID or name")
	cmd.Flags().StringVar(&name, "name", "", "thread name")
	cmd.Flags().StringVar(&fromMessage, "from-message", "", "start the thread from this message ID")
	bindMentionsFlag(cmd, &allowMentions)
	cmd.Flags().StringVar(&text, "text", "", "initial message (required for forum channels)")
	cmd.Flags().StringVar(&textFile, "text-file", "", "path to initial message file (use - for stdin)")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("name")
	return cmd
}

func newThreadReadCommand(g *GlobalFlags) *cobra.Command {
	var thread string
	var limit int
	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read messages in a thread",
		Annotations: map[string]string{
			"discordEndpoint": "GET /channels/{thread.id}/messages",
			"intents":         "MESSAGE_CONTENT",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			q := url.Values{}
			q.Set("limit", strconv.Itoa(limit))
			raw, err := client.Do("GET", "/channels/"+thread+"/messages?"+q.Encode(), nil)
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
	cmd.Flags().StringVar(&thread, "thread", "", "thread ID")
	cmd.Flags().IntVar(&limit, "limit", 10, "max messages")
	cmd.MarkFlagRequired("thread")
	return cmd
}

func newThreadReplyCommand(g *GlobalFlags) *cobra.Command {
	var thread, text, textFile string
	var allowMentions bool
	cmd := &cobra.Command{
		Use:   "reply",
		Short: "Reply in a thread",
		Annotations: map[string]string{
			"discordEndpoint": "POST /channels/{thread.id}/messages",
			"write":           "true",
			"perms":           "SEND_MESSAGES_IN_THREADS",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := readContent(text, textFile, "--text", "--text-file")
			if err != nil {
				return err
			}
			path := "/channels/" + thread + "/messages"
			body := messageBody(content, allowMentions)
			if g.DryRun {
				dryRunPrint(cmd, "POST", path, body)
				return nil
			}
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			raw, err := client.Do("POST", path, body)
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
			fmt.Fprintf(cmd.OutOrStdout(), "replied %s\n", sent.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&thread, "thread", "", "thread ID")
	bindMentionsFlag(cmd, &allowMentions)
	cmd.Flags().StringVar(&text, "text", "", "reply text")
	cmd.Flags().StringVar(&textFile, "text-file", "", "path to text file (use - for stdin)")
	cmd.MarkFlagRequired("thread")
	return cmd
}

func newThreadArchiveCommand(g *GlobalFlags) *cobra.Command {
	var thread string
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Archive a thread",
		Annotations: map[string]string{
			"discordEndpoint": "PATCH /channels/{thread.id}",
			"write":           "true",
			"perms":           "MANAGE_THREADS",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/channels/" + thread
			body := map[string]any{"archived": true}
			if g.DryRun {
				dryRunPrint(cmd, "PATCH", path, body)
				return nil
			}
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			if _, err := client.Do("PATCH", path, body); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "archived %s\n", thread)
			return nil
		},
	}
	cmd.Flags().StringVar(&thread, "thread", "", "thread ID")
	cmd.MarkFlagRequired("thread")
	return cmd
}
