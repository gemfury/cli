package terminal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

// useHome makes a temporary directory the home directory, so that
// no test gets to the files of whoever runs it
func useHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Home directory on Windows
	return home
}

// lockDir keeps a directory from taking another file, until the function
// that it returns is called, or else until the test ends. The test is
// skipped where the permissions of a directory do not keep files out.
func lockDir(t *testing.T, dir string) (unlock func()) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("Directories take a file whatever their permissions")
	}

	if err := os.Chmod(dir, 0500); err != nil {
		t.Fatal(err)
	}

	unlock = func() { os.Chmod(dir, 0700) }
	t.Cleanup(unlock)
	return unlock
}

// useNetrc points NETRC at a file of the home directory of the test, which
// has the given content, or which does not exist when content is empty.
// The FURY_NETRC_WRITE of whoever runs the test is cleared.
func useNetrc(t *testing.T, content string) string {
	t.Helper()
	t.Setenv(netrcWriteEnv, "")

	path := filepath.Join(useHome(t), "netrc")
	if content != "" {
		writeNetrc(t, path, content)
	}
	t.Setenv("NETRC", path)
	return path
}

// writeNetrc writes a file that others can read, whatever the umask, which
// is not how a .netrc should be, nor how it is left once it is written
func writeNetrc(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	} else if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
}

// setDuring sets a variable for the duration of a test
func setDuring[T any](t *testing.T, v *T, to T) {
	t.Helper()
	was := *v
	*v = to
	t.Cleanup(func() { *v = was })
}

// saveSession saves the session of furyEntry, which is then the one read
func saveSession(t *testing.T, auth Auther) {
	t.Helper()
	if err := auth.Append("u@example.com", "abc123"); err != nil {
		t.Fatal(err)
	}
	expectAuth(t, auth, "u@example.com", "abc123")
}

// wipeSession wipes the session, after which none is read
func wipeSession(t *testing.T, auth Auther) {
	t.Helper()
	if err := auth.Wipe(); err != nil {
		t.Fatal(err)
	}
	expectAuth(t, auth, "", "")
}

// expectAuth asserts what credentials are saved, both empty for none
func expectAuth(t *testing.T, auth Auther, user, pass string) {
	t.Helper()
	if u, p, err := auth.Auth(); err != nil {
		t.Errorf("Auth error: %s", err)
	} else if u != user || p != pass {
		t.Errorf("Expected saved credentials %q/%q, got %q/%q", user, pass, u, p)
	}
}

// expectUpdateFails asserts that saving a session and wiping it
// both fail with the given message
func expectUpdateFails(t *testing.T, msg string) {
	t.Helper()
	for op, err := range map[string]error{
		"Append": Netrc().Append("u@example.com", "other"),
		"Wipe":   Netrc().Wipe(),
	} {
		if err == nil || err.Error() != msg {
			t.Errorf("Expected %s to fail with %q, got: %v", op, msg, err)
		}
	}
}

// expectFile asserts the content of a file, and its permissions where
// the system has them
func expectFile(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	if out, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	} else if string(out) != content {
		t.Errorf("Expected %q in %s, got %q", content, path, out)
	}

	if runtime.GOOS == "windows" {
		return
	}

	if info, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if got := info.Mode().Perm(); got != perm {
		t.Errorf("Expected permissions %o for %s, got %o", perm, path, got)
	}
}

// expectReplaced asserts whether path is now a different file than before,
// rather than the same file rewritten. Without a file before, it is neither.
func expectReplaced(t *testing.T, path string, before os.FileInfo, replaced bool) {
	t.Helper()
	if before == nil {
		return
	}

	if after, err := os.Stat(path); err != nil {
		t.Fatal(err)
	} else if os.SameFile(before, after) == replaced {
		t.Errorf("Expected replaced=%v for %s", replaced, path)
	}
}

// expectNextTo asserts that a temporary file is in the directory of the
// file that it is to replace, or the rename may be to another filesystem
func expectNextTo(t *testing.T, tmp, path string) {
	t.Helper()
	if filepath.Dir(tmp) != filepath.Dir(path) {
		t.Errorf("Expected %s to be next to %s", tmp, path)
	}
}

// expectAlone asserts that the file has its directory to itself, with
// nothing left behind by its replacement
func expectAlone(t *testing.T, path string) {
	t.Helper()
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("Expected only the .netrc file, got %d files", len(entries))
	}
}

