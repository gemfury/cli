package terminal

import (
	"github.com/manifoldco/promptui"

	"bytes"
	"io"
)

func NewForTest() *testTerm {
	streams := []*bytes.Buffer{{}, {}, {}}
	return &testTerm{
		prompts: map[string]string{},
		streams: streams,
		term: &term{
			ioErr: writeCloser{streams[0]},
			ioOut: writeCloser{streams[1]},
			ioIn:  io.NopCloser(streams[2]),
		},
	}
}

type testTerm struct {
	prompts map[string]string
	streams []*bytes.Buffer
	*term
}

func (tt testTerm) ErrBytes() []byte {
	return tt.streams[0].Bytes()
}

func (tt testTerm) OutBytes() []byte {
	return tt.streams[1].Bytes()
}

func (tt testTerm) InWrite(b []byte) (int, error) {
	return tt.streams[2].Write(b)
}

// Handle PromptUI to avoid messing with Readline
func (tt *testTerm) SetPromptResponses(p map[string]string) {
	tt.prompts = p
}

func (tt testTerm) RunPrompt(p *promptui.Prompt) (string, error) {
	if l, ok := p.Label.(string); ok {
		if out, ok := tt.prompts[l]; ok {
			return out, nil
		}
	}

	return "", io.EOF
}

// Implements Auther interface for testing
func TestAuther(u, p string, err error) *testAuth {
	return &testAuth{u, p, err}
}

type testAuth struct {
	User string
	Pass string
	Err  error
}

func (a testAuth) Auth() (string, string, error) {
	return a.User, a.Pass, a.Err
}

func (a *testAuth) Append(u, p string) error {
	a.User, a.Pass = u, p
	return a.Err
}

func (a *testAuth) Wipe() error {
	a.User, a.Pass = "", ""
	return a.Err
}

// Equivalent to io.NopCloser for writers
type writeCloser struct {
	io.Writer
}

func (writeCloser) Close() error {
	return nil
}
