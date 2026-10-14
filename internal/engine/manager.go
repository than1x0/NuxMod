package engine

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const (
	DefaultSingBoxVersion = "1.8.14"
)

// EngineManager handles lifecycle of the Sing-box core process.
type EngineManager struct {
	BinaryPath string
	ConfigPath string
	cmd        *exec.Cmd
}

// NewEngineManager creates a new instance of EngineManager.
func NewEngineManager(configPath string) *EngineManager {
	return &EngineManager{
		ConfigPath: configPath,
	}
}

// EnsureBinary checks if sing-box binary exists, or downloads it to targetDir.
func (m *EngineManager) EnsureBinary(targetDir string) (string, error) {
	// 1. Check if sing-box is already in system PATH
	if path, err := exec.LookPath("sing-box"); err == nil {
		m.BinaryPath = path
		return path, nil
	}

	// 2. Check if already downloaded to targetDir
	binName := "sing-box"
	if runtime.GOOS == "windows" {
		binName = "sing-box.exe"
	}
	localPath := filepath.Join(targetDir, binName)
	if _, err := os.Stat(localPath); err == nil {
		m.BinaryPath = localPath
		return localPath, nil
	}

	// 3. Download appropriate binary for OS/Arch
	fmt.Printf("[*] Sing-box core not found. Auto-downloading v%s for %s/%s...\n", DefaultSingBoxVersion, runtime.GOOS, runtime.GOARCH)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	url, err := getDownloadURL(DefaultSingBoxVersion, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	if err := downloadAndExtract(url, targetDir, binName); err != nil {
		return "", fmt.Errorf("failed to download sing-box: %w", err)
	}

	m.BinaryPath = localPath
	return localPath, nil
}

func getDownloadURL(version, goos, goarch string) (string, error) {
	var arch string
	switch goarch {
	case "amd64":
		arch = "amd64"
	case "arm64":
		arch = "arm64"
	default:
		return "", fmt.Errorf("unsupported architecture: %s", goarch)
	}

	if goos == "linux" {
		return fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/v%s/sing-box-%s-linux-%s.tar.gz", version, version, arch), nil
	} else if goos == "windows" {
		return fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/v%s/sing-box-%s-windows-%s.zip", version, version, arch), nil
	}

	return "", fmt.Errorf("unsupported OS: %s", goos)
}

func downloadAndExtract(downloadURL, targetDir, binName string) error {
	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download server returned HTTP %d", resp.StatusCode)
	}

	// Extract tar.gz (for Linux)
	if strings.HasSuffix(downloadURL, ".tar.gz") {
		gzr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return err
		}
		defer gzr.Close()

		tr := tar.NewReader(gzr)
		for {
			header, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}

			if strings.HasSuffix(header.Name, "sing-box") && !header.FileInfo().IsDir() {
				destFile := filepath.Join(targetDir, binName)
				outFile, err := os.OpenFile(destFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
				if err != nil {
					return err
				}
				defer outFile.Close()

				if _, err := io.Copy(outFile, tr); err != nil {
					return err
				}
				return nil
			}
		}
		return fmt.Errorf("sing-box binary not found in archive")
	}

	return fmt.Errorf("archive format not supported in auto-extractor")
}

// Start launches the sing-box process.
func (m *EngineManager) Start(ctx context.Context, stdout, stderr io.Writer) error {
	if m.BinaryPath == "" {
		return fmt.Errorf("binary path is empty")
	}

	m.cmd = exec.CommandContext(ctx, m.BinaryPath, "run", "-c", m.ConfigPath)
	m.cmd.Stdout = stdout
	m.cmd.Stderr = stderr

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start sing-box: %w", err)
	}

	return nil
}

// Stop gracefully terminates the sing-box process.
func (m *EngineManager) Stop() error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}

	// Try graceful SIGINT/SIGTERM first
	if runtime.GOOS == "windows" {
		_ = m.cmd.Process.Kill()
	} else {
		_ = m.cmd.Process.Signal(syscall.SIGINT)
	}

	// Wait up to 5 seconds
	done := make(chan error, 1)
	go func() {
		done <- m.cmd.Wait()
	}()

	select {
	case <-time.After(5 * time.Second):
		_ = m.cmd.Process.Kill()
	case <-done:
	}

	return nil
}

// Wait blocks until the underlying process exits.
func (m *EngineManager) Wait() error {
	if m.cmd == nil {
		return nil
	}
	return m.cmd.Wait()
}
