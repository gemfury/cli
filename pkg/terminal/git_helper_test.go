package terminal

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// useGitConfig points the global configuration of Git at a file of the
// home directory of the test, which has the given content. That is where
// a Git that knows no GIT_CONFIG_GLOBAL looks for it.
func useGitConfig(t *testing.T, content string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is not installed")
	}

	home := useHome(t)
	path := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("GIT_CONFIG_GLOBAL", path)
	return path
}

// useFakeGit puts a shell script on PATH in the place of Git, which it has
// as $GIT, and gives Git an empty global configuration
func useFakeGit(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("No shell to run a script as Git")
	}

	useGitConfig(t, "")
	git, _ := exec.LookPath("git") // There, or useGitConfig skips the test
	script = "#!/bin/sh\nGIT=" + shellQuote(git) + "\n" + script + "\n"

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// expectGitHelpers asserts the credential helpers set for GitHost, in order
func expectGitHelpers(t *testing.T, helpers ...string) {
	t.Helper()
	if got := gitHelpers(); !slices.Equal(got, helpers) {
		t.Errorf("Expected helpers %q, got %q", helpers, got)
	}
}

// useHelperPath puts this CLI on PATH, by a link in a directory whose name
// needs quotes in a shell, and after another program of its name. The path
// of the link is the one that Git is to be given for it.
func useHelperPath(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(t.TempDir(), "on path")
	link := filepath.Join(dir, filepath.Base(exe))
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	} else if err := os.Symlink(exe, link); err != nil {
		t.Skipf("Cannot link: %s", err)
	}

	other := t.TempDir()
	if err := os.WriteFile(filepath.Join(other, filepath.Base(exe)), nil, 0700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", strings.Join([]string{other, dir, os.Getenv("PATH")}, string(os.PathListSeparator)))
	return link
}

// expectRegistered asserts that the credential helpers of GitHost are reset,
// and then this CLI
func expectRegistered(t *testing.T) {
	t.Helper()
	helpers := gitHelpers()
	if len(helpers) != 2 || helpers[0] != "" || !isGitHelper(helpers[1]) {
		t.Errorf("Expected a reset and this CLI as helpers, got %q", helpers)
	}
}

// Whatever helpers Git has for the host are replaced by this CLI
func TestGitHelperRegister(t *testing.T) {
	for name, content := range map[string]string{
		"no configuration":     "",
		"helper for all hosts": "[credential]\n\thelper = store\n",
		"helpers for the host": "[credential \"https://git.fury.io\"]\n\thelper = store\n\thelper = cache\n",
	} {
		t.Run(name, func(t *testing.T) {
			path := useGitConfig(t, content)
			gitConfig{}.Register()
			expectRegistered(t)

			// Registered again, the configuration is not written again. The
			// hard link keeps the file as it was in place on disk, so that
			// one written twice over cannot take that place, and pass for it.
			if err := os.Link(path, path+".before"); err != nil {
				t.Skipf("Cannot link: %s", err)
			}

			before, _ := os.Stat(path + ".before")
			gitConfig{}.Register()
			expectRegistered(t)
			expectReplaced(t, path, before, false)
		})
	}
}

// A configuration that Git cannot change is left for its user to change:
// by the commands that are returned, which set the helper as we would,
// with a path that needs quotes within those of the helper itself
func TestGitHelperRegisterByHand(t *testing.T) {
	path := useGitConfig(t, "")
	useHelperPath(t)

	// Git replaces the file by one that it writes next to it
	unlock := lockDir(t, filepath.Dir(path))
	setup := gitConfig{}.Register()
	expectGitHelpers(t)
	unlock()

	for _, command := range setup {
		if out, err := exec.Command("sh", "-c", command).CombinedOutput(); err != nil {
			t.Fatalf("Command %q failed: %s", command, out)
		}
	}

	// What is set by hand is what we set: registered again, it stays
	expectRegistered(t)
	byHand := gitHelpers()
	gitConfig{}.Register()
	expectGitHelpers(t, byHand...)
}

