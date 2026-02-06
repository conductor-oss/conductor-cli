/*
 * Copyright 2026 Conductor Authors.
 * <p>
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 * <p>
 * http://www.apache.org/licenses/LICENSE-2.0
 * <p>
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */


package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

const (
	// OSS Conductor server download URL template
	// Version is substituted: "latest" or specific version like "3.21.23"
	ossJarURLTemplate = "https://conductor-server.s3.us-east-2.amazonaws.com/conductor-server-%s.jar"

	// Orkes Conductor server download URL template (TBD)
	orkesJarURLTemplate = "" // TODO: Will be provided later

	jarFileName = "conductor-server.jar"
	pidFileName = "conductor.pid"
	logFileName = "conductor.log"

	// Minimum required Java version
	minJavaVersion = 21

	// Default server port
	defaultPort = 8080
)

// Server type constants
const (
	serverTypeOSS   = "oss"
	serverTypeOrkes = "orkes"
)

var (
	serverCmd = &cobra.Command{
		Use:     "server",
		Short:   "Local Conductor server management",
		Long:    "Start, stop, and manage a local Conductor server for development and testing.",
		GroupID: "development",
	}

	serverStartCmd = &cobra.Command{
		Use:   "start",
		Short: "Start a local Conductor server",
		Long: `Start a local Conductor server for development and testing.

The server runs as a background process and listens on port 8080 by default.
The server JAR file is downloaded automatically on first run.

Server Types:
  --oss     Use the open-source Conductor server (default)
  --orkes   Use the Orkes Conductor server (coming soon)

Requirements:
  - Java 21 or higher must be installed and available in PATH

The server files are stored in ~/.conductor-cli/server/

Examples:
  # Start OSS Conductor server (default)
  conductor server start

  # Start with specific version
  conductor server start --version 3.21.23

  # Start on a different port
  conductor server start --port 9090

  # Run in foreground mode
  conductor server start --foreground`,
		RunE:         startServer,
		SilenceUsage: true,
	}

	serverStopCmd = &cobra.Command{
		Use:          "stop",
		Short:        "Stop the local Conductor server",
		Long:         "Stop the running Conductor server.",
		RunE:         stopServer,
		SilenceUsage: true,
	}

	serverStatusCmd = &cobra.Command{
		Use:          "status",
		Short:        "Check the status of the server",
		Long:         "Check if the Conductor server is running.",
		RunE:         statusServer,
		SilenceUsage: true,
	}

	serverLogsCmd = &cobra.Command{
		Use:          "logs",
		Short:        "Show server logs",
		Long:         "Display the logs from the Conductor server.",
		RunE:         logsServer,
		SilenceUsage: true,
	}
)

// getServerDir returns the path to the server directory (~/.conductor-cli/server)
func getServerDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".conductor-cli", "server"), nil
}

// getServerDirForType returns the path to the server directory for a specific server type and version
func getServerDirForType(serverType, version string) (string, error) {
	baseDir, err := getServerDir()
	if err != nil {
		return "", err
	}
	// Store different server types/versions in separate directories
	return filepath.Join(baseDir, serverType, version), nil
}

// getJarPathForType returns the path to the server JAR file for a specific type and version
func getJarPathForType(serverType, version string) (string, error) {
	serverDir, err := getServerDirForType(serverType, version)
	if err != nil {
		return "", err
	}
	return filepath.Join(serverDir, jarFileName), nil
}

// getPidPath returns the path to the PID file
func getPidPath() (string, error) {
	serverDir, err := getServerDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(serverDir, pidFileName), nil
}

// getLogPath returns the path to the log file
func getLogPath() (string, error) {
	serverDir, err := getServerDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(serverDir, logFileName), nil
}

// getJarDownloadURL returns the download URL for the JAR based on server type and version
func getJarDownloadURL(serverType, version string) (string, error) {
	switch serverType {
	case serverTypeOSS:
		return fmt.Sprintf(ossJarURLTemplate, version), nil
	case serverTypeOrkes:
		// TODO: Update orkesJarURLTemplate when URL is provided
		return "", fmt.Errorf("Orkes Conductor server download is not yet available. Please use --oss flag for now")
	default:
		return "", fmt.Errorf("unknown server type: %s", serverType)
	}
}

