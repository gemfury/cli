package cli

import (
	"fmt"
	"regexp"
	"time"
)

var (
	roundDurationRE = regexp.MustCompile(`^\d+\w`)
)

func timeStringWithAgo(t time.Time) string {
	out := t.Local().Format("2006-01-02 15:04")

	if ago := time.Now().Sub(t); ago < 24*time.Hour {
		agoStr := ago.Round(time.Second).String()
		if str := roundDurationRE.FindString(agoStr); str != "" {
			out = fmt.Sprintf("%s (~ %s ago)", out, str)
		}
	}

	return out
}