// The entries of a .netrc file
const (
	otherEntry = "machine example.com\n\tlogin o@example.com\n\tpassword other\n"
	furyEntry  = "machine api.fury.io\n\tlogin u@example.com\n\tpassword abc123\n" +
		"machine git.fury.io\n\tlogin u@example.com\n\tpassword abc123\n"

	// What the file has once a session is saved next to otherEntry
	savedEntries = otherEntry + furyEntry

	// The entry for every host that has none of its own, which comes last
	defaultEntry = "default\n\tlogin d@example.com\n\tpassword other\n"

	// Entries of Gemfury that a session was not saved by: one has no
	// password, one is of Git alone, and one is there for the second time
	staleEntries = "machine api.fury.io\n\tlogin old@example.com\n" + otherEntry +
		"machine git.fury.io\n\tlogin old@example.com\n\tpassword old\n" +
		"machine api.fury.io\n\tlogin older@example.com\n\tpassword older\n"

	// An entry that does not end its line, as the last one of a file may
	openEntry = "machine example.com login o@example.com password other"

	// A macro, which an empty line would end
	macroEntry = "macdef init\ncd /pub\nls"
)

// A saved session replaces the entries that Gemfury has, however many, and
// wiping removes them. The entries of other hosts stay as they were. The
// file is replaced by one that only its owner can read, whatever it was
// before, and that ends with a single line break.
func TestNetrcUpdate(t *testing.T) {
	for name, tc := range map[string]struct {
		before, saved, wiped string
	}{
		"no file":        {"", furyEntry, ""},
		"other hosts":    {otherEntry, savedEntries, otherEntry},
		"stale entries":  {staleEntries, savedEntries, otherEntry},
		"line not ended": {openEntry, openEntry + "\n" + furyEntry, openEntry + "\n"},
		"empty lines":    {otherEntry + "\n\n", savedEntries, otherEntry},
		"before default": {otherEntry + defaultEntry, savedEntries + defaultEntry, otherEntry + defaultEntry},
		"default only":   {defaultEntry, furyEntry + defaultEntry, defaultEntry},
	} {
		t.Run(name, func(t *testing.T) {
			path := useNetrc(t, tc.before)
			before, _ := os.Stat(path)

			saveSession(t, Netrc())
			expectFile(t, path, tc.saved, 0600)
			expectReplaced(t, path, before, true)

			wipeSession(t, Netrc())
			expectFile(t, path, tc.wiped, 0600)
			expectAlone(t, path)
		})
	}
}

