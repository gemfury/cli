package cli

import (
	"github.com/gemfury/cli/api"
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

	return betaCmd
}

// NewCmdBackup creates a Cobra command for "backup"
func NewCmdBackup() *cobra.Command {
	var kindFlag string

	backupCmd := &cobra.Command{
		Use:   "backup DIR",
		Short: "Save all files to a directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return backupEverything(cmd, args, kindFlag)
		},
	}

	// Flags and options
	backupCmd.Flags().StringVar(&kindFlag, "kind", "", "Filter to one kind of package")

	return backupCmd
}

func backupEverything(cmd *cobra.Command, args []string, kindFlag string) error {
	if len(args) != 1 {
		return fmt.Errorf("Please specify the destination")
	}

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
	term := ctxTerminal(cc)

	slash := string(filepath.Separator)
	pkgName := strings.ReplaceAll(v.Package.Name, slash, "_")
	fileName := strings.ReplaceAll(v.ID+"_"+v.Filename, slash, "_")
	subPath := slash + v.Package.Kind + slash + pkgName + slash + fileName

	// Sanitize and join destDir
	subPath = filepath.Clean(subPath)
	path := filepath.Clean(filepath.Join(destDir, subPath))
	pkgDir := filepath.Dir(path)

	// Status string template for inserting status emoji
	statusFmt := fmt.Sprintf("%-16s%%s %s", v.ID, strings.TrimPrefix(subPath, slash))

	// Verify or create package directory
	if s, err := os.Stat(pkgDir); os.IsNotExist(err) {
		if err := os.MkdirAll(pkgDir, 0700); err != nil {
			return err
		}
	} else if err != nil || !s.IsDir() {
		return fmt.Errorf("Problem creating directory %q", pkgDir)
	}

	// Check if file exists, and validate checksum
	if err := backupCheckPath(term, v, path, statusFmt); errors.Is(err, backupSkip) {
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
	bar := term.StartProgress(size, fmt.Sprintf(statusFmt+" ", "⌛"))
	reader := bar.NewProxyReader(body)

	// Download and write to disk
	_, err = io.Copy(file, reader)
	bar.Finish()

	// Status output
	if err == nil {
		term.Printf(statusFmt+"\n", "💾")
	}

	return err
}

// Validate checksum for file
func backupCheckPath(term terminal.Terminal, v *api.Version, path, statusFmt string) error {
	// Check if file exists, and validate checksum if it does
	if s, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	} else if s.IsDir() {
		return fmt.Errorf("Dir exists: %s", path)
	}

	if v == nil || v.Digests.SHA512 == "" {
		term.Printf(statusFmt+" (WARNING: No checksum provided by API)\n", "❓")
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
		term.Printf(statusFmt+" (CHECKSUM MISMATCH)\n", "❌")
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

	term.Printf(statusFmt+"\n", "✅")
	return backupSkip
}