// checkJavaVersion checks if Java is installed and returns the major version
func checkJavaVersion() (int, error) {
	cmd := exec.Command("java", "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("Java is not installed or not in PATH.\n\nPlease install Java 21 or higher:\n  - macOS: brew install openjdk@21\n  - Ubuntu/Debian: sudo apt install openjdk-21-jdk\n  - Windows: Download from https://adoptium.net/\n\nAfter installation, ensure 'java' is in your PATH")
	}

	// Parse Java version from output
	// Output format varies:
	// - openjdk version "21.0.1" 2023-10-17
	// - java version "1.8.0_291"
	// - openjdk version "17.0.1" 2021-10-19
	outputStr := string(output)

	// Try to find version pattern like "21.0.1" or "17.0.1" or "1.8.0"
	versionRegex := regexp.MustCompile(`version "(\d+)(?:\.(\d+))?`)
	matches := versionRegex.FindStringSubmatch(outputStr)

	if len(matches) < 2 {
		return 0, fmt.Errorf("could not determine Java version from output:\n%s", outputStr)
	}

	majorVersion, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("could not parse Java version: %w", err)
	}

	// Handle old versioning scheme (1.8 = Java 8)
	if majorVersion == 1 && len(matches) >= 3 {
		minorVersion, _ := strconv.Atoi(matches[2])
		majorVersion = minorVersion
	}

	return majorVersion, nil
}

// downloadJar downloads the server JAR file with progress indicator
func downloadJar(jarPath, jarURL string) error {
	fmt.Printf("Downloading Conductor server...\n")
	fmt.Printf("URL: %s\n", jarURL)

	// Create server directory
	serverDir := filepath.Dir(jarPath)
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		return fmt.Errorf("failed to create server directory: %w", err)
	}

	// Create temporary file for download
	tmpPath := jarPath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath) // Clean up temp file on error
	}()

	// Create HTTP request with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", jarURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		// Follow redirects (GitHub releases use redirects)
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Get content length for progress
	contentLength := resp.ContentLength
	if contentLength > 0 {
		fmt.Printf("Size: %.1f MB\n", float64(contentLength)/1024/1024)
	}

	// Download with progress
	var downloaded int64
	buf := make([]byte, 32*1024)
	lastProgress := -1

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := tmpFile.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			downloaded += int64(n)

			// Show progress
			if contentLength > 0 {
				progress := int(float64(downloaded) / float64(contentLength) * 100)
				if progress != lastProgress && progress%10 == 0 {
					fmt.Printf("Progress: %d%%\n", progress)
					lastProgress = progress
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("download interrupted: %w", err)
		}
	}

	// Close temp file before rename
	tmpFile.Close()

	// Rename temp file to final path
	if err := os.Rename(tmpPath, jarPath); err != nil {
		return fmt.Errorf("failed to save JAR file: %w", err)
	}

	fmt.Printf("Download complete: %s\n", jarPath)
	return nil
}

// readPid reads the PID from the pid file
func readPid() (int, error) {
	pidPath, err := getPidPath()
	if err != nil {
		return 0, err
	}

	data, err := os.ReadFile(pidPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	return pid, nil
}

// writePid writes the PID to the pid file
func writePid(pid int) error {
	pidPath, err := getPidPath()
	if err != nil {
		return err
	}

	return os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0644)
}

