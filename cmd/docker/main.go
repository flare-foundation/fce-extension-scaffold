// Package main provides a combined entry point for Docker that starts both the
// tee-node server and the extension in a single process. Unlike tools/cmd/start-tee,
// this avoids importing extension-e2e — Docker sets PROXY_URL as an env var which
// tee-node reads directly via settings.init().
package main

import (
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/flare-foundation/go-flare-common/pkg/logger"
	teeServer "github.com/flare-foundation/tee-node/pkg/server"

	"extension-scaffold/internal/config"
	extserver "extension-scaffold/pkg/server"
)

func main() {
	configPort := intEnv("CONFIG_PORT", 5501)

	// config.SignPort and config.ExtensionPort are set from SIGN_PORT and
	// EXTENSION_PORT env vars via config.init().
	signPort := config.SignPort
	extensionPort := config.ExtensionPort

	// Start tee-node in extension mode.
	go teeServer.StartServerExtension(configPort, signPort, extensionPort)

	// Start extension server — fail fast if port binding fails.
	extErrCh := extserver.StartExtension(extensionPort, signPort)

	// Give server a moment to bind, then check for early failures.
	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-extErrCh:
		logger.Fatalf("extension server failed to start: %v", err)
	default:
	}

	logger.Infof("extension TEE running (config=%d, sign=%d, ext=%d)", configPort, signPort, extensionPort)

	// Wait for signal or server error.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigChan:
		logger.Info("shutting down")
	case err := <-extErrCh:
		logger.Fatalf("extension server error: %v", err)
	}
}

func intEnv(key string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return fallback
}
