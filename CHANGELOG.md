# Changelog

## v1.0.0

`fury` 1.0 is the Gemfury CLI continued in Go. It replaces the CLI
of the `gemfury` [Ruby gem][RubyCLI], whose own history is in its
[changelog][RubyLog], with one binary that needs no runtime, for
macOS, Linux, and Windows. Most of the Ruby CLI commands have a
counterpart here, and the old colon form, as `fury git:config`,
is still accepted.

What follows is what this CLI does beyond the Ruby one.

### Installation

- One static binary per platform: macOS (universal), Linux (x86_64 and
  ARM64), and Windows (x86_64 and ARM64), as archives on GitHub, with a
  checksums file. The binaries are built reproducibly, from the module as the
  Go module proxy serves it, so a build from the tag matches the release,
  except that the macOS binary may order its two architectures differently.
- Homebrew: `brew install --cask gemfury/tap/fury-cli`. The formula of the
  same name is retired: `brew update` moves an existing install to the cask,
  after which `brew uninstall --formula fury-cli` removes the old copy.
- Debian, Alpine, and Arch Linux packages, for apt, apk, and pacman, on the
  Gemfury `cli` repository.

### Authentication

- `fury login` opens the browser. With `--interactive`, or where the browser
  login is not available, it asks for email and password at the terminal.
- The session is saved in the system keychain: Keychain on macOS, Credential
  Manager on Windows, and the Secret Service of a Linux desktop. Without
  one, as on a server or in a container, it is saved in `.netrc`, and
  `FURY_NETRC_WRITE=true` keeps it there where there is a keychain. A
  session in `.netrc` is read first, as Git does.
- `fury login` sets a credential helper in Git's global configuration, for
  `https://git.fury.io` only, so `git push` finds the session. `fury logout`
  removes it, revokes the session, and clears it from the keychain and
  `.netrc`.
- `FURY_TOKEN` authenticates and `FURY_ACCOUNT` names the account to act on,
  for scripts and CI. The `--api-token` and `--account` flags take
  precedence, and `FURY_TOKEN` takes precedence over the saved session.
- `--api-token -` reads the token from stdin, so that it is not in the
  command line or the shell history. When there is someone to ask, it is
  asked for, masked.
- `fury login --api-token TOKEN` saves the token as the session, in place of
  any saved before. With `FURY_TOKEN` alone, `fury login` only verifies, so a
  token set for every command is not saved by accident. A token of an
  organization acts on that organization without `--account`.

### Unattended use

- Without credentials and without a terminal, a command fails at once with
  `Not logged in. Set FURY_TOKEN or run "fury login" in a terminal.`, and no
  request is made.
- `--yes`, or `-y`, answers every y/N question. `--no-input` asks nothing,
  as when there is no terminal, and a command that needs an answer fails
  as a usage mistake unless `--yes` is given.
- `yank`, `git destroy`, and `git reset` ask before they change anything.
  `--force`, or `-f`, skips the question.
- Ctrl-C, or a SIGTERM, cancels the running command, which prints a single
  `Cancelled` and exits as interrupted. A command with several items stops
  at the item that it was on.

### Exit status

- Distinct failures have distinct exit statuses: a usage mistake, not found,
  not authenticated, already exists, and unavailable each have their own,
  and an interruption has the conventional one. A script can react without
  reading the error. The statuses are listed in `fury --help`.
- A command that fails on several items exits with the status that all of
  its failures share, or else with the general one.

### Errors

- A failure is one `Error:` line on stderr. Usage text follows only a
  mistake in the arguments or flags, so piped output stays clean.
- An error has the message of the server, where it sends one, and names what
  the command asked for, as in `Package "foo": Doesn't look like this
  exists`. A failure of the server has what to report it by:
  `Something went wrong. Please contact support. (HTTP 502, request ID ...)`.
- A command with several items, as `push`, `yank`, `sharing add`, and
  `sharing remove`, keeps going after one fails, reports each failure as it
  happens, and ends with a summary such as `Error: 1 of 3 uploads failed`.

### JSON output

- `--json` on the commands that read: `packages`, `versions`, `sharing`,
  `accounts`, `git list`, `git config`, `git config get`, `git stack`, and
  `whoami`. The flag follows the command, and is unknown elsewhere.
- The result is the only thing on stdout. Every field of a record is
  printed, empty when the API does not provide it; an empty listing is `[]`,
  and `null` is printed only for the release version of a package that has
  none. Fields carry the API's names, such as `kind_key` and `created_at`,
  and times are RFC 3339 in UTC.
- With `--json`, nothing is asked.

### Quiet output

- `--quiet`, or `-q`, shows only results, warnings, and errors: no progress,
  no banners, no confirmation of a change, no "No ... found". The exit
  status tells whether a change was made. Questions are still asked.
- `--no-progress` leaves out the progress bars and spinners, and nothing
  else. Without a terminal they are left out anyway.

### Resilience

- A request that reads is sent again when it is rate limited, the server is
  unavailable, or the connection fails: three times, waiting a second, then
  two, then four, or as long as the server asks, up to a minute. Each wait
  is announced on stderr, as `Rate limited. Retrying in 10s`, unless
  `--quiet`. `FURY_RETRIES` sets the count; `0` turns it off. A request that
  writes, such as an upload, is never sent again.
- `--timeout` gives up on a request after that long, as unavailable. By
  default an API request has 30 seconds, and an upload or a download has
  10 minutes.

### Commands

- `git stack` lists the build stacks of a repository, with the current one
  marked, and `git stack set` changes it.
- `git config` shows the build environment of a repository, and `git config
  get`, `set`, and `unset` read and change its variables.
- `git destroy` removes a repository; `git reset` empties it. `git rebuild`
  runs the builder, on a revision with `--revision`.
- `push --public` uploads a public version. `sharing add --role` gives the
  collaborator `pull`, `push`, or `owner`.
- `yank` removes several versions at once, as `PACKAGE@VERSION`, and takes
  `KIND:PACKAGE@VERSION` when packages of different kinds share a name.
- `versions` lists the filename of each version, and who created it and
  when.

### Help

- Every command has examples in its help, and `fury --help` says what the
  CLI does, how it authenticates, which environment variables it reads, and
  what each exit status means.
- A flag that takes a value from a fixed set lists it, as `--role` of
  `sharing add` does.

[RubyLog]: https://github.com/gemfury/gemfury/blob/main/CHANGELOG
[RubyCLI]: https://github.com/gemfury/gemfury
