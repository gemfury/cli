package terminal

import (
	"github.com/zalando/go-keyring"

	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// What the keychain holds for the session of furyEntry
const keyringItem = `{"user":"u@example.com","token":"abc123"}`

// fakeRing is a keychain in memory, which counts what it is asked. With err
// it fails whatever it is asked, and with deleteErr only to remove; with
// hang it never answers; with gate it answers once that is closed.
type fakeRing struct {
	item      string
	err       error
	deleteErr error
	hang      bool
	gate      chan struct{}
	asked     atomic.Int32
}

func (r *fakeRing) ask() error {
	r.asked.Add(1)
	if r.hang {
		select {}
	} else if r.gate != nil {
		<-r.gate
	}
	return r.err
}

func (r *fakeRing) Set(item string) error {
	if err := r.ask(); err != nil {
		return err
	}
	r.item = item
	return nil
}

func (r *fakeRing) Get() (string, error) {
	if err := r.ask(); err != nil {
		return "", err
	} else if r.item == "" {
		return "", keyring.ErrNotFound
	}
	return r.item, nil
}

func (r *fakeRing) Delete() error {
	if err := r.ask(); err != nil {
		return err
	} else if r.deleteErr != nil {
		return r.deleteErr
	} else if r.item == "" {
		return keyring.ErrNotFound
	}
	r.item = ""
	return nil
}

// fakeGit tells whether Git has this CLI as its credential helper. With
// setup, it cannot be given one, and that is how to do so by hand.
type fakeGit struct {
	registered bool
	setup      []string
}

func (g *fakeGit) Register() []string {
	g.registered = g.setup == nil
	return g.setup
}

func (g *fakeGit) Unregister() { g.registered = false }

// hangTimeout is how long a keychain that never answers is waited for
const hangTimeout = 10 * time.Millisecond

// testStore is a credStore of the given keychain, and of the .netrc that
// useNetrc has set. A keychain that answers has more time than it could
// need, and no wait for it is told of.
func testStore(ring *fakeRing) (*credStore, *fakeGit) {
	timeout := time.Minute
	if ring.hang {
		timeout = hangTimeout
	}

	git := &fakeGit{}
	return &credStore{
		netrc:     Netrc(),
		ring:      ring,
		git:       git,
		reachable: func() bool { return true },
		timeout:   timeout,
	}, git
}

// expectNoFile asserts that there is no file at path
func expectNoFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected no file at %s, got: %v", path, err)
	}
}

// expectKeychain asserts what the keychain holds, which is empty for nothing
func expectKeychain(t *testing.T, ring *fakeRing, item string) {
	t.Helper()
	if ring.item != item {
		t.Errorf("Expected %q in the keychain, got %q", item, ring.item)
	}
}

// expectAsked asserts how many times the keychain was asked
func expectAsked(t *testing.T, ring *fakeRing, times int32) {
	t.Helper()
	if n := ring.asked.Load(); n != times {
		t.Errorf("Expected the keychain to be asked %d times, got %d", times, n)
	}
}

// expectHelper asserts whether Git has this CLI as its credential helper
func expectHelper(t *testing.T, git *fakeGit, registered bool) {
	t.Helper()
	if git.registered != registered {
		t.Errorf("Expected the credential helper of Git registered=%v", registered)
	}
}

// expectFallback asserts what the store tells of its last save: where the
// session was saved for want of a keychain, which is empty when it was
// not, and the commands that are left to set Git by hand, if any
func expectFallback(t *testing.T, store *credStore, path string, setup ...string) {
	t.Helper()
	if fb := store.Fallback(); fb != path {
		t.Errorf("Expected fallback to %q, got %q", path, fb)
	}
	if got := store.GitSetup(); !slices.Equal(got, setup) {
		t.Errorf("Expected Git setup %q, got %q", setup, got)
	}
}

// The session is saved in the keychain alone, and Git is given the helper
// to read it by. No .netrc is created, and a session that was saved in one
// is removed from it, with the entries of other hosts left as they were.
func TestCredentialStoreKeychain(t *testing.T) {
	for name, before := range map[string]string{
		"without .netrc":         "",
		"with a session to move": savedEntries,
	} {
		t.Run(name, func(t *testing.T) {
			path := useNetrc(t, before)
			t.Setenv(netrcWriteEnv, "false") // Only "true" keeps it in .netrc
			ring := &fakeRing{}
			store, git := testStore(ring)

			saveSession(t, store)
			expectKeychain(t, ring, keyringItem)
			expectHelper(t, git, true)
			expectFallback(t, store, "")

			if before == "" {
				expectNoFile(t, path)
			} else {
				expectFile(t, path, otherEntry, 0600)
			}
		})
	}
}