// removePid removes the pid file
func removePid() error {
	pidPath, err := getPidPath()
	if err != nil {
		return err
	}

	err = os.Remove(pidPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// isProcessRunning checks if a process with the given PID is running
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix, FindProcess always succeeds. We need to send signal 0 to check.
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// waitForServer waits for the server to become ready
func waitForServer(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://localhost:%d/health", port)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("server did not become ready within %v", timeout)
}

// determineServerType determines which server type to use based on flags
func determineServerType(cmd *cobra.Command) (string, error) {
	useOSS, _ := cmd.Flags().GetBool("oss")
	useOrkes, _ := cmd.Flags().GetBool("orkes")

	// Check for mutual exclusivity
	if useOSS && useOrkes {
		return "", fmt.Errorf("cannot use both --oss and --orkes flags. Please choose one")
	}

	// Default to OSS if neither is specified
	if !useOSS && !useOrkes {
		return serverTypeOSS, nil
	}

	if useOrkes {
		return serverTypeOrkes, nil
	}

	return serverTypeOSS, nil
}

func startServer(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetInt("port")
	foreground, _ := cmd.Flags().GetBool("foreground")
	version, _ := cmd.Flags().GetString("version")

	// Determine server type
	serverType, err := determineServerType(cmd)
	if err != nil {
		return err
	}

	// Check if already running
	pid, _ := readPid()
	if isProcessRunning(pid) {
		return fmt.Errorf("Conductor server is already running (PID: %d)\nUse 'conductor server stop' to stop it first", pid)
	}

	// Check Java version
	fmt.Println("Checking Java version...")
	javaVersion, err := checkJavaVersion()
	if err != nil {
		return err
	}

	if javaVersion < minJavaVersion {
		return fmt.Errorf("Java %d is installed, but Java %d or higher is required.\n\nPlease upgrade your Java installation:\n  - macOS: brew install openjdk@21\n  - Ubuntu/Debian: sudo apt install openjdk-21-jdk\n  - Windows: Download from https://adoptium.net/", javaVersion, minJavaVersion)
	}

	fmt.Printf("Java %d detected\n", javaVersion)

	// Get JAR download URL
	jarURL, err := getJarDownloadURL(serverType, version)
	if err != nil {
		return err
	}

	// Check if JAR exists, download if not
	jarPath, err := getJarPathForType(serverType, version)
	if err != nil {
		return err
	}

	if _, err := os.Stat(jarPath); os.IsNotExist(err) {
		if err := downloadJar(jarPath, jarURL); err != nil {
			return fmt.Errorf("failed to download server: %w", err)
		}
	} else {
		fmt.Printf("Using cached server JAR: %s\n", jarPath)
	}

	// Get log file path
	logPath, err := getLogPath()
	if err != nil {
		return err
	}

	// Build Java command
	javaArgs := []string{
		"-jar", jarPath,
	}

	// Add port configuration if not default
	if port != defaultPort {
		javaArgs = append(javaArgs, fmt.Sprintf("--server.port=%d", port))
	}

	serverTypeDisplay := "OSS"
	if serverType == serverTypeOrkes {
		serverTypeDisplay = "Orkes"
	}

	if foreground {
		// Run in foreground
		fmt.Printf("Starting %s Conductor server (version: %s) on port %d (foreground mode)...\n", serverTypeDisplay, version, port)
		fmt.Println("Press Ctrl+C to stop")
		fmt.Println()

		javaCmd := exec.Command("java", javaArgs...)
		javaCmd.Stdout = os.Stdout
		javaCmd.Stderr = os.Stderr
		javaCmd.Stdin = os.Stdin

		return javaCmd.Run()
	}

	// Run in background
	fmt.Printf("Starting %s Conductor server (version: %s) on port %d...\n", serverTypeDisplay, version, port)

	// Ensure server directory exists for log file
	serverDir, _ := getServerDir()
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		return fmt.Errorf("failed to create server directory: %w", err)
	}

	// Open log file
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	javaCmd := exec.Command("java", javaArgs...)
	javaCmd.Stdout = logFile
	javaCmd.Stderr = logFile

	// Start the process
	if err := javaCmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Save PID
	if err := writePid(javaCmd.Process.Pid); err != nil {
		// Try to kill the process if we can't save PID
		javaCmd.Process.Kill()
		logFile.Close()
		return fmt.Errorf("failed to save PID: %w", err)
	}

	fmt.Printf("Server starting (PID: %d)...\n", javaCmd.Process.Pid)
	fmt.Printf("Logs: %s\n", logPath)

	// Wait for server to be ready
	fmt.Println("Waiting for server to be ready...")
	if err := waitForServer(port, 60*time.Second); err != nil {
		fmt.Printf("Warning: %v\n", err)
		fmt.Println("The server may still be starting. Check the logs for details.")
	} else {
		fmt.Printf("\nConductor server is ready!\n")
		fmt.Printf("  API: http://localhost:%d/api\n", port)
		fmt.Printf("  UI:  http://localhost:%d\n", port)
	}

	fmt.Printf("\nUse 'conductor server stop' to stop the server\n")
	fmt.Printf("Use 'conductor server logs' to view logs\n")

	// Detach the process (don't wait for it)
	go func() {
		javaCmd.Wait()
		logFile.Close()
	}()

	return nil
}

func stopServer(cmd *cobra.Command, args []string) error {
	pid, err := readPid()
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	if pid == 0 {
		fmt.Println("Conductor server is not running")
		return nil
	}

	if !isProcessRunning(pid) {
		fmt.Printf("Server process (PID: %d) is not running. Cleaning up...\n", pid)
		removePid()
		return nil
	}

	fmt.Printf("Stopping Conductor server (PID: %d)...\n", pid)

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}

	// Wait for process to exit (with timeout)
	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	select {
	case <-done:
		fmt.Println("Server stopped")
	case <-time.After(10 * time.Second):
		// Force kill if graceful shutdown times out
		fmt.Println("Graceful shutdown timed out, forcing...")
		process.Kill()
	}

	// Clean up PID file
	removePid()

	return nil
}

