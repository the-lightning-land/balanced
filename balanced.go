package main

import (
	"github.com/go-errors/errors"
	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
	"github.com/the-lightning-land/balanced/balancer"
	"github.com/the-lightning-land/balanced/lndc"
	"github.com/the-lightning-land/balanced/rpc"
	"google.golang.org/grpc"
	"net"
	"os"
	"os/signal"
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

	// Load CLI configuration and defaults
	cfg, err := loadConfig()
	if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
		return nil
	} else if err != nil {
		return errors.Errorf("Failed parsing arguments: %v", err)
	}

	// Set logger into debug mode if called with --debug
	if cfg.Debug {
		log.SetLevel(log.DebugLevel)
		log.Info("Setting debug mode.")
	}

	// Print version of the daemon
	log.Infof("Version %s (commit %s)", version, commit)
	log.Infof("Built on %s", date)

	// Create the lnd client that manages the connection to the lnd instance
	// and transforms lnd specific models into a more generic one
	client, err := lndc.NewClient(&lndc.Config{
		RpcServer:    cfg.LndNode.RpcServer,
		TlsCertPath:  cfg.LndNode.TlsCertPath,
		MacaroonPath: cfg.LndNode.MacaroonPath,
	})
	if err != nil {
		return errors.Errorf("Could not create lnd client: %v", err)
	}

	// Start the lnd client so it can manage its own internal lifecycle
	err = client.Start()
	if err != nil {
		return errors.Errorf("Could not start lnd client: %v", err)
	}
	defer func() {
		err := client.Stop()
		if err != nil {
			log.Errorf("Could not properly stop lnd client: %v", err)
		}
	}()

	// Create the balancer which has the task of automatic balancing and
	// coordination
	err, balancer := balancer.NewBalancer(&balancer.Config{
		Logger: log.New().WithField("system", "balancer"),
		Client: client,
	})
	if err != nil {
		return errors.Errorf("Could not create balancer: %v", err)
	}

	go func() {
		// Handle interrupt signals
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt)
		sig := <-signals
		log.Info(sig)
		log.Info("Received an interrupt, stopping dispenser...")
		balancer.Stop()
	}()

	// Create a gRPC server for remote control of the balancer
	if len(cfg.Listeners) > 0 {
		grpcServer := grpc.NewServer()

		rpc.RegisterBalanceServer(grpcServer, newRPCServer(&rpcServerConfig{
			balancer: balancer,
			version:  version,
			commit:   commit,
		}))

		// Next, Start the gRPC server listening for HTTP/2 connections.
		for _, listener := range cfg.Listeners {
			lis, err := net.Listen(listener.Network(), listener.String())
			if err != nil {
				return errors.Errorf("RPC server unable to listen on %v%v", listener.Network(), listener.String())
			}

			defer lis.Close()

			go func() {
				log.Infof("RPC server listening on %s", lis.Addr())
				grpcServer.Serve(lis)
			}()
		}
	}

	// Run balancer and block until it's stopped
	err = balancer.Run()
	if err != nil {
		return errors.Errorf("Could not run balancer: %v", err)
	}

	// finish without error
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
