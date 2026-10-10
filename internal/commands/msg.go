package commands

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/howar31/dscrd/internal/resolve"
	"github.com/spf13/cobra"
)

func newMsgCommand(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "msg", Short: "Read and send messages"}
	cmd.AddCommand(
		newMsgReadCommand(g),
		newMsgSendCommand(g),
		newMsgEditCommand(g),
		newMsgDeleteCommand(g),
		newMsgReactCommand(g),
		newMsgUnreactCommand(g),
		newMsgReactionsCommand(g),
		newMsgPinCommand(g),
		newMsgUnpinCommand(g),
		newMsgPinsCommand(g),
		newMsgPermalinkCommand(g),
	)
	return cmd
}

// rawMessage is the wire shape of a Discord message (the fields we render).
type rawMessage struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
	ChannelID string `json:"channel_id"`
	Author    struct {
		Username   string `json:"username"`
		GlobalName string `json:"global_name"`
	} `json:"author"`
	Attachments []struct {
		Filename string `json:"filename"`
		URL      string `json:"url"`
	} `json:"attachments"`
	Embeds []struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	} `json:"embeds"`
}

// msgItem is one rendered message.
type msgItem struct {
	Author string `json:"author"`
	Text   string `json:"text"`
	When   string `json:"when"`
	ID     string `json:"id"`
}

func (m msgItem) Concise() string {
	return fmt.Sprintf("%s: %s [%s] (%s)", m.Author, m.Text, m.When, m.ID)
}

// messageWhen renders a Discord ISO8601 timestamp as local MM-DD HH:MM.
func messageWhen(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return t.Local().Format("01-02 15:04")
}

// mentionPattern matches user (<@id>, <@!id>) and channel (<#id>) mentions.
var mentionPattern = regexp.MustCompile(`<(@!?|#)(\d+)>`)

// prettifyMentions rewrites raw mention markup to @name / #name via r.
// A nil resolver (--no-resolve) leaves content untouched.
func prettifyMentions(r *resolve.Resolver, content string) string {
	if r == nil {
		return content
	}
	return mentionPattern.ReplaceAllStringFunc(content, func(m string) string {
		parts := mentionPattern.FindStringSubmatch(m)
		id := parts[2]
		if parts[1] == "#" {
			return "#" + r.Resolve("channel", id)
		}
		return "@" + r.Resolve("user", id)
	})
}

// renderMessage converts a wire message into a display item.
func renderMessage(r *resolve.Resolver, m rawMessage) msgItem {
	author := m.Author.GlobalName
	if author == "" {
		author = m.Author.Username
	}
	text := prettifyMentions(r, m.Content)
	for _, a := range m.Attachments {
		if text != "" {
			text += " "
		}
		text += fmt.Sprintf("[file:%s %s]", a.Filename, a.URL)
	}
	for _, e := range m.Embeds {
		part := strings.TrimSpace(strings.TrimSpace(e.Title) + " — " + strings.TrimSpace(embedExcerpt(e.Description)))
		part = strings.Trim(part, "— ")
		if part == "" {
			continue
		}
		if text != "" {
			text += " "
		}
		text += "[embed: " + part + "]"
	}
	return msgItem{Author: author, Text: text, When: messageWhen(m.Timestamp), ID: m.ID}
}

