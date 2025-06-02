package common

import (
	"context"
	"os/exec"
	"runtime"
)

// Provisioner defines the interface for a cloud provider provisioner.
// All operations are expected to be non-blocking and return immediately.
// The actual work is done asynchronously, and the provisioner should track events
// using the provided session. The session can be used to track events such as
// OAuth flow, validation, and provisioning.
type Provisioner interface {
	// Validate checks if the provided token is valid for the provisioner and collects compartment information.
	Validate(ctx context.Context, token string)
	// Compartments returns a list of compartments available in the provisioner (e.g., billing accounts, projects).
	Compartments() []Compartment
	// Session returns the current session for the provisioner, which can be used to track events.
	Session() *Session
	// Provision creates a new instance in the cloud provider using the specified placement ID and location ID.
	Provision(ctx context.Context, placementID string, locationID string)
}

type BrowserOpener func(url string) error

// OpenBrowserDesktop tries to open the URL in a browser.
func OpenBrowserDesktop(url string) error {
	var args []string
	switch runtime.GOOS {
	case "darwin":
		args = []string{"open"}
	case "windows":
		args = []string{"cmd", "/c", "start"}
	default:
		args = []string{"xdg-open"}
	}
	cmd := exec.Command(args[0], append(args[1:], url)...)
	return cmd.Start()
}
