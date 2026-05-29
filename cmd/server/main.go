// Re-enable legacy TLS RSA key exchange ciphers and SHA-1 signatures.
// Required for connecting to old ESXi hosts (5.x / 6.0 / 6.5) that only
// offer these cipher suites. Go 1.22 disabled them by default.
//
//go:debug tlsrsakex=1
//go:debug tls10server=1
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	// kept for godebug directives above
	_ "unsafe"

	"github.com/vmOrbit/backend/internal/bootstrap"
	"github.com/vmOrbit/backend/pkg/logger"
)

// @title           VmOrbit API
// @version         1.0
// @description     Unified Hypervisor Management Platform API
// @termsOfService  http://vmOrbit.io/terms/

// @contact.name   VmOrbit Support
// @contact.url    http://vmOrbit.io/support
// @contact.email  support@vmOrbit.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	log := logger.NewLogger()

	app, err := bootstrap.NewApplication(log)
	if err != nil {
		log.Fatal("failed to initialize application", logger.Error(err))
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.Start(ctx); err != nil {
			log.Fatal("application failed to start", logger.Error(err))
			os.Exit(1)
		}
	}()

	<-quit
	log.Info("shutting down VmOrbit server...")

	if err := app.Stop(ctx); err != nil {
		log.Error("error during shutdown", logger.Error(err))
	}

	log.Info("VmOrbit server stopped")
}
