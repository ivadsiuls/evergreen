package util

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapio"
)

// RunRosemary runs rosemary with args, logging its output. env is added to Evergreen's own environment.
func RunRosemary(ctx context.Context, log *zap.Logger, env []string, args ...string) error {
	stdout := &zapio.Writer{Log: log.Named("rosemary"), Level: zap.InfoLevel}
	defer stdout.Close()

	// rosemary and the Player report progress on stderr too, so it isn't a sign of failure
	stderr := &zapio.Writer{Log: log.Named("rosemary"), Level: zap.InfoLevel}
	defer stderr.Close()

	cmd := exec.CommandContext(ctx, "rosemary", args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// PrefixExists reports whether rosemary recognizes its default prefix
func PrefixExists(ctx context.Context) bool {
	return exec.CommandContext(ctx, "rosemary", "prefix", "info").Run() == nil
}

// RosemaryVersion returns the output of rosemary --version
func RosemaryVersion(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "rosemary", "--version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