func statusServer(cmd *cobra.Command, args []string) error {
	pid, err := readPid()
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	if pid == 0 || !isProcessRunning(pid) {
		fmt.Println("Conductor server is not running")
		if pid != 0 {
			// Clean up stale PID file
			removePid()
		}
		return nil
	}

	fmt.Printf("Conductor server is running (PID: %d)\n", pid)

	// Try to get server health
	resp, err := http.Get("http://localhost:8080/health")
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			fmt.Println("Status: healthy")
			fmt.Println("  API: http://localhost:8080/api")
			fmt.Println("  UI:  http://localhost:8080")
		} else {
			fmt.Printf("Status: unhealthy (HTTP %d)\n", resp.StatusCode)
		}
	} else {
		fmt.Println("Status: starting or unreachable")
	}

	return nil
}

func logsServer(cmd *cobra.Command, args []string) error {
	follow, _ := cmd.Flags().GetBool("follow")
	lines, _ := cmd.Flags().GetInt("lines")

	logPath, err := getLogPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return fmt.Errorf("no log file found. Has the server been started?")
	}

	if follow {
		// Use tail -f for following logs
		tailCmd := exec.Command("tail", "-f", "-n", strconv.Itoa(lines), logPath)
		tailCmd.Stdout = os.Stdout
		tailCmd.Stderr = os.Stderr
		return tailCmd.Run()
	}

	// Read and print last N lines
	tailCmd := exec.Command("tail", "-n", strconv.Itoa(lines), logPath)
	tailCmd.Stdout = os.Stdout
	tailCmd.Stderr = os.Stderr
	return tailCmd.Run()
}

func init() {
	// Start command flags
	serverStartCmd.Flags().Int("port", defaultPort, "Port to run the server on")
	serverStartCmd.Flags().BoolP("foreground", "f", false, "Run server in foreground (don't daemonize)")
	serverStartCmd.Flags().String("version", "latest", "Server version to download and run (e.g., 'latest', '3.21.23')")
	serverStartCmd.Flags().Bool("oss", false, "Use open-source Conductor server (default)")
	serverStartCmd.Flags().Bool("orkes", false, "Use Orkes Conductor server (coming soon)")
	serverStartCmd.MarkFlagsMutuallyExclusive("oss", "orkes")

	// Logs command flags
	serverLogsCmd.Flags().BoolP("follow", "f", false, "Follow log output (like tail -f)")
	serverLogsCmd.Flags().IntP("lines", "n", 50, "Number of lines to show")

	// Add subcommands
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverStatusCmd)
	serverCmd.AddCommand(serverLogsCmd)

	// Add to root
	rootCmd.AddCommand(serverCmd)
}