// Where Git cannot be given the helper, the session is in the keychain
// all the same, and how to give it by hand is told until it is wiped
func TestCredentialStoreGitSetup(t *testing.T) {
	useNetrc(t, "")
	ring := &fakeRing{}
	store, git := testStore(ring)
	git.setup = []string{"git config --global ..."}

	saveSession(t, store)
	expectKeychain(t, ring, keyringItem)
	expectFallback(t, store, "", git.setup...)

	wipeSession(t, store)
	expectFallback(t, store, "")
}

// What is saved or wiped is what is read next, whatever was read before
func TestCredentialStoreRead(t *testing.T) {
	useNetrc(t, "")
	store, _ := testStore(&fakeRing{})

	expectAuth(t, store, "", "")
	saveSession(t, store)
	wipeSession(t, store)
}

// A .netrc that has a macro is never written, which does not keep a
// session from the keychain when the file has none to remove
func TestCredentialStoreKeychainMacro(t *testing.T) {
	content := otherEntry + macroEntry
	path := useNetrc(t, content)
	store, _ := testStore(&fakeRing{})

	saveSession(t, store)
	expectFile(t, path, content, 0644)
}

// Where a .netrc that has a macro would have to be written, the session
// is saved nowhere, and Git is not given the helper
func TestCredentialStoreUnsaved(t *testing.T) {
	for name, tc := range map[string]struct {
		entry, env string
		ring       *fakeRing
		user, pass string
	}{
		// Without a keychain that works, the file is where the session would
		// go, as it is with FURY_NETRC_WRITE=true
		"to fall back": {otherEntry, "", &fakeRing{err: errors.New("locked")}, "", ""},
		"as asked":     {otherEntry, "true", &fakeRing{}, "", ""},

		// An entry of Gemfury would be read before the keychain, by Git as by us
		"to remove an entry": {"machine api.fury.io login old@example.com password old\n", "", &fakeRing{}, "old@example.com", "old"},
	} {
		t.Run(name, func(t *testing.T) {
			content := tc.entry + macroEntry
			path := useNetrc(t, content)
			t.Setenv(netrcWriteEnv, tc.env)
			store, git := testStore(tc.ring)

			if err := store.Append("u@example.com", "abc123"); err == nil {
				t.Error("Expected an error")
			}

			expectKeychain(t, tc.ring, "")
			expectHelper(t, git, false)
			expectFallback(t, store, "")
			expectAuth(t, store, tc.user, tc.pass)
			expectFile(t, path, content, 0644)
		})
	}
}

// Without a keychain that works, the session is saved in .netrc, where
// Git reads it without a helper
func TestCredentialStoreFallback(t *testing.T) {
	for name, tc := range map[string]struct {
		ring      *fakeRing
		reachable bool
		asked     int32
	}{
		"keychain fails":         {&fakeRing{err: errors.New("locked")}, true, 1},
		"keychain never answers": {&fakeRing{hang: true}, true, 1},
		"no keychain to ask":     {&fakeRing{}, false, 0},
	} {
		t.Run(name, func(t *testing.T) {
			path := useNetrc(t, "")
			store, git := testStore(tc.ring)
			store.reachable = func() bool { return tc.reachable }

			saveSession(t, store)
			expectFile(t, path, furyEntry, 0600)
			expectHelper(t, git, false)
			expectFallback(t, store, path)

			// The session in .netrc is read without the keychain
			expectAsked(t, tc.ring, tc.asked)
		})
	}
}

// With FURY_NETRC_WRITE=true the session is saved in .netrc as asked, which
// is no fallback, and a session that the keychain has is not read
func TestCredentialStoreNetrcOnly(t *testing.T) {
	for _, value := range []string{"true", " TRUE "} {
		t.Run(value, func(t *testing.T) {
			path := useNetrc(t, "")
			t.Setenv(netrcWriteEnv, value)

			ring := &fakeRing{item: keyringItem}
			store, git := testStore(ring)
			expectAuth(t, store, "", "")

			saveSession(t, store)
			expectFile(t, path, furyEntry, 0600)
			expectHelper(t, git, false)
			expectFallback(t, store, "")
			expectAsked(t, ring, 0)
		})
	}
}

