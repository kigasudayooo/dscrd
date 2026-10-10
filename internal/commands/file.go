package commands

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// allowedCDNHosts are the only hosts `file download` will fetch from.
var allowedCDNHosts = map[string]bool{
	"cdn.discordapp.com":   true,
	"media.discordapp.net": true,
}

func newFileCommand(g *GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "file", Short: "Download message attachments"}
	cmd.AddCommand(newFileDownloadCommand(g))
	return cmd
}

// checkCDNURL admits only https URLs on the Discord CDN. DSCRD_CDN_ALLOW_HOST
// admits one extra host, over any scheme (tests).
func checkCDNURL(u *url.URL) error {
	if extra := os.Getenv("DSCRD_CDN_ALLOW_HOST"); extra != "" && u.Host == extra {
		return nil
	}
	if !allowedCDNHosts[u.Host] {
		return fmt.Errorf("refusing to download from %q (only Discord CDN hosts)", u.Host)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("refusing to download over %q (https only)", u.Scheme)
	}
	return nil
}

func newFileDownloadCommand(g *GlobalFlags) *cobra.Command {
	var out string
	var force bool
	cmd := &cobra.Command{
		Use:   "download <attachment-url>",
		Short: "Download an attachment from the Discord CDN",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := url.Parse(args[0])
			if err != nil {
				return err
			}
			if err := checkCDNURL(u); err != nil {
				return err
			}
			target := out
			if target == "" {
				target = path.Base(u.Path)
			}
			if target == "" || target == "/" || target == "." {
				return fmt.Errorf("cannot derive a file name from %q; pass --out", args[0])
			}
			if g.DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[dry-run] GET %s -> %s\n", args[0], target)
				return nil
			}
			// A redirect must not lead off the CDN either.
			client := &http.Client{
				Timeout: 5 * time.Minute,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					if len(via) >= 5 {
						return fmt.Errorf("too many redirects")
					}
					return checkCDNURL(req.URL)
				},
			}
			resp, err := client.Get(args[0])
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
			}
			// The default name comes from the URL, i.e. from whoever posted the
			// attachment: never let it replace an existing file silently.
			flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
			if force {
				flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
			}
			f, err := os.OpenFile(target, flags, 0o644)
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("%s already exists; pass --force to overwrite or --out to rename", target)
			}
			if err != nil {
				return err
			}
			n, err := io.Copy(f, resp.Body)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved %s (%d bytes)\n", strings.TrimPrefix(target, "./"), n)
			return nil
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output path (default: attachment file name)")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the output file if it already exists")
	return cmd
}
