package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/base-org/pessimism/cmd/doc"
	"github.com/base-org/pessimism/internal/app"
	"github.com/base-org/pessimism/internal/client"
	"github.com/base-org/pessimism/internal/config"
	"github.com/base-org/pessimism/internal/logging"
	"github.com/base-org/pessimism/internal/state"

	"github.com/urfave/cli"
	"go.uber.org/zap"
)

const (
	// cfgPath ... env file path
	cfgPath = "config.env"
	extJSON = ".json"
)

// main ... Application driver
func main() {
	ctx := context.Background() // Create context
	logger := logging.WithContext(ctx)

	app := cli.NewApp()
	app.Name = "pessimism"
	app.Usage = "Pessimism Application"
	app.Description = "A monitoring service that allows for " +
		"Op-Stack and EVM compatible blockchains to be continuously assessed for real-time threats"
	app.Action = RunPessimism
	app.Commands = []cli.Command{
		{
			Name:        "doc",
			Subcommands: doc.Subcommands,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		logger.Fatal("Error running application", zap.Error(err))
	}
}

// RunPessimism ... Application entry point
func RunPessimism(_ *cli.Context) error {
	cfg := config.NewConfig(cfgPath) // Load env vars
	ctx := context.Background()

	// Init logger
	logging.New(cfg.Environment)
	logger := logging.WithContext(ctx)

	ss := state.NewMemState()
	bundle, err := client.NewBundle(ctx, cfg.ClientConfig)
	if err != nil {
		logger.Fatal("Error creating client bundle", zap.Error(err))
		return err
	}

	ctx = app.InitializeContext(ctx, ss, bundle)

	pessimism, shutDown, err := app.NewPessimismApp(ctx, cfg)

	if err != nil {
		logger.Fatal("Error creating pessimism application", zap.Error(err))
		return err
	}

	logger.Info("Starting pessimism application")
	if err := pessimism.Start(); err != nil {
		logger.Fatal("Error starting pessimism application", zap.Error(err))
		return err
	}

	if cfg.IsBootstrap() {
		logger.Debug("Bootstrapping application state")

		sessions, err := fetchBootSessions(cfg.BootStrapPath)
		if err != nil {
			logger.Fatal("Error loading bootstrap file", zap.Error(err))
			return err
		}

		ids, err := pessimism.BootStrap(sessions)
		if err != nil {
			logger.Fatal("Error bootstrapping application state", zap.Error(err))
			return err
		}

		logger.Info("Received bootstrapped session UUIDs", zap.Any(logging.Session, ids))

		logger.Debug("Application state successfully bootstrapped")
	}

	pessimism.ListenForShutdown(shutDown)

	logger.Debug("Waiting for all application threads to end")

	logger.Info("Successful pessimism shutdown")
	return nil
}

// fetchBootSessions ... Loads all the bootstrap file
func fetchBootSessions(path string) ([]*app.BootSession, error) {
	if !strings.HasSuffix(path, extJSON) {
		return nil, fmt.Errorf("invalid bootstrap file format; expected %s", extJSON)
	}

	file, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	data := []*app.BootSession{}

	err = json.Unmarshal(file, &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}
