package main

import (
	"context"
	"os"

	"github.com/urfave/cli/v3"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func noProgress(string, int64, int64) {}

func main() {
	cfg := zap.NewDevelopmentConfig()
	cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	cfg.DisableStacktrace = true
	logger, _ := cfg.Build()
	home, err := os.UserHomeDir() // /home/user
	if err != nil {
		logger.Fatal("home directory err", zap.Error(err))
	}
	root := &cli.Command{
		Name:      "evergreen",
		Usage:     "Launch the Roblox Player",
		ArgsUsage: "[roblox-player: or roblox:// link]",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			runLauncher(home, logger, cmd.Args().First())
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:    "setup",
				Aliases: []string{"init", "initialize"},
				Usage:   "Set up Evergreen",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return setup(home)
				},
			},
			{
				Name:  "install",
				Usage: "Install the latest Roblox Player",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return installLatest(ctx, home, logger, noProgress)
				},
			},
			{
				Name:      "run",
				Usage:     "Update and run the Player from the terminal",
				ArgsUsage: "[roblox-player: or roblox:// link]",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if err := updatePlayer(ctx, home, logger, noProgress); err != nil {
						return err
					}
					return runPlayer(ctx, home, logger, cmd.Args().First())
				},
			},
			{
				Name:    "cfg",
				Aliases: []string{"config"},
				Usage:   "Open the Evergreen window",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					runConfig(home, logger)
					return nil
				},
			},
		},
	}
	if err := root.Run(context.Background(), os.Args); err != nil {
		logger.Fatal("error", zap.Error(err))
	}
}
