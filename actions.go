package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// targetFPS replaces the 60 FPS Roblox and rosemary settle on by default
const targetFPS = 240

func versionFile(home string) string {
	return fmt.Sprintf("%s/.local/share/evergreen/version", home)
}

// channelFile holds the beta channel the installed build is from, empty for the live one
func channelFile(home string) string {
	return fmt.Sprintf("%s/.local/share/evergreen/channel", home)
}

// installPlayer downloads build version into the prefix, replacing the Player that's there
func installPlayer(ctx context.Context, home string, logger *zap.Logger, version, channel string, progress util.ProgressFunc) error {
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
	logger.Info("installed roblox player", zap.String("version", version), zap.String("channel", channel))
	if err := os.WriteFile(channelFile(home), []byte(channel+"\n"), 0o644); err != nil {
		return err
	}
	info, err := os.ReadFile(filepath.Join(app, "Contents", "Info.plist"))
	if err != nil {
		return err
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(info))
	if err := os.WriteFile(filepath.Join(home, ".local/share/evergreen/bundle-checksum"), []byte(checksum), 0o644); err != nil {
		return err
	}
	return os.WriteFile(versionFile(home), []byte(version+"\n"), 0o644)
}

// installLatest installs the newest Player whether or not it's already installed.
// On a beta channel that's the installed build, since only the Player can find out about newer ones.
func installLatest(ctx context.Context, home string, logger *zap.Logger, progress util.ProgressFunc) error {
	if channel := installedChannel(home); channel != "" && playerInstalled(home) {
		return installPlayer(ctx, home, logger, installedVersion(home), channel, progress)
	}
	progress("Checking version", 0, -1)
	version, err := util.GetVersion(ctx, logger)
	if err != nil {
		return err
	}
	return installPlayer(ctx, home, logger, version.ClientVersionUpload, "", progress)
}

// requestedBuild returns the build the Player last asked its own updater for, if that happened after Evergreen's last install.
// The updater can't install under rosemary (it crashes closing the menu bar app), but it logs the build and channel
// first. Beta channels' builds can't be looked up without signing in, so this is the only way Evergreen learns them.
func requestedBuild(home string) (version, channel string, ok bool) {
	logs, _ := filepath.Glob(fmt.Sprintf("%s/root/Users/*/Library/Roblox/RobloxPlayerInstaller/InstallerInfo.log", prefixDir(home)))
	installed, err := os.Stat(versionFile(home))
	if len(logs) == 0 || err != nil {
		return "", "", false
	}
	data, err := os.ReadFile(logs[0])
	if err != nil {
		return "", "", false
	}
	// lines look like "<unix time> IN_PROGRESS <pid> version-<guid> <channel>", the channel missing for live builds
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		fields := strings.Fields(lines[i])
		if len(fields) < 4 || !strings.HasPrefix(fields[3], "version-") {
			continue
		}
		at, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil || at <= installed.ModTime().Unix() {
			return "", "", false
		}
		if len(fields) > 4 {
			channel = fields[4]
		}
		return fields[3], channel, true
	}
	return "", "", false
}

// updatePlayer installs the build Roblox wants when the installed one is missing or out of date.
// Roblox only lets the current Player join games, but an installed one is still worth launching when Roblox can't be reached.
func updatePlayer(ctx context.Context, home string, logger *zap.Logger, progress util.ProgressFunc) error {
	if version, channel, ok := requestedBuild(home); ok && version != installedVersion(home) {
		logger.Info("installing the build the player asked for", zap.String("version", version), zap.String("channel", channel))
		return installPlayer(ctx, home, logger, version, channel, progress)
	}
	// the live version says nothing about a beta channel's build, the Player asks for those itself
	if channel := installedChannel(home); channel != "" && playerInstalled(home) {
		if installedBuildIntact(home) {
			return nil
		}
		logger.Info("restoring beta build replaced by Roblox's updater", zap.String("channel", channel))
		return installPlayer(ctx, home, logger, installedVersion(home), channel, progress)
	}
	progress("Checking for updates", 0, -1)
	version, err := util.GetVersion(ctx, logger)
	if err != nil {
		if playerInstalled(home) {
			logger.Warn("couldn't check for updates, launching the installed player", zap.Error(err))
			return nil
		}
		return err
	}
	if playerInstalled(home) && installedVersion(home) == version.ClientVersionUpload && installedBuildIntact(home) {
		return nil
	}
	return installPlayer(ctx, home, logger, version.ClientVersionUpload, "", progress)
}

// runPlayer runs the Player, handing it url (a roblox:// or roblox-player: link) when there is one
func runPlayer(ctx context.Context, home string, logger *zap.Logger, url string) error {
	if err := unlockFPS(home); err != nil {
		logger.Error("failed to write client settings", zap.Error(err))
		return err
	}
	// rosemary fires the Player's display link, which it paces frames by, at the screen's refresh rate.
	// Reporting a faster screen lifts that limit, unless the user picked a rate of their own.
	var env []string
	if os.Getenv("ROSEMARY_SCREEN_REFRESH_RATE") == "" {
		env = append(env, "ROSEMARY_SCREEN_REFRESH_RATE="+strconv.Itoa(targetFPS))
	}
	args := []string{"run", playerApp(home)}
	if url != "" {
		args = append(args, playerLaunchURL(url, installedChannel(home)))
	}
	return util.RunRosemary(ctx, logger, env, args...)
}

// unlockFPS raises Roblox's own frame cap through its ClientAppSettings.json, keeping any other flags in it
func unlockFPS(home string) error {
	path := fmt.Sprintf("%s/Contents/MacOS/ClientSettings/ClientAppSettings.json", playerApp(home))
	flags := map[string]any{}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &flags); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	flags["DFIntTaskSchedulerTargetFps"] = targetFPS
	data, err = json.MarshalIndent(flags, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// installedVersion returns the version Evergreen last installed, or "" if none
func installedVersion(home string) string {
	data, err := os.ReadFile(versionFile(home))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// installedChannel returns the beta channel of the installed build, or "" for the live one
func installedChannel(home string) string {
	data, err := os.ReadFile(channelFile(home))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func playerInstalled(home string) bool {
	_, err := os.Stat(playerApp(home))
	return err == nil && installedVersion(home) != ""
}

// The website can send channel: with no value, which makes the Player replace a
// beta build with production before it processes the game join.
func playerLaunchURL(url, channel string) string {
	if channel == "" || !strings.HasPrefix(url, "roblox-player:") {
		return url
	}
	fields := strings.Split(url, "+")
	found := false
	for i, field := range fields {
		if strings.HasPrefix(field, "channel:") {
			fields[i] = "channel:" + channel
			found = true
		}
	}
	if !found {
		fields = append(fields, "channel:"+channel)
	}
	return strings.Join(fields, "+")
}

// Roblox's own updater can overwrite the app without updating Evergreen's receipt.
func installedBuildIntact(home string) bool {
	checksum, err := os.ReadFile(filepath.Join(home, ".local/share/evergreen/bundle-checksum"))
	if err != nil {
		return false
	}
	info, err := os.ReadFile(filepath.Join(playerApp(home), "Contents", "Info.plist"))
	if err != nil {
		return false
	}
	return string(checksum) == fmt.Sprintf("%x", sha256.Sum256(info))
}
