package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"context"
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	backupSkip = fmt.Errorf("Skip file")
)

// NewCmdBeta creates a Cobra command for "beta"
func NewCmdBeta() *cobra.Command {
	betaCmd := &cobra.Command{
		Hidden: true,
		Use:    "beta",
		Short:  "Experimental features",
	}

	betaCmd.AddCommand(NewCmdBackup())
	betaCmd.AddCommand(NewCmdDownload())

	return betaCmd
}

// NewCmdDownload creates a Cobra command for "download"
func NewCmdDownload() *cobra.Command {
	return &cobra.Command{
		Use:   "download PACKAGE@VERSION...",
		Short: "Download a package to the current directory",
		Args:  usageArgs(cobra.MinimumNArgs(1), "Please specify at least one PACKAGE@VERSION"),
		RunE:  downloadVersions,
	}
}

func downloadVersions(cmd *cobra.Command, args []string) error {
	cc := cmd.Context()
	term := ctx.Terminal(cc)
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	fails := newFailures(term, len(args), "downloads")
	for _, arg := range args {
		fails.add("downloading", arg, downloadArg(cc, c, arg))
	}

	return fails.err()
}

// downloadArg resolves a PACKAGE@VERSION argument and
// downloads its file into the current directory
func downloadArg(cc context.Context, c *api.Client, arg string) error {
	at := strings.LastIndex(arg, "@")
	if at <= 0 {
		return fmt.Errorf("Argument format: PACKAGE@VERSION")
	}
	pkg, ver := arg[0:at], arg[at+1:]

	v, err := c.Version(cc, pkg, ver)
	if err != nil {
		return err
	}

	filename := strings.ReplaceAll(v.Filename, string(filepath.Separator), "_")
	return downloadVersion(cc, c, v, ".", filename)
}

// NewCmdBackup creates a Cobra command for "backup"
func NewCmdBackup() *cobra.Command {
	var kindFlag string

	backupCmd := &cobra.Command{
		Use:   "backup DIR",
		Short: "Save all files to a directory",
		Args:  usageArgs(cobra.ExactArgs(1), "Please specify exactly one destination directory"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return backupEverything(cmd, args, kindFlag)
		},
	}

	// Flags and options
	backupCmd.Flags().StringVar(&kindFlag, "kind", "", "Filter to one kind of package")

	return backupCmd
}

func backupEverything(cmd *cobra.Command, args []string, kindFlag string) error {
	// Verify destination directory
	destDir := filepath.Clean(args[0])
	if s, err := os.Stat(destDir); os.IsNotExist(err) {
		return fmt.Errorf("This directory doesn't exist")
	} else if !s.IsDir() {
		return fmt.Errorf("This is not a directory")
	}

	// Fire up the API
	cc := cmd.Context()
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	// Paginate over package listings until no more pages
	return iterateAll(cc, false, func(pageReq *api.PaginationRequest) (*api.PaginationResponse, error) {
		resp, err := c.DumpVersions(cc, pageReq, kindFlag)
		if err != nil {
			return nil, err
		}

		// Save each version to disk
		for _, v := range resp.Versions {
			if kindFlag != "" && kindFlag != v.Package.Kind {
				continue
			}

			if err := backupVersion(cc, c, v, destDir); err != nil {
				return nil, err
			}
		}

		return resp.Pagination, nil
	})
}

func backupVersion(cc context.Context, client *api.Client, v *api.Version, destDir string) error {
	slash := string(filepath.Separator)
	pkgName := strings.ReplaceAll(v.Package.Name, slash, "_")
	fileName := strings.ReplaceAll(v.ID+"_"+v.Filename, slash, "_")
	subPath := slash + v.Package.Kind + slash + pkgName + slash + fileName
	return downloadVersion(cc, client, v, destDir, filepath.Clean(subPath))
}

func downloadVersion(cc context.Context, client *api.Client, v *api.Version, destDir, subPath string) error {
	slash := string(filepath.Separator)
	term := ctx.Terminal(cc)

	path := filepath.Clean(filepath.Join(destDir, subPath))
	pkgDir := filepath.Dir(path)

	// Status line with a slot for the status emoji
	status := func(emoji string) string {
		return fmt.Sprintf("%-16s%s %s", v.ID, emoji, strings.TrimPrefix(subPath, slash))
	}

	// Verify or create package directory
	if s, err := os.Stat(pkgDir); os.IsNotExist(err) {
		if err := os.MkdirAll(pkgDir, 0700); err != nil {
			return err
		}
	} else if err != nil || !s.IsDir() {
		return fmt.Errorf("Problem creating directory %q", pkgDir)
	}

	// Check if file exists, and validate checksum
	if err := backupCheckPath(term, v, path, status); errors.Is(err, backupSkip) {
		return nil // Checksum match => skip download
	} else if err != nil {
		return err
	}

	// Open file for writing. It must not exist, otherwise fail
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	// Request file from Gemfury API
	body, size, err := client.DownloadVersion(cc, v)
	if err != nil {
		return err
	}
	defer body.Close()

	// Wrap with status bar
	bar := term.StartProgress(size, status("⌛")+" ")
	reader := bar.NewProxyReader(body)

	// Download and write to disk
	_, err = io.Copy(file, reader)
	bar.Finish()

	// Status output
	if err == nil {
		term.Println(status("💾"))
	}

	return err
}

// Validate checksum for file
func backupCheckPath(term terminal.Terminal, v *api.Version, path string, status func(string) string) error {
	// Check if file exists, and validate checksum if it does
	if s, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	} else if s.IsDir() {
		return fmt.Errorf("Dir exists: %s", path)
	}

	if v == nil || v.Digests.SHA512 == "" {
		term.Printf("%s (WARNING: No checksum provided by API)\n", status("❓"))
		return backupSkip // API should always have digests (theoretically)
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha512.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	sum := fmt.Sprintf("%x", hash.Sum(nil))
	if exp := v.Digests.SHA512; exp != sum {
		term.Printf("%s (CHECKSUM MISMATCH)\n", status("❌"))
		prompt := promptui.Prompt{
			Label:   "Do you want to delete and redownload? [y/N]",
			Default: "N",
		}

		result, err := term.RunPrompt(&prompt)
		if err != nil {
			return err
		} else if result == "Y" || result == "y" {
			file.Close()
			os.Remove(path)
			return nil
		}

		return fmt.Errorf("Checksum failed")
	}

	term.Println(status("✅"))
	return backupSkip
}