// A .netrc that links to a file elsewhere stays a link to it,
// and it is that file which is replaced
func TestNetrcUpdateThroughLink(t *testing.T) {
	target := filepath.Join(t.TempDir(), "dotfiles-netrc")
	writeNetrc(t, target, otherEntry)
	before, _ := os.Stat(target)

	// The link is relative, as from a dotfiles directory
	link := useNetrc(t, "")
	if rel, err := filepath.Rel(filepath.Dir(link), target); err != nil {
		t.Fatal(err)
	} else if err := os.Symlink(rel, link); err != nil {
		t.Skipf("Cannot link: %s", err)
	}

	setDuring(t, &renameFile, func(tmp, path string) error {
		expectNextTo(t, tmp, path)
		return os.Rename(tmp, path)
	})

	saveSession(t, Netrc())
	expectFile(t, target, savedEntries, 0600)
	expectReplaced(t, target, before, true)

	if info, err := os.Lstat(link); err != nil {
		t.Fatal(err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("Expected %s to remain a link", link)
	}
}

// Where the file cannot be replaced, it is written in place,
// and nothing is left next to it
func TestNetrcUpdateInPlace(t *testing.T) {
	notRenamed := func(t *testing.T, _ string) {
		setDuring(t, &renameFile, func(tmp, path string) error {
			expectNextTo(t, tmp, path)
			return errors.New("busy")
		})
	}

	for name, tc := range map[string]struct {
		before string
		setup  func(t *testing.T, path string)
	}{
		// The directory takes no other file: for want of permission,
		// or as its filesystem is read-only
		"no permission": {otherEntry, func(t *testing.T, path string) {
			lockDir(t, filepath.Dir(path))
		}},

		"read-only": {otherEntry, func(t *testing.T, _ string) {
			setDuring(t, &createTemp, func(dir, pattern string) (*os.File, error) {
				return nil, &os.PathError{Op: "open", Path: dir, Err: syscall.EROFS}
			})
		}},

		// The file is not renamed over, as one that is mounted alone into
		// a container
		"not renamed":          {otherEntry, notRenamed},
		"not renamed, no file": {"", notRenamed},
	} {
		t.Run(name, func(t *testing.T) {
			path := useNetrc(t, tc.before)
			before, _ := os.Stat(path)
			tc.setup(t, path)

			saveSession(t, Netrc())
			expectFile(t, path, tc.before+furyEntry, 0600)
			expectReplaced(t, path, before, false)
			expectAlone(t, path)
		})
	}
}

// A temporary file that is not created, or not written, is an error,
// and the file is left as it was
func TestNetrcUpdateFails(t *testing.T) {
	for name, fail := range map[string]func(dir, pattern string) (*os.File, error){
		"no temporary file": func(dir, pattern string) (*os.File, error) {
			return nil, errors.New("no space left on device")
		},

		// Only to read from, so that writing alone fails, as on a full disk
		"temporary file not written": func(dir, pattern string) (*os.File, error) {
			tmp, err := os.CreateTemp(dir, pattern)
			if err != nil {
				return nil, err
			}
			tmp.Close()
			return os.Open(tmp.Name())
		},
	} {
		t.Run(name, func(t *testing.T) {
			path := useNetrc(t, otherEntry)
			setDuring(t, &createTemp, fail)

			if err := Netrc().Append("u@example.com", "abc123"); err == nil {
				t.Error("Expected an error")
			}

			expectFile(t, path, otherEntry, 0644)
			expectAlone(t, path)
		})
	}
}

// A file that has a macro is read, and never written, wherever the macro
// is in it. The error says what to do, as by then a login has its token,
// and a logout has revoked the one that stays in the file.
func TestNetrcUpdateMacro(t *testing.T) {
	for name, content := range map[string]string{
		"ends the file":      savedEntries + macroEntry,
		"before a line":      savedEntries + macroEntry + "\n\n",
		"before the session": otherEntry + macroEntry + "\n\n" + furyEntry,
	} {
		t.Run(name, func(t *testing.T) {
			path := useNetrc(t, content)
			expectAuth(t, Netrc(), "u@example.com", "abc123")

			expectUpdateFails(t, fmt.Sprintf("Error updating .netrc file %q: its macros (macdef) would not be kept. "+
				"Edit its entries for api.fury.io and git.fury.io by hand, or set FURY_TOKEN to authenticate instead.", path))
			expectFile(t, path, content, 0644)
		})
	}
}

// Wiping removes every entry that Gemfury has, and not only the first of each
func TestNetrcWipe(t *testing.T) {
	path := useNetrc(t, staleEntries)
	wipeSession(t, Netrc())
	expectFile(t, path, otherEntry, 0600)
}

// A .netrc that cannot be read is an error, and not the lack of a session:
// saving one would replace the file
func TestNetrcUnreadable(t *testing.T) {
	expectUnreadable := func(t *testing.T, path, cause string) {
		t.Helper()
		msg := fmt.Sprintf("Error reading .netrc file %q: %s", path, cause)
		if _, _, err := Netrc().Auth(); err == nil || err.Error() != msg {
			t.Errorf("Expected Auth to fail with %q, got: %v", msg, err)
		}
		expectUpdateFails(t, msg)
	}

	t.Run("not a file", func(t *testing.T) {
		path := useNetrc(t, "")
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}

		_, cause := os.ReadFile(path)
		expectUnreadable(t, path, cause.Error())
	})

	// The line is the one of the file, also where it
	// starts with "default", or with an empty line
	for line, content := range map[string]string{
		"line 4": defaultEntry + "bogus",
		"line 5": "\n" + otherEntry + "bogus",
	} {
		t.Run("malformed at "+line, func(t *testing.T) {
			expectUnreadable(t, useNetrc(t, content), line+": keyword expected; got bogus")
		})
	}
}

// Without NETRC, the file is the one of the home directory,
// where Git and others look for it
func TestNetrcPath(t *testing.T) {
	home := filepath.Dir(useNetrc(t, ""))
	t.Setenv("NETRC", "")

	name := ".netrc"
	if runtime.GOOS == "windows" {
		name = "_netrc"
	}

	saveSession(t, Netrc())
	expectFile(t, filepath.Join(home, name), furyEntry, 0600)
}

// What is read from the files that no other test writes
func TestNetrcAuth(t *testing.T) {
	for name, tc := range map[string]struct {
		content, user, pass string
	}{
		"no file":     {"", "", ""},
		"of Git only": {"machine git.fury.io login u@example.com password abc123", "", ""},

		// The first entry of a machine is the one that is read
		"entry twice": {furyEntry + "machine api.fury.io login old@example.com password old", "u@example.com", "abc123"},
	} {
		t.Run(name, func(t *testing.T) {
			useNetrc(t, tc.content)
			expectAuth(t, Netrc(), tc.user, tc.pass)
		})
	}
}