// Without Git, there is no helper to set, nor to tell of
func TestGitHelperWithoutGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if setup := (gitConfig{}).Register(); setup != nil {
		t.Errorf("Expected nothing to set, got %q", setup)
	}
}

// A Git that takes too long is given up on, and left to be set by hand
func TestGitHelperTimeout(t *testing.T) {
	useFakeGit(t, "sleep 5")
	setDuring(t, &gitTimeout, 50*time.Millisecond)

	start := time.Now()
	if setup := (gitConfig{}).Register(); len(setup) != 2 {
		t.Errorf("Expected two commands, got %q", setup)
	} else if took := time.Since(start); took > 2*time.Second {
		t.Errorf("Expected Git to be given up on, waited %s", took)
	}
}

// A helper that Git fails to add is not left as the reset alone,
// which would keep Git from every helper for the host
func TestGitHelperRegisterHalfway(t *testing.T) {
	useFakeGit(t, `case "$*" in *--add*) exit 1 ;; esac; exec "$GIT" "$@"`)

	if setup := (gitConfig{}).Register(); len(setup) != 2 {
		t.Errorf("Expected two commands, got %q", setup)
	}
	expectGitHelpers(t)
}

// A wrapper of Git may leave a process behind, which keeps its output open
// for longer than it is waited for: Git has set the helper all the same
func TestGitHelperWrapper(t *testing.T) {
	useFakeGit(t, `"$GIT" "$@"; status=$?; case "$*" in *--add*) sleep 2 & ;; esac; exit $status`)

	if setup := (gitConfig{}).Register(); setup != nil {
		t.Errorf("Expected nothing to set by hand, got %q", setup)
	}
	expectRegistered(t)
}

// The helper is this CLI by the path that it has on PATH, quoted for
// the shell, rather than by the path of the executable itself
func TestGitHelperPath(t *testing.T) {
	useGitConfig(t, "")
	link := useHelperPath(t)

	gitConfig{}.Register()
	expectGitHelpers(t, "", "!'"+filepath.ToSlash(link)+"' "+GitCredentialHelper)
}

func TestGitHelperUnregister(t *testing.T) {
	useGitConfig(t, "[credential]\n\thelper = store\n")
	gitConfig{}.Register()
	gitConfig{}.Unregister()
	expectGitHelpers(t)

	// The helper for all hosts is not ours to remove
	out, err := gitConfigOutput("--get-all", "credential.helper")
	if err != nil || out != "store\n" {
		t.Errorf("Expected the helper for all hosts to stay, got %q and: %v", out, err)
	}
}

// A helper that the user has set for the host is kept, with ours not set
func TestGitHelperUnregisterOthers(t *testing.T) {
	for _, helper := range []string{"store", "!pass show fury"} {
		useGitConfig(t, "[credential \"https://git.fury.io\"]\n\thelper = "+helper+"\n")
		gitConfig{}.Unregister()
		expectGitHelpers(t, helper)
	}
}

func TestShellQuote(t *testing.T) {
	for path, exp := range map[string]string{
		"/opt/homebrew/bin/fury":       "/opt/homebrew/bin/fury",
		"C:/Users/me/fury.exe":         "C:/Users/me/fury.exe",
		"/Users/my name/bin/fury":      "'/Users/my name/bin/fury'",
		"/Users/o'brien/$HOME/fury":    `'/Users/o'\''brien/$HOME/fury'`,
		`C:\Program Files\fury\fury`:   `'C:\Program Files\fury\fury'`,
		"/usr/local/bin/fury; rm -rf~": "'/usr/local/bin/fury; rm -rf~'",
	} {
		if got := shellQuote(path); got != exp {
			t.Errorf("Expected %q quoted as %q, got %q", path, exp, got)
		}
	}
}
