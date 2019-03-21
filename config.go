package main

import (
	"fmt"
	"github.com/jessevdk/go-flags"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

const (
	defaultRPCPort = 5000
)

type lndNodeConfig struct {
	RpcServer    string `long:"rpcserver" description:"host:port of ln daemon"`
	MacaroonPath string `long:"macaroonpath" description:"path to macaroon file"`
	TlsCertPath  string `long:"tlscertpath" description:"path to TLS certificate"`
}

type config struct {
	ShowVersion  bool     `short:"v" long:"version" description:"Display version information and exit."`
	Debug        bool     `long:"debug" description:"Start in debug mode."`
	Node         string   `long:"node" description:"The node that should be rebalanced." choice:"lnd"`
	RawListeners []string `long:"listen" description:"Add an interface/port/socket to listen for RPC connections"`
	Listeners    []net.Addr
	LndNode      *lndNodeConfig `group:"LND" namespace:"lnd"`
}

func loadConfig() (*config, error) {
	defaultCfg := config{
		Debug: false,
		Node:  "lnd",
		LndNode: &lndNodeConfig{
			RpcServer:    "localhost:10009",
			MacaroonPath: "admin.macaroon",
			TlsCertPath:  "tls.cert",
		},
	}

	preCfg := defaultCfg

	if _, err := flags.Parse(&preCfg); err != nil {
		return nil, err
	}

	cfg := preCfg

	cfg.LndNode.MacaroonPath = cleanAndExpandPath(cfg.LndNode.MacaroonPath)
	cfg.LndNode.TlsCertPath = cleanAndExpandPath(cfg.LndNode.TlsCertPath)

	// Listen on the default interface/port if no listeners were specified.
	// An empty address string means default interface/address, which on
	// most unix systems is the same as 0.0.0.0.
	if len(cfg.RawListeners) == 0 {
		addr := fmt.Sprintf(":%d", defaultRPCPort)
		cfg.RawListeners = append(cfg.RawListeners, addr)
	}

	cfg.Listeners = make([]net.Addr, 0, len(cfg.RawListeners))
	for _, addr := range cfg.RawListeners {
		parsedAddr, err := net.ResolveTCPAddr("tcp", addr)
		if err != nil {
			return nil, err
		}

		cfg.Listeners = append(cfg.Listeners, parsedAddr)
	}

	return &cfg, nil
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
// This function is taken from https://github.com/btcsuite/btcd
func cleanAndExpandPath(path string) string {
	if path == "" {
		return ""
	}

	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		var homeDir string
		user, err := user.Current()
		if err == nil {
			homeDir = user.HomeDir
		} else {
			homeDir = os.Getenv("HOME")
		}

		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but the variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}
