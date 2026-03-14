package runtime

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/phpToro/cli/internal/ui"
)

const (
	releasesAPI = "https://api.github.com/repos/phpToro/runtimes/releases/latest"
	runtimesDir = ".phptoro/runtimes"
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, runtimesDir)
}

func TargetDir(target string) string {
	return filepath.Join(Dir(), target)
}

func IsInstalled(target string) bool {
	// Check exact match first
	info, err := os.Stat(TargetDir(target))
	if err == nil && info.IsDir() {
		return true
	}
	// Check for versioned directory (e.g. php-8.5.3-ios-arm64-sim)
	return FindRuntimeDir(target) != ""
}

// FindRuntimeDir finds the actual runtime directory for a target.
// Handles both versioned dirs (php-8.5.3-ios-arm64-sim) and download dirs
// where the tar extracts nested (ios-arm64-sim/php-8.5.3-ios-arm64-sim/).
func FindRuntimeDir(target string) string {
	base := Dir()
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}

	// First pass: prefer versioned directories (e.g. php-X.Y.Z-target)
	for _, e := range entries {
		if e.IsDir() && strings.HasSuffix(e.Name(), target) && strings.HasPrefix(e.Name(), "php-") {
			dir := filepath.Join(base, e.Name())
			if hasRuntimeFiles(dir) {
				return dir
			}
		}
	}

	// Second pass: check for nested extraction (target/php-X.Y.Z-target/)
	for _, e := range entries {
		if e.IsDir() && e.Name() == target {
			dir := filepath.Join(base, e.Name())
			// Check for nested versioned dir
			nested, _ := os.ReadDir(dir)
			for _, n := range nested {
				if n.IsDir() && strings.HasSuffix(n.Name(), target) {
					ndir := filepath.Join(dir, n.Name())
					if hasRuntimeFiles(ndir) {
						return ndir
					}
				}
			}
			if hasRuntimeFiles(dir) {
				return dir
			}
		}
	}

	return ""
}

func hasRuntimeFiles(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "lib"))
	return err == nil
}

func EnsureRuntime(target string) error {
	if IsInstalled(target) {
		ui.Success("Runtime already installed: " + target)
		return nil
	}

	ui.Info("Downloading runtime: " + target + "...")

	release, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to fetch releases: %w", err)
	}

	var asset *githubAsset
	for _, a := range release.Assets {
		if strings.Contains(a.Name, target) {
			asset = &a
			break
		}
	}
	if asset == nil {
		return fmt.Errorf("no runtime found for target %q in release %s", target, release.TagName)
	}

	if err := downloadAndExtract(asset, target); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	ui.Success("Runtime installed: " + target + " (" + release.TagName + ")")
	return nil
}

func fetchLatestRelease() (*githubRelease, error) {
	resp, err := http.Get(releasesAPI)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}
	return &release, nil
}

func downloadAndExtract(asset *githubAsset, target string) error {
	resp, err := http.Get(asset.BrowserDownloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	pr := ui.ProgressWriter(asset.Size, "Downloading")
	reader := io.TeeReader(resp.Body, pr)

	destDir := TargetDir(target)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	gz, err := gzip.NewReader(reader)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path := filepath.Join(destDir, hdr.Name)

		// Prevent path traversal
		if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(destDir)+string(os.PathSeparator)) && filepath.Clean(path) != filepath.Clean(destDir) {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(path, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(path), 0755)
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			// Limit extraction size to 500MB per file
			if _, err := io.Copy(f, io.LimitReader(tr, 500*1024*1024)); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}

	fmt.Fprintln(os.Stderr) // newline after progress
	return nil
}
