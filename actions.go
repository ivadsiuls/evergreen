package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ivadsiuls/evergreen/util"
	"go.uber.org/zap"
)

func prefixDir(home string) string {
	return fmt.Sprintf("%s/.local/share/rosemary/prefixes/default", home)
}

// playerApp is where a Mac keeps the Player, inside the rosemary prefix
func playerApp(home string) string {
	return fmt.Sprintf("%s/root/Applications/Roblox.app", prefixDir(home))
}

func versionFile(home string) string {
	return fmt.Sprintf("%s/.local/share/evergreen/version", home)
}

// installPlayer downloads version into the prefix, replacing the Player that's there
func installPlayer(ctx context.Context, home string, logger *zap.Logger, version util.APIVersion, progress util.ProgressFunc) error {
	if err := setup(home); err != nil {
		return err
	}
	if !util.PrefixExists(ctx) {
		progress("Creating rosemary prefix", 0, -1)
		// Evergreen has no terminal to answer rosemary's question about fetching the Darwin base, so answer it here
		if err := util.RunRosemary(ctx, logger, []string{"ROSEMARY_PREFIX_SETUP=download"}, "prefix", "create"); err != nil {
			logger.Error("failed to create rosemary prefix", zap.Error(err))
			return err
		}
	}
	app := playerApp(home)
	staging := app + ".new"
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		logger.Error("failed to create staging dir", zap.Error(err))
		return err
	}
	if err := util.DownloadPlayer(ctx, logger, version, fmt.Sprintf("%s/.local/share/evergreen", home), staging, progress); err != nil {
		return err
	}
	progress("Installing", 0, -1)
	if err := os.RemoveAll(app); err != nil {
		logger.Error("failed to remove old player", zap.Error(err))
		return err
	}
	if err := os.Rename(staging, app); err != nil {
		logger.Error("failed to move player into place", zap.Error(err))
		return err
	}
	logger.Info("installed roblox player", zap.String("version", version.ClientVersionUpload))
	return os.WriteFile(versionFile(home), []byte(version.ClientVersionUpload+"\n"), 0o644)
}

// installLatest installs the newest Player whether or not it's already installed
func installLatest(ctx context.Context, home string, logger *zap.Logger, progress util.ProgressFunc) error {
	progress("Checking version", 0, -1)
	version, err := util.GetVersion(ctx, logger)
	if err != nil {
		return err
	}
	return installPlayer(ctx, home, logger, *version, progress)
}

// updatePlayer installs the newest Player when the installed one is missing or out of date.
// Roblox only lets the current Player join games, but an installed one is still worth launching when Roblox can't be reached.
func updatePlayer(ctx context.Context, home string, logger *zap.Logger, progress util.ProgressFunc) error {
	progress("Checking for updates", 0, -1)
	version, err := util.GetVersion(ctx, logger)
	if err != nil {
		if playerInstalled(home) {
			logger.Warn("couldn't check for updates, launching the installed player", zap.Error(err))
			return nil
		}
		return err
	}
	if playerInstalled(home) && installedVersion(home) == version.ClientVersionUpload {
		return nil
	}
	return installPlayer(ctx, home, logger, *version, progress)
}

// runPlayer runs the Player, handing it url (a roblox:// or roblox-player: link) when there is one
func runPlayer(ctx context.Context, home string, logger *zap.Logger, url string) error {
	args := []string{"run", playerApp(home)}
	if url != "" {
		args = append(args, url)
	}
	return util.RunRosemary(ctx, logger, nil, args...)
}

// installedVersion returns the version Evergreen last installed, or "" if none
func installedVersion(home string) string {
	data, err := os.ReadFile(versionFile(home))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func playerInstalled(home string) bool {
	_, err := os.Stat(playerApp(home))
	return err == nil && installedVersion(home) != ""
}
