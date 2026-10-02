package terminal

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"
)

// GitHost is the host of the Git repositories of Gemfury
const GitHost = "git.fury.io"

// GitCredentialHelper is the command of this CLI that Git runs as
// its credential helper for GitHost: see gitcredentials(7)
const GitCredentialHelper = "git credentials"

// gitHelperKey is the Git setting for the credential helpers of GitHost
const gitHelperKey = "credential.https://" + GitHost + ".helper"

// gitTimeout is how long Git has to read or to change its configuration.
// It is a variable for the tests of a Git that takes longer.
var gitTimeout = 5 * time.Second

// gitHelper sets this CLI as the credential helper of Git for GitHost,
// and unsets it. Without Git, there is nothing to set, nor to report.
type gitHelper interface {
	// Register returns the commands that set the helper by hand, where
	// Git did not let us: its configuration is not always ours to write
	Register() []string
	Unregister()
}

// gitConfig is the gitHelper of the global configuration of Git
type gitConfig struct{}

func (gitConfig) Register() []string {
	exe, err := helperExecutable()
	if err != nil {
		return nil
	}

	// The empty value comes first to reset the list of helpers for this
	// host. Without it, a helper that is set for all hosts is asked before
	// ours, and answers with what it has stored.
	helpers := []string{"", "!" + shellQuote(filepath.ToSlash(exe)) + " " + GitCredentialHelper}
	if slices.Equal(gitHelpers(), helpers) {
		return nil
	}

	err = runGitConfig("--replace-all", gitHelperKey, helpers[0])
	if err == nil {
		// The reset alone would leave Git without a helper for this host
		if err = runGitConfig("--add", gitHelperKey, helpers[1]); err != nil {
			runGitConfig("--unset-all", gitHelperKey)
		}
	}

	if err == nil || errors.Is(err, exec.ErrNotFound) {
		return nil
	}

	return []string{
		"git config --global --replace-all " + gitHelperKey + ` ""`,
		"git config --global --add " + gitHelperKey + " " + shellQuote(helpers[1]),
	}
}

func (gitConfig) Unregister() {
	// Without ours among them, the helpers of this host are not ours to unset
	if slices.ContainsFunc(gitHelpers(), isGitHelper) {
		runGitConfig("--unset-all", gitHelperKey)
	}
}

// isGitHelper reports whether a credential helper of Git is this CLI
func isGitHelper(helper string) bool {
	return strings.HasPrefix(helper, "!") && strings.HasSuffix(helper, " "+GitCredentialHelper)
}

// gitHelpers lists the credential helpers set for GitHost, in order
func gitHelpers() []string {
	out, err := gitConfigOutput("--null", "--get-all", gitHelperKey)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
}

func runGitConfig(args ...string) error {
	_, err := gitConfigOutput(args...)
	return err
}

func gitConfigOutput(args ...string) (string, error) {
	cc, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	// Once out of time, Git is asked to end, and killed only if it does not:
	// killed at once, it may leave its configuration locked. A process that
	// outlives it, as that of a wrapper script does, may hold its output
	// open: it is not waited for.
	cmd := exec.CommandContext(cc, "git", append([]string{"config", "--global"}, args...)...)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = gitTimeout / 5

	// Git has done its part where only such a process has not
	out, err := cmd.Output()
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	return string(out), err
}

// helperExecutable is the path of this CLI, for Git to run. The path that it
// has on PATH is preferred to that of the executable itself, which Homebrew
// keeps in a directory of its version, and removes on upgrade.
func helperExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	exe = filepath.Clean(exe)
	exeInfo, err := os.Stat(exe)
	if err != nil {
		return "", err
	}

	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		path, err := filepath.Abs(filepath.Join(dir, filepath.Base(exe)))
		if err != nil {
			continue
		}
		if info, err := os.Stat(path); err == nil && os.SameFile(info, exeInfo) {
			return path, nil
		}
	}

	return exe, nil
}

// shellSafeRegexp matches what needs no quotes in a shell
var shellSafeRegexp = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellQuote quotes a word for a POSIX shell: the path of this CLI for the
// shell that Git runs its helpers with, and the helper as a whole for the
// shell of whoever sets it by hand
func shellQuote(word string) string {
	if shellSafeRegexp.MatchString(word) {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}