func TestCredentialStoreAuth(t *testing.T) {
	for name, tc := range map[string]struct {
		netrc      string
		ring       *fakeRing
		user, pass string
		asked      int32
	}{
		"logged out":         {"", &fakeRing{}, "", "", 1},
		"in the keychain":    {otherEntry, &fakeRing{item: keyringItem}, "u@example.com", "abc123", 1},
		".netrc comes first": {furyEntry, &fakeRing{item: `{"user":"k@example.com","token":"other"}`}, "u@example.com", "abc123", 0},

		// None of these is a failure: the user is to login, not to be stopped
		"keychain fails":         {"", &fakeRing{item: keyringItem, err: errors.New("locked")}, "", "", 1},
		"keychain never answers": {"", &fakeRing{item: keyringItem, hang: true}, "", "", 1},
		"not a session":          {"", &fakeRing{item: "abc123"}, "", "", 1},
		"without a token":        {"", &fakeRing{item: `{"user":"u@example.com"}`}, "", "", 1},
	} {
		t.Run(name, func(t *testing.T) {
			useNetrc(t, tc.netrc)
			store, _ := testStore(tc.ring)

			// The keychain is asked at most once, however many times it is read
			expectAuth(t, store, tc.user, tc.pass)
			expectAuth(t, store, tc.user, tc.pass)
			expectAsked(t, tc.ring, tc.asked)
		})
	}
}

// A .netrc that cannot be read is an error, rather than
// a reason to read the keychain
func TestCredentialStoreUnreadable(t *testing.T) {
	useNetrc(t, otherEntry+"bogus")
	ring := &fakeRing{item: keyringItem}
	store, _ := testStore(ring)

	if _, token, err := store.Auth(); err == nil || token != "" {
		t.Errorf("Expected an error and no token, got %q and: %v", token, err)
	}
	expectAsked(t, ring, 0)
}

// The session is removed from the keychain and from .netrc, wherever
// it would be saved now, along with the credential helper of Git
func TestCredentialStoreWipe(t *testing.T) {
	for name, env := range map[string]string{"by default": "", "with FURY_NETRC_WRITE": "true"} {
		t.Run(name, func(t *testing.T) {
			path := useNetrc(t, savedEntries)
			t.Setenv(netrcWriteEnv, env)

			ring := &fakeRing{item: keyringItem}
			store, git := testStore(ring)
			git.registered = true

			wipeSession(t, store)
			expectFile(t, path, otherEntry, 0600)
			expectKeychain(t, ring, "")
			expectHelper(t, git, false)
		})
	}
}

// With nothing saved, nothing is removed, and no .netrc is created
func TestCredentialStoreWipeNothing(t *testing.T) {
	for name, ring := range map[string]*fakeRing{
		"empty keychain":         {},
		"keychain fails":         {err: errors.New("locked")},
		"keychain never answers": {hang: true},
	} {
		t.Run(name, func(t *testing.T) {
			path := useNetrc(t, "")
			store, _ := testStore(ring)

			wipeSession(t, store)
			expectNoFile(t, path)
		})
	}
}

// A session that stays in either store is a failure, as it would be read
// as if nothing had been removed. The other store is wiped all the same,
// and Git is no longer given the helper.
func TestCredentialStoreWipeFails(t *testing.T) {
	for name, tc := range map[string]struct {
		netrc string
		ring  *fakeRing

		// What is left, and so is read
		left, item string
		perm       os.FileMode
	}{
		"in the keychain": {furyEntry, &fakeRing{item: keyringItem, deleteErr: errors.New("read-only")}, "", keyringItem, 0600},
		"in .netrc":       {furyEntry + macroEntry, &fakeRing{item: keyringItem}, furyEntry + macroEntry, "", 0644},
	} {
		t.Run(name, func(t *testing.T) {
			path := useNetrc(t, tc.netrc)
			store, git := testStore(tc.ring)
			git.registered = true

			// The error is that of the keychain, where it has one
			err := store.Wipe()
			if err == nil || tc.ring.deleteErr != nil && !errors.Is(err, tc.ring.deleteErr) {
				t.Errorf("Expected the error of what is left, got: %v", err)
			}

			expectFile(t, path, tc.left, tc.perm)
			expectKeychain(t, tc.ring, tc.item)
			expectHelper(t, git, false)
			expectAuth(t, store, "u@example.com", "abc123")
		})
	}
}

