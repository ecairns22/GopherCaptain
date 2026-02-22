package runner

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Call records a single invocation of a command.
type Call struct {
	Name string
	Args []string
}

func (c Call) String() string {
	return c.Name + " " + strings.Join(c.Args, " ")
}

// Response is a pre-configured response for a command pattern.
type Response struct {
	Stdout string
	Stderr string
	Err    error
}

// FakeRunner records command calls and returns pre-configured responses.
// Exported for use by systemd/nginx tests.
type FakeRunner struct {
	mu        sync.Mutex
	Calls     []Call
	responses map[string]Response // key: "name arg1 arg2..."
	fallback  Response
}

// NewFakeRunner creates a FakeRunner with an optional default response.
func NewFakeRunner() *FakeRunner {
	return &FakeRunner{
		responses: make(map[string]Response),
	}
}

// SetResponse configures a response for a specific command string.
func (f *FakeRunner) SetResponse(cmd string, resp Response) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses[cmd] = resp
}

// SetFallback sets the default response for unmatched commands.
func (f *FakeRunner) SetFallback(resp Response) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fallback = resp
}

// Run records the call and returns the matching response.
func (f *FakeRunner) Run(_ context.Context, name string, args ...string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	call := Call{Name: name, Args: args}
	f.Calls = append(f.Calls, call)

	key := call.String()
	if resp, ok := f.responses[key]; ok {
		return resp.Stdout, resp.Stderr, resp.Err
	}

	// Try matching just the command name with first arg for broader matches
	if len(args) > 0 {
		partial := name + " " + args[0]
		if resp, ok := f.responses[partial]; ok {
			return resp.Stdout, resp.Stderr, resp.Err
		}
	}

	// Try matching just the command name
	if resp, ok := f.responses[name]; ok {
		return resp.Stdout, resp.Stderr, resp.Err
	}

	return f.fallback.Stdout, f.fallback.Stderr, f.fallback.Err
}

// Called returns true if a command matching the prefix was recorded.
func (f *FakeRunner) Called(prefix string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.Calls {
		if strings.HasPrefix(c.String(), prefix) {
			return true
		}
	}
	return false
}

// CallCount returns the number of times a command matching the prefix was called.
func (f *FakeRunner) CallCount(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.Calls {
		if strings.HasPrefix(c.String(), prefix) {
			n++
		}
	}
	return n
}

// Reset clears all recorded calls.
func (f *FakeRunner) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = nil
}

// Ensure FakeRunner implements CommandRunner.
var _ CommandRunner = (*FakeRunner)(nil)
var _ CommandRunner = (*OSRunner)(nil)

// Suppress unused import warning for fmt.
var _ = fmt.Sprintf
