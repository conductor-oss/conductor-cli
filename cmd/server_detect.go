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
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

// detectOrPromptServer tries to find a running local Conductor server.
// If none is found and stdin is a TTY, offers to start one.
// Returns the server API URL (e.g., "http://localhost:8080/api") or an error.
func detectOrPromptServer() (string, error) {
	// Try to detect a running server — check the CLI-managed server port first, then default
	portsToCheck := []int{}

	// Check if there's a CLI-managed server with a known port
	if port := readServerPort(); port > 0 && port != defaultPort {
		portsToCheck = append(portsToCheck, port)
	}
	portsToCheck = append(portsToCheck, defaultPort)

	for _, port := range portsToCheck {
		if checkLocalServer(port) {
			url := fmt.Sprintf("http://localhost:%d/api", port)
			fmt.Fprintf(os.Stderr, "Auto-detected Conductor server at http://localhost:%d\n", port)
			return url, nil
		}
	}

	// No server found — behavior depends on whether we're interactive
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return promptAndStartServer()
	}

	return "", fmt.Errorf("no Conductor server configured.\nSet CONDUCTOR_SERVER_URL, use --profile, or run 'conductor server start' first")
}

// checkLocalServer performs a quick health check against localhost on the given port.
func checkLocalServer(port int) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// readServerPort reads the port from the server state file.
// Returns 0 if the port cannot be determined.
func readServerPort() int {
	state := loadServerState()
	if state.Port > 0 {
		return state.Port
	}
	return 0
}

// promptAndStartServer asks the user if they want to start a local server,
// and starts one if they agree.
func promptAndStartServer() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stdout, "No Conductor server configured. Start a local server? [Y/n]: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input != "" && input != "y" && input != "yes" {
		return "", fmt.Errorf("no Conductor server configured.\nSet CONDUCTOR_SERVER_URL, use --profile, or run 'conductor server start' first")
	}

	port := defaultPort
	if err := startLocalServer(port); err != nil {
		return "", fmt.Errorf("failed to start local server: %w", err)
	}

	return fmt.Sprintf("http://localhost:%d/api", port), nil
}
