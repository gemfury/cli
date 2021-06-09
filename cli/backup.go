package cli

import (
	"github.com/cheggaaa/pb/v3"
	"github.com/gemfury/cli/api"
	"github.com/spf13/cobra"

	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Root for sharing/collaboration subcommands
func NewCmdBackup() *cobra.Command {
	return &cobra.Command{
		Hidden: true,
		Use:    "backup DIR",
		Short:  "Save all packages to directory",
		RunE:   backupEverything,
	}
}

func backupEverything(cmd *cobra.Command, args []string) error {
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
		resp, err := c.DumpVersions(cc, pageReq)
		if err != nil {
			return nil, err
		}

		// Save each version to disk
		for _, v := range resp.Versions {
			if err := backupVersion(cc, c, v, destDir); err != nil {
				return nil, err
			}
		}

		return resp.Pagination, nil
	})
}

func backupVersion(cc context.Context, client *api.Client, v *api.Version, destDir string) error {
	slash := string(filepath.Separator)
	fName := strings.ReplaceAll(v.Filename, slash, "_")
	pName := strings.ReplaceAll(v.Package.Name, slash, "_")
	subPath := slash + v.Package.Kind + slash + pName + slash + fName
	subPath = filepath.Clean(subPath) // Final safety sanitizing
	path := filepath.Clean(filepath.Join(destDir, subPath))
	pkgDir := filepath.Dir(path)

	// Verify or create package directory
	if s, err := os.Stat(pkgDir); os.IsNotExist(err) {
		if err := os.MkdirAll(pkgDir, 0700); err != nil {
			return err
		}
	} else if err != nil || !s.IsDir() {
		return fmt.Errorf("Problem creating directory %q", pkgDir)
	}

	// Open file for writing. It must not exist.
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
	bar := pbFactory.Start64(size)
	verID := fmt.Sprintf("%-16s", v.ID)
	bar = bar.Set("prefix", verID+"⌛ "+subPath+" ")
	bar = bar.Set(pb.CleanOnFinish, true)
	reader := bar.NewProxyReader(body)

	// Download and write to disk
	_, err = io.Copy(file, reader)
	bar.Finish()

	// Status output
	if err == nil {
		fmt.Printf("%s💾 %s\n", verID, subPath)
	}

	return err
}