// A session that was read from the keychain is known to be there. It may
// well be there still when the keychain then fails, or stops answering,
// unlike with a keychain that has never had it read.
func TestCredentialStoreWipeUnanswered(t *testing.T) {
	for name, fail := range map[string]func(*fakeRing){
		"keychain fails":           func(ring *fakeRing) { ring.err = errors.New("locked") },
		"keychain stops answering": func(ring *fakeRing) { ring.hang = true },
	} {
		t.Run(name, func(t *testing.T) {
			useNetrc(t, "")
			ring := &fakeRing{item: keyringItem}
			store, _ := testStore(ring)
			expectAuth(t, store, "u@example.com", "abc123")

			fail(ring)
			store.timeout = hangTimeout
			if err := store.Wipe(); err == nil {
				t.Error("Expected an error")
			}
		})
	}
}

// A keychain that is slow to answer has its user told of the wait,
// as it may be asking them to unlock it, and told once
func TestCredentialStoreWaiting(t *testing.T) {
	useNetrc(t, "")
	ring := &fakeRing{item: keyringItem, gate: make(chan struct{})}
	store, _ := testStore(ring)

	// The keychain answers once its user is told of the wait
	told := 0
	store.notice, store.timeout = time.Millisecond, 5*time.Second
	store.waiting = func() {
		told++
		close(ring.gate)
	}

	expectAuth(t, store, "u@example.com", "abc123")

	// Slow again, it is waited for without a word, and given up on
	ring.gate, store.timeout = make(chan struct{}), hangTimeout
	if err := store.Wipe(); err == nil {
		t.Error("Expected an error")
	}
	if told != 1 {
		t.Errorf("Expected the wait to be told of once, got %d times", told)
	}
}

// A keychain that has not answered in time is not waited for again
func TestCredentialStoreTimeout(t *testing.T) {
	useNetrc(t, "")
	ring := &fakeRing{hang: true}
	store, _ := testStore(ring)

	expectAuth(t, store, "", "")
	saveSession(t, store)
	wipeSession(t, store)
	expectAsked(t, ring, 1)
}

// With no one at the terminal, there is no one to unlock the keychain, and
// less time for it to answer. Either way, a wait that gets long is told of
// on stderr, as a warning is.
func TestCredentialStoreTimeoutByTerminal(t *testing.T) {
	for interactive, exp := range map[bool]time.Duration{
		true:  keyringTimeout,
		false: keyringUnattendedTimeout,
	} {
		term := NewForTest()
		term.SetInteractive(interactive)

		store := CredentialStore(term).(*credStore)
		if store.timeout != exp {
			t.Errorf("Expected %s for interactive=%v, got %s", exp, interactive, store.timeout)
		} else if store.notice != keyringNotice {
			t.Errorf("Expected the wait to be told of after %s, got %s", keyringNotice, store.notice)
		}

		store.waiting()
		if msg, got := "Waiting for the system keychain ...\n", string(term.ErrBytes()); got != msg {
			t.Errorf("Expected %q on stderr, got %q", msg, got)
		} else if out := term.OutBytes(); len(out) != 0 {
			t.Errorf("Expected nothing on stdout, got %q", out)
		}
	}
}

// There is a session bus by its address, or by what the runtime
// directory of the user has, for a keychain to be asked on it
func TestSessionBus(t *testing.T) {
	for name, tc := range map[string]struct {
		address, file string
		exp           bool
	}{
		"address":            {"unix:path=/run/user/1000/bus", "", true},
		"socket":             {"", "bus", true},
		"file of an address": {"", "dbus-session", true},
		"another file":       {"", "other", false},
		"to be launched":     {"autolaunch:", "", false},
		"nothing":            {"", "", false},
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("DBUS_SESSION_BUS_ADDRESS", tc.address)
			if tc.file != "" {
				if err := os.WriteFile(filepath.Join(dir, tc.file), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}

			if got := sessionBus(dir); got != tc.exp {
				t.Errorf("Expected a session bus=%v, got %v", tc.exp, got)
			}
		})
	}
}
