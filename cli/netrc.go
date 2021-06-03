package cli

import (
	"github.com/bgentry/go-netrc/netrc"
	"github.com/gemfury/cli/api"

	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
)

func newAPIClient(cc context.Context) (c *api.Client, err error) {
	flags := ctxGlobalFlags(cc)

	// Token comes from CLI flags or .netrc
	token := flags.AuthToken
	if token == "" {
		token, err = netrcAuth()
		if err != nil {
			return nil, err
		}
	}

	c = api.NewClient(token, flags.Account)
	return c, nil
}

func netrcAuth() (string, error) {
	path, err := netrcPath()
	if err != nil {
		return "", err
	}

	// Load up the netrc file
	net, err := netrc.ParseFile(path)
	if err != nil {
		return "", fmt.Errorf("Error reading .netrc file %q: %w", path, err)
	}

	machine := net.FindMachine("api.fury.io")
	if machine == nil {
		return "", nil
	}

	return machine.Password, nil
}

func netrcAppend(machines []string, user, pass string) error {
	return netrcUpdate(func(net *netrc.Netrc) {
		for _, m := range machines {
			net.NewMachine(m, user, pass, "")
		}
	})
}

func netrcWipe(machines []string) error {
	return netrcUpdate(func(net *netrc.Netrc) {
		for _, m := range machines {
			net.RemoveMachine(m)
		}
	})
}

func netrcUpdate(update func(net *netrc.Netrc)) error {
	path, err := netrcPath()
	if err != nil {
		return err
	}

	// Load up the .netrc file
	net, err := netrc.ParseFile(path)
	if err != nil {
		return fmt.Errorf("Error reading .netrc %q: %w", path, err)
	}

	// Apply updates
	update(net)

	// Write new .netrc file
	out, _ := net.MarshalText()
	out = bytes.TrimSpace(out)
	return ioutil.WriteFile(path, out, 0600)
}

func netrcPath() (string, error) {

	if path := os.Getenv("NETRC"); path != "" {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	if runtime.GOOS == "windows" {
		return filepath.Join(home, "_netrc"), nil
	}

	return filepath.Join(home, ".netrc"), nil

}
