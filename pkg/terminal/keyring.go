package terminal

import (
	"github.com/zalando/go-keyring"

	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Item of the system keychain that holds the saved session
const (
	keyringService = "fury-cli"
	keyringAccount = "api.fury.io"
)

// netrcWriteEnv is the environment variable that keeps the session
// in .netrc, even where there is a keychain to hold it
const netrcWriteEnv = "FURY_NETRC_WRITE"

// How long the keychain has to answer. It may be asking its user to unlock
// it, which takes a while: they are told of a wait that gets long. With no
// one at the terminal, there is no one to unlock it, and far less time.
const (
	keyringTimeout           = 60 * time.Second
	keyringUnattendedTimeout = 3 * time.Second
	keyringNotice            = 2 * time.Second
)

// errKeyringUnavailable is of a keychain that is not there to ask,
// or that did not answer in time
var errKeyringUnavailable = errors.New("System keychain is not available")

// keyringSession is the content of the keychain item, as JSON
type keyringSession struct {
	User  string `json:"user"`
	Token string `json:"token"`
}

// secrets is the item of the system keychain that holds the session:
// in Keychain of macOS, Credential Manager of Windows, or the Secret
// Service of a Linux desktop. Its errors are those of go-keyring,
// with ErrNotFound for an item that is not there.
type secrets interface {
	Set(item string) error
	Get() (string, error)
	Delete() error
}

type systemKeyring struct{}

func (systemKeyring) Set(item string) error {
	return keyring.Set(keyringService, keyringAccount, item)
}

func (systemKeyring) Get() (string, error) {
	return keyring.Get(keyringService, keyringAccount)
}

func (systemKeyring) Delete() error {
	return keyring.Delete(keyringService, keyringAccount)
}

// FallbackAuther is an Auther that tells what its user is to know of the
// session that it has saved last, when that did not go as it would by
// default: it is to be asked right after the session is saved
type FallbackAuther interface {
	Auther

	// Fallback is the path of .netrc when the session was saved there for
	// want of a keychain, rather than as asked
	Fallback() string

	// GitSetup is the commands that set this CLI as the credential helper
	// of Git, when the session is in the keychain and Git could not be set
	GitSetup() []string
}

var _ FallbackAuther = (*credStore)(nil)

// CredentialStore saves the session in the system keychain, or else in
// .netrc: where there is no keychain, where the keychain fails, or as
// asked by FURY_NETRC_WRITE=true. A session in the keychain is not in
// .netrc for Git to read, so Git is given "fury git credentials" as its
// credential helper for git.fury.io.
func CredentialStore(t Terminal) Auther {
	timeout := keyringUnattendedTimeout
	if t.IsInteractive() {
		timeout = keyringTimeout
	}

	return &credStore{
		netrc:     Netrc(),
		ring:      systemKeyring{},
		git:       gitConfig{},
		reachable: keyringReachable,
		timeout:   timeout,
		notice:    keyringNotice,
		waiting:   func() { t.EPrintf("Waiting for the system keychain ...\n") },
	}
}

type credStore struct {
	netrc     Auther
	ring      secrets
	git       gitHelper
	reachable func() bool
	timeout   time.Duration
	notice    time.Duration
	waiting   func() // Tells of a wait for the keychain: nil once it has

	session  *keyringSession // Of the keychain once it is read: empty for none
	down     bool            // The keychain has failed to answer in time
	fallback string
	gitSetup []string
}

func (s *credStore) Auth() (string, string, error) {
	// .netrc comes first, as it does for Git, which reads it before it asks
	// a helper. And a session that is there is read without the keychain.
	user, pass, err := s.netrc.Auth()
	if err != nil || pass != "" {
		return user, pass, err
	}

	if netrcOnly() {
		return "", "", nil
	}

	// Without an item, or without a keychain to hold it, there is no
	// session: an error here would keep its user from a login, which
	// is what saves the session to .netrc instead
	if s.session == nil {
		s.session = s.keyringGet()
	}

	return s.session.User, s.session.Token, nil
}

func (s *credStore) Append(user, pass string) error {
	s.session, s.fallback, s.gitSetup = nil, "", nil

	if netrcOnly() {
		return s.netrc.Append(user, pass)
	}

	// The session is to be in the keychain alone: an entry left in .netrc
	// would be read first, by Git as by us. It is removed first: if it
	// cannot be, nothing is saved.
	if err := s.netrc.Wipe(); err != nil {
		return err
	}

	if s.keyringSet(user, pass) == nil {
		s.gitSetup = s.git.Register()
		return nil
	}

	if err := s.netrc.Append(user, pass); err != nil {
		return err
	}

	s.fallback, _ = netrcPath()
	return nil
}

// Wipe removes the session from wherever it may be,
// rather than only from where it would be saved now
func (s *credStore) Wipe() error {
	wasRead := s.session != nil && s.session.Token != ""
	s.session, s.fallback, s.gitSetup = nil, "", nil
	s.git.Unregister()
	return errors.Join(s.keyringDelete(wasRead), s.netrc.Wipe())
}

func (s *credStore) Fallback() string {
	return s.fallback
}

func (s *credStore) GitSetup() []string {
	return s.gitSetup
}

// keyringGet is the session that the keychain has, which is empty when
// it has none, or when it cannot be asked
func (s *credStore) keyringGet() *keyringSession {
	session := keyringSession{}
	out, err := s.ask(s.ring.Get)
	if err != nil || json.Unmarshal([]byte(out), &session) != nil || session.Token == "" {
		return &keyringSession{}
	}
	return &session
}

func (s *credStore) keyringSet(user, token string) error {
	item, _ := json.Marshal(keyringSession{User: user, Token: token})
	_, err := s.ask(func() (string, error) {
		return "", s.ring.Set(string(item))
	})
	return err
}

// keyringDelete removes the session from the keychain. With wasRead,
// the session is known to be there, as it was read from it.
func (s *credStore) keyringDelete(wasRead bool) error {
	_, err := s.ask(func() (string, error) {
		return "", s.ring.Delete()
	})

	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}

	// A keychain that fails, or that is not there to ask, has no session
	// to be read: the failure matters only for a session that is known to
	// be there, or that is read still
	if wasRead || s.keyringGet().Token != "" {
		return fmt.Errorf("Error removing credentials from the system keychain: %w", err)
	}

	return nil
}

