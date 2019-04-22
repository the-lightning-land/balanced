package main

import (
	"context"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
	"github.com/the-lightning-land/balanced/rpc"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"os"
	"strconv"
)

var (
	// Commit stores the current commit hash of this build. This should be set using -ldflags during compilation.
	commit string
	// Version stores the version string of this build. This should be set using -ldflags during compilation.
	version string
	// Stores the date of this build. This should be set using -ldflags during compilation.
	date string
)

// balanceMain is the true entry point for balance. This is required since defers
// created in the top-level scope of a main method aren't executed if os.Exit() is called.
func balanceMain() error {
	app := cli.NewApp()
	app.EnableBashCompletion = true
	app.Version = version

	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("version=%s commit=%s date=%s\n", version, commit, date)
	}

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "rpcserver",
			Value: "localhost:5000",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:      "balance",
			ArgsUsage: "[from] [to] [amt]",
			Aliases:   []string{"b"},
			Usage:     "balance a particular channel",
			Action: func(c *cli.Context) error {
				conn, err := grpc.Dial(c.GlobalString("rpcserver"), grpc.WithInsecure())
				if err != nil {
					return errors.Errorf("Could not connect to lightning node: %v", err)
				}

				ctx := context.Background()
				client := rpc.NewBalanceClient(conn)

				fromChanId, _ := strconv.Atoi(c.Args().Get(0))
				toChanId, _ := strconv.Atoi(c.Args().Get(1))
				amtMsat, _ := strconv.Atoi(c.Args().Get(2))

				res, err := client.Balance(ctx, &rpc.BalanceRequest{
					FromChanId: uint64(fromChanId),
					ToChanId:   uint64(toChanId),
					AmtMsat:    uint64(amtMsat),
				})
				if err != nil || !res.Rebalanced {
					return errors.Errorf("Could not balance: %v", err)
				}

				return nil
			},
			BashComplete: func(c *cli.Context) {
				// This will complete if no args are passed
				if c.NArg() > 0 {
					return
				}
				fmt.Println("yoyoyo")
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

func main() {
	// Call the "real" main in a nested manner so the defers will properly
	// be executed in the case of a graceful shutdown.
	if err := balanceMain(); err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
		} else {
			log.WithError(err).Println("Failed running balance.")
		}
		os.Exit(1)
	}
}