// embedExcerpt caps an embed description to one concise line.
func embedExcerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const max = 120
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func newMsgReadCommand(g *GlobalFlags) *cobra.Command {
	var channel, before, after, around string
	var limit int
	cmd := &cobra.Command{
		Use:   "read",
		Short: "Read recent messages in a channel",
		Annotations: map[string]string{
			"discordEndpoint": "GET /channels/{channel.id}/messages",
			"perms":           "VIEW_CHANNEL,READ_MESSAGE_HISTORY",
			"intents":         "MESSAGE_CONTENT",
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
			q := url.Values{}
			q.Set("limit", strconv.Itoa(limit))
			if before != "" {
				q.Set("before", before)
			}
			if after != "" {
				q.Set("after", after)
			}
			if around != "" {
				q.Set("around", around)
			}
			raw, err := client.Do("GET", "/channels/"+cid+"/messages?"+q.Encode(), nil)
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
			// Discord returns newest first; render oldest first for reading flow.
			for i := len(msgs) - 1; i >= 0; i-- {
				items = append(items, renderMessage(r, msgs[i]))
			}
			return emit(cmd, g, items)
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().IntVar(&limit, "limit", 5, "max messages")
	cmd.Flags().StringVar(&before, "before", "", "only messages before this message ID")
	cmd.Flags().StringVar(&after, "after", "", "only messages after this message ID")
	cmd.Flags().StringVar(&around, "around", "", "messages around this message ID")
	cmd.MarkFlagRequired("channel")
	return cmd
}

func newMsgSendCommand(g *GlobalFlags) *cobra.Command {
	var channel, text, textFile, replyTo, file string
	var allowMentions bool
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a message (optionally as a reply or with a file)",
		Annotations: map[string]string{
			"discordEndpoint": "POST /channels/{channel.id}/messages",
			"write":           "true",
			"perms":           "SEND_MESSAGES",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			content := ""
			if text != "" || textFile != "" {
				var err error
				content, err = readContent(text, textFile, "--text", "--text-file")
				if err != nil {
					return err
				}
			} else if file == "" {
				return fmt.Errorf("provide --text, --text-file, or --file")
			}
			body := messageBody(content, allowMentions)
			if replyTo != "" {
				body["message_reference"] = map[string]string{"message_id": replyTo}
			}
			client, cid, err := clientAndChannel(g, channel)
			if err != nil {
				return err
			}
			path := "/channels/" + cid + "/messages"
			if g.DryRun {
				dryRunPrint(cmd, "POST", path, body)
				return nil
			}
			var raw []byte
			if file != "" {
				payload, err := json.Marshal(body)
				if err != nil {
					return err
				}
				raw, err = client.DoMultipart("POST", path, payload, file)
				if err != nil {
					return err
				}
			} else {
				raw, err = client.Do("POST", path, body)
				if err != nil {
					return err
				}
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
			fmt.Fprintf(cmd.OutOrStdout(), "sent %s\n", sent.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().StringVar(&text, "text", "", "message text")
	cmd.Flags().StringVar(&textFile, "text-file", "", "path to text file (use - for stdin)")
	cmd.Flags().StringVar(&replyTo, "reply-to", "", "message ID to reply to")
	cmd.Flags().StringVar(&file, "file", "", "path of a file to attach")
	bindMentionsFlag(cmd, &allowMentions)
	cmd.MarkFlagRequired("channel")
	return cmd
}

func newMsgEditCommand(g *GlobalFlags) *cobra.Command {
	var channel, message, text, textFile string
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit a message the bot sent",
		Annotations: map[string]string{
			"discordEndpoint": "PATCH /channels/{channel.id}/messages/{message.id}",
			"write":           "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			content, err := readContent(text, textFile, "--text", "--text-file")
			if err != nil {
				return err
			}
			client, cid, err := clientAndChannel(g, channel)
			if err != nil {
				return err
			}
			path := "/channels/" + cid + "/messages/" + message
			body := map[string]any{"content": content}
			if g.DryRun {
				dryRunPrint(cmd, "PATCH", path, body)
				return nil
			}
			raw, err := client.Do("PATCH", path, body)
			if err != nil {
				return err
			}
			if g.Raw {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "edited %s\n", message)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().StringVar(&message, "message", "", "message ID")
	cmd.Flags().StringVar(&text, "text", "", "new text")
	cmd.Flags().StringVar(&textFile, "text-file", "", "path to text file (use - for stdin)")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("message")
	return cmd
}

func newMsgDeleteCommand(g *GlobalFlags) *cobra.Command {
	var channel, message string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a message",
		Annotations: map[string]string{
			"discordEndpoint": "DELETE /channels/{channel.id}/messages/{message.id}",
			"write":           "true",
			"perms":           "MANAGE_MESSAGES",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cid, err := clientAndChannel(g, channel)
			if err != nil {
				return err
			}
			path := "/channels/" + cid + "/messages/" + message
			if g.DryRun {
				dryRunPrint(cmd, "DELETE", path, nil)
				return nil
			}
			if _, err := client.Do("DELETE", path, nil); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %s\n", message)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().StringVar(&message, "message", "", "message ID")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("message")
	return cmd
}

// reactionPath builds the reaction endpoint for an emoji (unicode or name:id).
func reactionPath(cid, mid, emoji, suffix string) string {
	return "/channels/" + cid + "/messages/" + mid + "/reactions/" + url.PathEscape(emoji) + suffix
}

func newMsgReactCommand(g *GlobalFlags) *cobra.Command {
	var channel, message, emoji string
	cmd := &cobra.Command{
		Use:   "react",
		Short: "Add a reaction (unicode emoji or custom name:id)",
		Annotations: map[string]string{
			"discordEndpoint": "PUT /channels/{channel.id}/messages/{message.id}/reactions/{emoji}/@me",
			"write":           "true",
			"perms":           "ADD_REACTIONS",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cid, err := clientAndChannel(g, channel)
			if err != nil {
				return err
			}
			path := reactionPath(cid, message, emoji, "/@me")
			if g.DryRun {
				dryRunPrint(cmd, "PUT", path, nil)
				return nil
			}
			if _, err := client.Do("PUT", path, nil); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "reacted")
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().StringVar(&message, "message", "", "message ID")
	cmd.Flags().StringVar(&emoji, "emoji", "", "emoji (unicode or custom name:id)")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("message")
	cmd.MarkFlagRequired("emoji")
	return cmd
}

func newMsgUnreactCommand(g *GlobalFlags) *cobra.Command {
	var channel, message, emoji string
	cmd := &cobra.Command{
		Use:   "unreact",
		Short: "Remove the bot's reaction",
		Annotations: map[string]string{
			"discordEndpoint": "DELETE /channels/{channel.id}/messages/{message.id}/reactions/{emoji}/@me",
			"write":           "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cid, err := clientAndChannel(g, channel)
			if err != nil {
				return err
			}
			path := reactionPath(cid, message, emoji, "/@me")
			if g.DryRun {
				dryRunPrint(cmd, "DELETE", path, nil)
				return nil
			}
			if _, err := client.Do("DELETE", path, nil); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "unreacted")
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().StringVar(&message, "message", "", "message ID")
	cmd.Flags().StringVar(&emoji, "emoji", "", "emoji (unicode or custom name:id)")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("message")
	cmd.MarkFlagRequired("emoji")
	return cmd
}

// userItem is a rendered user reference (reaction lists, member lists).
type userItem struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

func (u userItem) Concise() string { return fmt.Sprintf("%s (%s)", u.Name, u.ID) }

func newMsgReactionsCommand(g *GlobalFlags) *cobra.Command {
	var channel, message, emoji string
	cmd := &cobra.Command{
		Use:   "reactions",
		Short: "List users who reacted with an emoji",
		Annotations: map[string]string{
			"discordEndpoint": "GET /channels/{channel.id}/messages/{message.id}/reactions/{emoji}",
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
			raw, err := client.Do("GET", reactionPath(cid, message, emoji, ""), nil)
			if err != nil {
				return err
			}
			if g.Raw {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var users []struct {
				ID         string `json:"id"`
				Username   string `json:"username"`
				GlobalName string `json:"global_name"`
			}
			if err := json.Unmarshal(raw, &users); err != nil {
				return err
			}
			items := make([]userItem, len(users))
			for i, u := range users {
				name := u.GlobalName
				if name == "" {
					name = u.Username
				}
				items[i] = userItem{Name: name, ID: u.ID}
			}
			return emit(cmd, g, items)
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().StringVar(&message, "message", "", "message ID")
	cmd.Flags().StringVar(&emoji, "emoji", "", "emoji (unicode or custom name:id)")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("message")
	cmd.MarkFlagRequired("emoji")
	return cmd
}

func newMsgPinCommand(g *GlobalFlags) *cobra.Command {
	var channel, message string
	cmd := &cobra.Command{
		Use:   "pin",
		Short: "Pin a message",
		Annotations: map[string]string{
			"discordEndpoint": "PUT /channels/{channel.id}/pins/{message.id}",
			"write":           "true",
			"perms":           "MANAGE_MESSAGES",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cid, err := clientAndChannel(g, channel)
			if err != nil {
				return err
			}
			path := "/channels/" + cid + "/pins/" + message
			if g.DryRun {
				dryRunPrint(cmd, "PUT", path, nil)
				return nil
			}
			if _, err := client.Do("PUT", path, nil); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "pinned %s\n", message)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().StringVar(&message, "message", "", "message ID")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("message")
	return cmd
}

func newMsgUnpinCommand(g *GlobalFlags) *cobra.Command {
	var channel, message string
	cmd := &cobra.Command{
		Use:   "unpin",
		Short: "Unpin a message",
		Annotations: map[string]string{
			"discordEndpoint": "DELETE /channels/{channel.id}/pins/{message.id}",
			"write":           "true",
			"perms":           "MANAGE_MESSAGES",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cid, err := clientAndChannel(g, channel)
			if err != nil {
				return err
			}
			path := "/channels/" + cid + "/pins/" + message
			if g.DryRun {
				dryRunPrint(cmd, "DELETE", path, nil)
				return nil
			}
			if _, err := client.Do("DELETE", path, nil); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "unpinned %s\n", message)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().StringVar(&message, "message", "", "message ID")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("message")
	return cmd
}

func newMsgPinsCommand(g *GlobalFlags) *cobra.Command {
	var channel string
	cmd := &cobra.Command{
		Use:   "pins",
		Short: "List pinned messages",
		Annotations: map[string]string{
			"discordEndpoint": "GET /channels/{channel.id}/pins",
			"intents":         "MESSAGE_CONTENT",
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
			raw, err := client.Do("GET", "/channels/"+cid+"/pins", nil)
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
			items := make([]msgItem, len(msgs))
			for i, m := range msgs {
				items[i] = renderMessage(r, m)
			}
			return emit(cmd, g, items)
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.MarkFlagRequired("channel")
	return cmd
}

func newMsgPermalinkCommand(g *GlobalFlags) *cobra.Command {
	var channel, message string
	cmd := &cobra.Command{
		Use:   "permalink",
		Short: "Print the web URL of a message",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			gid, err := guildID(g, client)
			if err != nil {
				return err
			}
			cid, err := channelID(g, client, channel)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "https://discord.com/channels/%s/%s/%s\n", gid, cid, message)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().StringVar(&message, "message", "", "message ID")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("message")
	return cmd
}
