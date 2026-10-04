package cli

import (
	"fmt"
	"strings"
	"time"
)

// timeString is a time as printed in a table, in local time. At a terminal
// it has how long ago it was, when under a day. Piped, it has the zone
// offset, as the output may be read in another zone, and no "ago", which
// would give some rows more columns.
func timeString(t time.Time, tty bool) string {
	if t.IsZero() {
		return "N/A" // Like the other fields the API leaves out
	}

	local := t.Local()
	if !tty {
		return local.Format("2006-01-02 15:04 -07:00")
	}

	out := local.Format("2006-01-02 15:04")
	// A time ahead of the clock, by skew, has no "ago"
	if ago := time.Since(t); ago >= 0 && ago < 24*time.Hour {
		out += fmt.Sprintf(" (~ %s ago)", agoString(ago))
	}
	return out
}

// agoString is how long ago, in its largest whole unit: 2h for 2h30m
func agoString(d time.Duration) string {
	switch {
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// packageKinds lists the package kinds of the API, for help only. They are
// not validated here, so a new kind can be used before a release lists it.
const packageKinds = "ruby, js, python, php, deb, rpm, bower, nuget, maven, go"

// splitPackageVersion splits an argument of PACKAGE@VERSION
// into its two parts, neither of which may be empty
func splitPackageVersion(arg string) (pkg, ver string, ok bool) {
	at := strings.LastIndex(arg, "@")
	if at <= 0 || at == len(arg)-1 {
		return "", "", false
	}
	return arg[:at], arg[at+1:], true
}
