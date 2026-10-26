package util

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

type APIVersion struct {
	Version             string `json:"version"`
	ClientVersionUpload string `json:"clientVersionUpload"`
	BootstrapperVersion string `json:"bootstrapperVersion"`
}

// bundlePrefix is the directory every entry of the Player zip sits under
const bundlePrefix = "RobloxPlayer.app/"

func get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

func GetVersion(ctx context.Context, logger *zap.Logger) (*APIVersion, error) {
	resp, err := get(ctx, "https://clientsettingscdn.roblox.com/v2/client-version/MacPlayer")
	if err != nil {
		logger.Error("failed to get current roblox player version", zap.Error(err))
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logger.Error("unexpected status getting roblox player version", zap.Int("status", resp.StatusCode))
		return nil, fmt.Errorf("unexpected status %d getting the player version", resp.StatusCode)
	}
	var data APIVersion
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		logger.Error("error decoding version json", zap.Error(err))
		return nil, err
	}
	return &data, nil
}

// ProgressFunc reports progress for a stage of work. total is -1 when unknown.
type ProgressFunc func(stage string, done, total int64)

type progressReader struct {
	r        io.Reader
	done     int64
	total    int64
	progress ProgressFunc
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	p.progress("Downloading", p.done, p.total)
	return n, err
}

// DownloadPlayer downloads the macOS Player and extracts RobloxPlayer.app into dest.
// The zip is spooled to a temporary file in tmpDir, since it is too big to hold in memory comfortably.
func DownloadPlayer(ctx context.Context, logger *zap.Logger, version APIVersion, tmpDir, dest string, progress ProgressFunc) error {
	url := fmt.Sprintf("https://setup.rbxcdn.com/mac/%s-RobloxPlayer.zip", version.ClientVersionUpload)
	resp, err := get(ctx, url)
	if err != nil {
		logger.Error("failed to download roblox player", zap.Error(err))
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logger.Error("unexpected status downloading roblox player", zap.Int("status", resp.StatusCode), zap.String("url", url))
		return fmt.Errorf("unexpected status %d downloading %s", resp.StatusCode, url)
	}

	tmp, err := os.CreateTemp(tmpDir, "RobloxPlayer-*.zip")
	if err != nil {
		logger.Error("failed to create download file", zap.Error(err))
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	_, err = io.Copy(tmp, &progressReader{r: resp.Body, total: resp.ContentLength, progress: progress})
	if err != nil {
		logger.Error("failed to download roblox player", zap.Error(err))
		return err
	}
	size, err := tmp.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(tmp, size)
	if err != nil {
		logger.Error("failed to read zip", zap.Error(err))
		return err
	}

	for i, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		progress("Extracting", int64(i+1), int64(len(zr.File)))
		name, ok := strings.CutPrefix(f.Name, bundlePrefix)
		if !ok || name == "" {
			continue
		}
		target := filepath.Join(dest, name)
		if !inside(dest, target) {
			logger.Error("illegal path", zap.String("path", target))
			return fmt.Errorf("illegal path in zip: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				logger.Error("failed to make dir", zap.Error(err))
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			logger.Error("failed to make dir", zap.Error(err))
			return err
		}
		if err := extractFile(f, dest, target); err != nil {
			logger.Error("failed to extract file", zap.Error(err), zap.String("path", target))
			return err
		}
	}
	return nil
}

func inside(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func extractFile(f *zip.File, dest, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	// .app bundles contain symlinks (Frameworks/*/Versions/Current), the link target is stored as the file contents
	if f.Mode()&os.ModeSymlink != 0 {
		link, err := io.ReadAll(rc)
		if err != nil {
			return err
		}
		linkTarget := string(link)
		if filepath.IsAbs(linkTarget) || !inside(dest, filepath.Join(filepath.Dir(target), linkTarget)) {
			return fmt.Errorf("illegal symlink %s -> %s", f.Name, linkTarget)
		}
		return os.Symlink(linkTarget, target)
	}

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
