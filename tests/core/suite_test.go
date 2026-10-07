package core_test

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/davidherring123/godot-cli/tests/harness"
)

var (
	suiteSession *harness.Session
	suiteError   error
)

func TestMain(m *testing.M) {
	flag.Parse()

	suiteSession, suiteError = harness.Start(suiteLogf)

	code := m.Run()

	if suiteSession != nil {
		if err := suiteSession.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "closing core test suite:", err)

			if code == 0 {
				code = 1
			}
		}
	}

	os.Exit(code)
}

func requireSuite(t *testing.T) *harness.Session {
	t.Helper()

	if errors.Is(suiteError, harness.ErrGodotNotFound) {
		t.Skip(suiteError)
	}

	if suiteError != nil {
		t.Fatal(suiteError)
	}

	return suiteSession
}

func suiteLogf(format string, args ...any) {
	if testing.Verbose() {
		fmt.Printf(format+"\n", args...)
	}
}