// ask asks the keychain, for no longer than the timeout: it has none of
// its own, and may never answer. Once it has not, it is not asked again.
// A wait that gets long is told of, once.
func (s *credStore) ask(question func() (string, error)) (string, error) {
	if s.down || !s.reachable() {
		return "", errKeyringUnavailable
	}

	type answer struct {
		out string
		err error
	}

	done := make(chan answer, 1)
	go func() {
		out, err := question()
		done <- answer{out, err}
	}()

	var notice <-chan time.Time
	if s.waiting != nil {
		notice = time.After(s.notice)
	}

	timeout := time.After(s.timeout)
	for {
		select {
		case a := <-done:
			return a.out, a.err
		case <-notice:
			s.waiting()
			s.waiting = nil
		case <-timeout:
			s.down = true
			return "", errKeyringUnavailable
		}
	}
}

// netrcOnly reports whether the session is kept in .netrc, as asked
func netrcOnly() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(netrcWriteEnv)), "true")
}

// keyringReachable reports whether there may be a keychain to ask. Other
// than on macOS and Windows, it is a service of the session bus, without
// which there is none.
func keyringReachable() bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return true
	}
	return sessionBus("/run/user/" + strconv.Itoa(os.Getuid()))
}

// sessionBus reports whether there is a session bus, by its address or in
// the runtime directory of the user. That is where the library looks for
// it, before it resorts to starting one by dbus-launch.
func sessionBus(dir string) bool {
	if addr := os.Getenv("DBUS_SESSION_BUS_ADDRESS"); addr != "" && addr != "autolaunch:" {
		return true
	}

	for _, name := range []string{"bus", "dbus-session"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}

	return false
}
