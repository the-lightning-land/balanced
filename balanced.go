package main

import (
	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
	"os"
)

var (
	// Commit stores the current commit hash of this build. This should be set using -ldflags during compilation.
	commit string
	// Version stores the version string of this build. This should be set using -ldflags during compilation.
	version string
	// Stores the date of this build. This should be set using -ldflags during compilation.
	date string
)

// balancedMain is the true entry point for balanced. This is required since defers
// created in the top-level scope of a main method aren't executed if os.Exit() is called.
func balancedMain() error {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)

	log.Debug("Starting balanced...")

	// Print version of the daemon
	log.Infof("Version %s (commit %s)", version, commit)
	log.Infof("Built on %s", date)

	return nil
}

func main() {
	// Call the "real" main in a nested manner so the defers will properly
	// be executed in the case of a graceful shutdown.
	if err := balancedMain(); err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
		} else {
			log.WithError(err).Println("Failed running balanced.")
		}
		os.Exit(1)
	}
}
