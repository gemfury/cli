package terminal

import (
	"github.com/bgentry/go-netrc/netrc"

	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
)

// Machines for Gemfury in .netrc file
var (
	netrcMachines = []string{"api.fury.io", "git.fury.io"}
)

type Auther interface {
	Auth() (string, string, error)
	Append(string, string) error
	Wipe() error
}

func Netrc() Auther {
	return nrc{machines: netrcMachines}
}

type nrc struct {
	machines []string
}

func (n nrc) Auth() (string, string, error) {
	path, err := netrcPath()
	if err != nil {
		return "", "", err
	}

	// Load up the netrc file. ParseFile uses os.Open
	// And will return *PathError if it's not readable
	net, err := netrc.ParseFile(path)
	if os.IsNotExist(err) {
		return "", "", nil
	} else if err != nil {
		return "", "", fmt.Errorf("Error reading .netrc file %q: %w", path, err)
	}

	machine := net.FindMachine(n.machines[0])
	if machine == nil {
		return "", "", nil
	}

	return machine.Login, machine.Password, nil
}

func (n nrc) Append(user, pass string) error {
	return netrcUpdate(func(net *netrc.Netrc) {
		for _, m := range n.machines {
			net.NewMachine(m, user, pass, "")
		}
	})
}

func (n nrc) Wipe() error {
	return netrcUpdate(func(net *netrc.Netrc) {
		for _, m := range n.machines {
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
