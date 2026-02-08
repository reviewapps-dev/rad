package updater

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/reviewapps-dev/rad/internal/version"
)

const (
	githubRepo   = "reviewapps-dev/rad"
	githubAPIURL = "https://api.github.com/repos/" + githubRepo + "/releases/latest"
)

// ReleaseInfo describes an available release.
type ReleaseInfo struct {
	Version     string
	DownloadURL string
	Checksum    string // SHA-256 hex
	Changelog   string
}

// githubRelease is the GitHub API response for a release.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Body    string        `json:"body"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckForUpdate queries GitHub releases for the latest version.
// Returns nil if already up to date.
func CheckForUpdate() (*ReleaseInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", githubAPIURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if !isNewer(latestVersion, version.Version) {
		return nil, nil // up to date
	}

	// Find the asset for this platform
	platform := runtime.GOOS + "_" + runtime.GOARCH // e.g. linux_amd64
	assetName := fmt.Sprintf("rad_%s.tar.gz", platform)

	var downloadURL string
	var checksumURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
		}
		if asset.Name == "checksums.txt" {
			checksumURL = asset.BrowserDownloadURL
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("no binary available for %s (looking for %s)", platform, assetName)
	}

	// Fetch checksum if available
	var checksum string
	if checksumURL != "" {
		checksum, _ = fetchChecksum(client, checksumURL, assetName)
	}

	return &ReleaseInfo{
		Version:     latestVersion,
		DownloadURL: downloadURL,
		Checksum:    checksum,
		Changelog:   release.Body,
	}, nil
}

// Apply downloads and installs an update.
func Apply(info *ReleaseInfo) error {
	fmt.Printf("Downloading rad %s...\n", info.Version)

	// Download to temp file
	tmpFile, err := download(info.DownloadURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer os.Remove(tmpFile)

	// Verify checksum if provided
	if info.Checksum != "" {
		fmt.Printf("Verifying checksum...\n")
		if err := verifyChecksum(tmpFile, info.Checksum); err != nil {
			return fmt.Errorf("checksum: %w", err)
		}
	}

	// Extract binary from tarball
	fmt.Printf("Extracting...\n")
	binPath, err := extractBinary(tmpFile)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	defer os.Remove(binPath)

	// Replace current binary
	fmt.Printf("Installing...\n")
	if err := replaceBinary(binPath); err != nil {
		return fmt.Errorf("install: %w", err)
	}

	fmt.Printf("Updated rad to %s (was %s)\n", info.Version, version.Version)
	return nil
}

// download fetches a URL to a temp file and returns the path.
func download(url string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download returned %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "rad-update-*.tar.gz")
	if err != nil {
		return "", err
	}

	totalBytes := resp.ContentLength
	var written int64
	buf := make([]byte, 32*1024)
	lastPct := -1

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := tmp.Write(buf[:n]); err != nil {
				tmp.Close()
				return "", err
			}
			written += int64(n)

			// Progress
			if totalBytes > 0 {
				pct := int(written * 100 / totalBytes)
				pct = (pct / 10) * 10 // round to 10%
				if pct != lastPct {
					fmt.Printf("  %d%%\n", pct)
					lastPct = pct
				}
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			tmp.Close()
			return "", readErr
		}
	}

	tmp.Close()
	return tmp.Name(), nil
}

// verifyChecksum checks the SHA-256 hash of a file.
func verifyChecksum(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

// extractBinary extracts the "rad" binary from a gzipped tarball.
func extractBinary(tarballPath string) (string, error) {
	f, err := os.Open(tarballPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	// tar is a simple format — we just need to find the "rad" file
	// Each tar entry: 512-byte header, then data padded to 512 bytes
	return extractFromTar(gz)
}

// replaceBinary replaces the current executable with a new one.
func replaceBinary(newBinPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find current executable: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve symlinks: %w", err)
	}

	// Get current permissions
	info, err := os.Stat(exe)
	if err != nil {
		return err
	}

	// Create backup
	backupPath := exe + ".bak"
	if err := copyFile(exe, backupPath); err != nil {
		return fmt.Errorf("backup: %w", err)
	}

	// Try atomic rename first
	if err := os.Rename(newBinPath, exe); err != nil {
		// Fall back to copy
		if err := copyFile(newBinPath, exe); err != nil {
			// Restore from backup
			_ = copyFile(backupPath, exe)
			os.Remove(backupPath)
			return fmt.Errorf("replace binary: %w", err)
		}
	}

	// Preserve permissions
	os.Chmod(exe, info.Mode())

	// Clean up backup
	os.Remove(backupPath)
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	// Preserve source permissions
	info, err := os.Stat(src)
	if err == nil {
		os.Chmod(dst, info.Mode())
	}

	return out.Close()
}

// fetchChecksum downloads checksums.txt and extracts the hash for the given asset.
func fetchChecksum(client *http.Client, url, assetName string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Format: "hash  filename"
	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			return parts[0], nil
		}
	}

	return "", fmt.Errorf("checksum not found for %s", assetName)
}

// isNewer returns true if a is newer than b (semver comparison).
func isNewer(a, b string) bool {
	av := parseSemver(a)
	bv := parseSemver(b)
	for i := 0; i < 3; i++ {
		if av[i] > bv[i] {
			return true
		}
		if av[i] < bv[i] {
			return false
		}
	}
	return false
}

func parseSemver(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	// Strip pre-release suffix
	if idx := strings.Index(v, "-"); idx != -1 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	var result [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		result[i], _ = strconv.Atoi(parts[i])
	}
	return result
}

// IsSystemd returns true if we appear to be running under systemd.
func IsSystemd() bool {
	return os.Getenv("INVOCATION_ID") != "" || os.Getppid() == 1
}
