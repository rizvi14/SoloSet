// Package superset holds the details of the Superset instance SoloSet runs:
// where to reach it, how to log in, the one-time init commands, and a health
// probe.
package superset

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	// BaseURL is where the container publishes Superset on the host.
	BaseURL   = "http://localhost:8088"
	HealthURL = BaseURL + "/health"

	// Default local credentials created during init. Fine to be well-known:
	// the instance is bound to the user's own machine.
	Username = "admin"
	Password = "admin"

	// InitMarker is a sentinel file on the data volume that records that the
	// one-time init (db upgrade / admin / init / examples) already ran, so
	// restarts skip the slow example-loading step.
	InitMarker = "/app/superset_home/.soloset_initialized"
)

// DBUpgradeArgs migrates the metadata database.
func DBUpgradeArgs() []string { return []string{"superset", "db", "upgrade"} }

// CreateAdminArgs creates the default admin user.
func CreateAdminArgs() []string {
	return []string{
		"superset", "fab", "create-admin",
		"--username", Username,
		"--firstname", "Superset",
		"--lastname", "Admin",
		"--email", "admin@superset.com",
		"--password", Password,
	}
}

// InitArgs initializes roles and permissions.
func InitArgs() []string { return []string{"superset", "init"} }

// LoadExamplesArgs loads the bundled example dashboards and datasets.
func LoadExamplesArgs() []string { return []string{"superset", "load_examples"} }

// WaitHealthy polls Superset's /health endpoint until it returns 200 or the
// timeout elapses.
func WaitHealthy(ctx context.Context, timeout time.Duration) error {
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)

	for {
		if req, err := http.NewRequestWithContext(ctx, http.MethodGet, HealthURL, nil); err == nil {
			if resp, err := client.Do(req); err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("Superset did not become healthy within %s", timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}
