// SoloSet is a single-click launcher that stands up Apache Superset locally.
// This file wires together the local status web server and the browser opener;
// Docker detection and Superset orchestration are layered on in later milestones.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/rizvi14/SoloSet/internal/system"
	"github.com/rizvi14/SoloSet/internal/web"
)

// preferredPort is tried first so the status page lives at a stable, memorable
// address across runs; if it is taken we fall back to an OS-assigned free port.
const preferredPort = 7654

func main() {
	if err := run(); err != nil {
		log.Fatalf("SoloSet: %v", err)
	}
}

func run() error {
	store := web.NewStore()
	server := web.NewServer(store)

	listener, err := listen()
	if err != nil {
		return fmt.Errorf("could not start local server: %w", err)
	}
	url := fmt.Sprintf("http://%s", listener.Addr().String())

	httpServer := &http.Server{Handler: server.Handler()}
	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("status server stopped: %v", err)
		}
	}()

	fmt.Printf("SoloSet status page: %s\n", url)
	// SOLOSET_NO_BROWSER lets tests and headless runs skip the browser popup.
	if os.Getenv("SOLOSET_NO_BROWSER") == "" {
		if err := system.OpenBrowser(url); err != nil {
			fmt.Printf("Could not open a browser automatically. Open this URL manually:\n  %s\n", url)
		}
	}

	// M1 placeholder: prove the status pipeline end-to-end. Docker detection
	// (M2) and Superset launch (M3) replace this block.
	store.Set(web.PhaseInit, "SoloSet is running.",
		"Skeleton is live — Docker detection and Superset launch come next.")
	store.AppendLog("Local status server started at " + url)

	// Block until interrupted, then shut the server down cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	<-ctx.Done()

	fmt.Println("\nShutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

// listen binds to the preferred port on loopback, falling back to any free port.
func listen() (net.Listener, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", preferredPort)
	if l, err := net.Listen("tcp", addr); err == nil {
		return l, nil
	}
	return net.Listen("tcp", "127.0.0.1:0")
}
