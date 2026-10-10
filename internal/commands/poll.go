package commands

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func newPollCommand(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "poll", Short: "Create and read polls"}
	cmd.AddCommand(newPollCreateCommand(g), newPollResultsCommand(g), newPollEndCommand(g))
	return cmd
}

func newPollCreateCommand(g *GlobalFlags) *cobra.Command {
	var channel, question string
	var answers []string
	var hours int
	var multi bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Post a poll to a channel",
		Annotations: map[string]string{
			"discordEndpoint": "POST /channels/{channel.id}/messages",
			"write":           "true",
			"perms":           "SEND_MESSAGES",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(answers) < 2 {
				return fmt.Errorf("a poll needs at least two --answer values")
			}
			pollAnswers := make([]map[string]any, len(answers))
			for i, a := range answers {
				pollAnswers[i] = map[string]any{"poll_media": map[string]string{"text": a}}
			}
			body := map[string]any{
				"poll": map[string]any{
					"question":          map[string]string{"text": question},
					"answers":           pollAnswers,
					"duration":          hours,
					"allow_multiselect": multi,
				},
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
			fmt.Fprintf(cmd.OutOrStdout(), "poll created %s\n", sent.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().StringVar(&question, "question", "", "poll question")
	cmd.Flags().StringArrayVar(&answers, "answer", nil, "poll answer (repeat 2-10 times)")
	cmd.Flags().IntVar(&hours, "duration", 24, "poll duration in hours")
	cmd.Flags().BoolVar(&multi, "multi", false, "allow selecting multiple answers")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("question")
	return cmd
}

func newPollResultsCommand(g *GlobalFlags) *cobra.Command {
	var channel, message, answer string
	cmd := &cobra.Command{
		Use:   "results",
		Short: "List voters for one poll answer",
		Annotations: map[string]string{
			"discordEndpoint": "GET /channels/{channel.id}/polls/{message.id}/answers/{answer.id}",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if n, err := strconv.Atoi(answer); err != nil || n < 1 {
				return fmt.Errorf("--answer must be a 1-based answer position, got %q", answer)
			}
			client, err := buildClient(g)
			if err != nil {
				return err
			}
			cid, err := channelID(g, client, channel)
			if err != nil {
				return err
			}
			raw, err := client.Do("GET", "/channels/"+cid+"/polls/"+message+"/answers/"+answer, nil)
			if err != nil {
				return err
			}
			if g.Raw {
				fmt.Fprintln(cmd.OutOrStdout(), string(raw))
				return nil
			}
			var resp struct {
				Users []struct {
					ID         string `json:"id"`
					Username   string `json:"username"`
					GlobalName string `json:"global_name"`
				} `json:"users"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				return err
			}
			items := make([]userItem, len(resp.Users))
			for i, u := range resp.Users {
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
	cmd.Flags().StringVar(&message, "message", "", "poll message ID")
	cmd.Flags().StringVar(&answer, "answer", "", "answer ID (1-based position)")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("message")
	cmd.MarkFlagRequired("answer")
	return cmd
}

func newPollEndCommand(g *GlobalFlags) *cobra.Command {
	var channel, message string
	cmd := &cobra.Command{
		Use:   "end",
		Short: "End a poll immediately",
		Annotations: map[string]string{
			"discordEndpoint": "POST /channels/{channel.id}/polls/{message.id}/expire",
			"write":           "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, cid, err := clientAndChannel(g, channel)
			if err != nil {
				return err
			}
			path := "/channels/" + cid + "/polls/" + message + "/expire"
			if g.DryRun {
				dryRunPrint(cmd, "POST", path, nil)
				return nil
			}
			if _, err := client.Do("POST", path, nil); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "poll ended %s\n", message)
			return nil
		},
	}
	cmd.Flags().StringVar(&channel, "channel", "", "channel ID or name")
	cmd.Flags().StringVar(&message, "message", "", "poll message ID")
	cmd.MarkFlagRequired("channel")
	cmd.MarkFlagRequired("message")
	return cmd
}
