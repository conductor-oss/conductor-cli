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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/conductor-oss/conductor-cli/internal"
	"github.com/conductor-oss/conductor-cli/internal/taskworker"
	"github.com/conductor-sdk/conductor-go/sdk/authentication"
	"github.com/conductor-sdk/conductor-go/sdk/settings"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	workerCmd = &cobra.Command{
		Use:     "worker",
		Short:   "Task worker management",
		Long:    "Commands for managing task workers",
		GroupID: "development",
	}

	workerJsCmd = &cobra.Command{
		Use:          "js <js_file>",
		Short:        "Run a JavaScript worker that polls and processes tasks (EXPERIMENTAL)",
		Long:         "⚠️  EXPERIMENTAL FEATURE - Run a JavaScript worker that continuously polls for tasks of a specific type and executes the provided JavaScript file for each task.",
		RunE:         runJsWorker,
		SilenceUsage: true,
		Example:      "conductor worker js --type my_task worker.js",
	}

	workerStdioCmd = &cobra.Command{
		Use:   "stdio <command> [args...]",
		Short: "Poll tasks and execute command via stdin/stdout",
		Long: `CLI polls tasks and executes the command. The task is passed in the standard input and the result is expected in the standard output.

The worker runs in continuous mode, polling for tasks and executing them in
parallel goroutines.

The task JSON is passed to the command via stdin. The command should read the task
from stdin and write a result JSON to stdout.

Environment variables set for the worker:
  TASK_TYPE      - Type of the task
  TASK_ID        - Task ID
  WORKFLOW_ID    - Workflow ID
  EXECUTION_ID   - Workflow execution ID (same as WORKFLOW_ID)

Expected stdout format:
  {
    "status": "COMPLETED|FAILED|IN_PROGRESS",
    "output": {"key": "value"},
    "logs": ["log line 1", "log line 2"],
    "reason": "failure reason (optional)"
  }

Exit codes:
  0: Task handled successfully (status determines success/failure)
  non-zero: Failure (task marked as FAILED)`,
		RunE:         execWorker,
		SilenceUsage: true,
		Example:      "worker stdio --type greet_task python worker.py\nworker stdio --type greet_task python worker.py --count 5\nworker stdio --type greet_task ./worker.sh --verbose",
	}

	workerRemoteCmd = &cobra.Command{
		Use:   "remote",
		Short: "Run a worker from the job-runner registry (EXPERIMENTAL, Orkes Conductor only)",
		Long: `⚠️  EXPERIMENTAL FEATURE - Download and execute a worker from the Orkes Conductor job-runner.
⚠️  Requires Orkes Conductor. Not available in OSS Conductor.

The worker is downloaded from the configured Conductor server and cached locally for
subsequent runs. Use --refresh to force re-download from the registry.

Supported worker languages:
  - NODEJS: JavaScript/Node.js workers (executed using built-in JavaScript engine)
  - PYTHON: Python workers (executed using python3 interpreter)

The worker runs in continuous mode, polling for tasks and executing them in parallel.`,
		RunE:         runRemoteWorker,
		SilenceUsage: true,
		Example:      "conductor worker remote --type greet_task\nconductor worker remote --type greet_task --count 5 --refresh\nconductor worker remote --type greet_task --worker-id worker-1 --domain prod",
	}

	workerListRemoteCmd = &cobra.Command{
		Use:   "list-remote",
		Short: "List available workers in the job-runner registry (EXPERIMENTAL, Orkes Conductor only)",
		Long: `⚠️  EXPERIMENTAL FEATURE - List all available workers in the Orkes Conductor job-runner registry.
⚠️  Requires Orkes Conductor. Not available in OSS Conductor.`,
		RunE:         listRemoteWorkers,
		SilenceUsage: true,
		Example:      "conductor worker list-remote\nconductor worker list-remote --namespace production",
	}
)

// WorkerCodeResponse represents the response from the job-runner worker-code API
type WorkerCodeResponse struct {
	Id           string    `json:"id"`
	UserId       string    `json:"userId"`
	Namespace    string    `json:"namespace"`
	TaskName     string    `json:"taskName"`
	Language     string    `json:"language"`
	Code         string    `json:"code"`
	Version      int       `json:"version"`
	Description  string    `json:"description"`
	Dependencies []string  `json:"dependencies"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
	CreatedBy    string    `json:"createdBy"`
}

// WorkerMetadata represents cached worker metadata
type WorkerMetadata struct {
	TaskName     string    `json:"taskName"`
	Language     string    `json:"language"`
	Version      int       `json:"version"`
	WorkerCodeId string    `json:"workerCodeId"`
	CachedAt     time.Time `json:"cachedAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func runJsWorker(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("JavaScript file path is required")
	}

	jsFile := args[0]
	taskType, _ := cmd.Flags().GetString("type")
	if taskType == "" {
		return fmt.Errorf("--type flag is required")
	}

	scriptContent, err := os.ReadFile(jsFile)
	if err != nil {
		return fmt.Errorf("error reading JavaScript file: %v", err)
	}

	pollOpts, _ := workerPollFlags(cmd)

	fmt.Printf("Starting worker for task type: %s\n", taskType)
	fmt.Printf("JavaScript file: %s\n", jsFile)
	fmt.Printf("Worker ID: %s\n", pollOpts.WorkerID)

	handler, err := taskworker.NewGojaHandler(string(scriptContent), jsFile)
	if err != nil {
		return err
	}

	return runWorkerLoop(cmd, taskType, handler, jsRunnerOptions(pollOpts))
}

// jsRunnerOptions adjusts poll options for JavaScript workers, which report the polled
// task's own worker id on the result rather than the configured --worker-id.
func jsRunnerOptions(opts taskworker.RunnerOptions) taskworker.RunnerOptions {
	opts.UseTaskWorkerID = true
	return opts
}

func execWorker(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return cmd.Usage()
	}

	taskType, _ := cmd.Flags().GetString("type")
	if taskType == "" {
		return fmt.Errorf("--type flag is required")
	}

	workerCmd := args[0]
	workerArgs := args[1:]

	pollOpts, execTimeout := workerPollFlags(cmd)
	verbose, _ := cmd.Flags().GetBool("verbose")

	fmt.Printf("Starting worker for task type: %s\n", taskType)
	fmt.Printf("Command: %s %v\n", workerCmd, workerArgs)
	if pollOpts.WorkerID != "" {
		fmt.Printf("Worker ID: %s\n", pollOpts.WorkerID)
	}

	handler := taskworker.NewStdioHandler(taskworker.StdioOptions{
		Command:     workerCmd,
		Args:        workerArgs,
		Env:         workerChildEnv(),
		Domain:      pollOpts.Domain,
		ExecTimeout: execTimeout,
		Verbose:     verbose,
	})

	return runWorkerLoop(cmd, taskType, handler, pollOpts)
}

// runWorkerLoop drives a handler with the shared poll loop until the user interrupts it.
// Every worker flavour funnels through here, so the loop exists once rather than per
// flavour.
func runWorkerLoop(cmd *cobra.Command, taskType string, h taskworker.Handler, opts taskworker.RunnerOptions) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()
	stop := interruptWithEscalation(cancel)
	defer stop()

	runner := taskworker.NewConductorRunner(internal.GetTaskClient(), opts)
	taskworker.NewWorker(runner, taskworker.Config{}).Run(ctx, taskType, h)
	return nil
}

// interruptWithEscalation cancels on the first interrupt and force-exits on the second.
//
// signal.NotifyContext alone is not enough here: it keeps the signal channel registered
// after firing once, so the default disposition never returns and further Ctrl-C presses
// are swallowed. A handler that cannot be interrupted — a goja script with no vm.Interrupt
// wired, or a subprocess whose grandchild is holding the captured pipes open — would then
// leave the worker unkillable by anything short of SIGQUIT.
//
// So the first signal asks the loop to drain, and a second one means the user is done
// waiting.
func interruptWithEscalation(cancel context.CancelFunc) (stop func()) {
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		select {
		case <-signals:
			fmt.Fprintln(os.Stderr, "\nShutting down; press Ctrl-C again to exit immediately.")
			cancel()
		case <-done:
			return
		}

		select {
		case <-signals:
			os.Exit(130)
		case <-done:
		}
	}()

	return func() {
		signal.Stop(signals)
		close(done)
	}
}

// workerChildEnv builds the Conductor environment handed to worker subprocesses, so a
// worker can call back into Conductor with the same server and credentials as the CLI.
//
// Resolving viper here keeps process-global config in the cmd layer: internal/taskworker
// receives a plain []string.
func workerChildEnv() []string {
	var env []string

	if serverUrl := viper.GetString("server"); serverUrl != "" {
		serverUrl = strings.TrimSuffix(serverUrl, "/")
		if !strings.HasSuffix(serverUrl, "/api") {
			serverUrl = serverUrl + "/api"
		}
		env = append(env, "CONDUCTOR_SERVER_URL="+serverUrl)
	}
	if authKey := viper.GetString("auth-key"); authKey != "" {
		env = append(env, "CONDUCTOR_ACCESS_KEY_ID="+authKey)
	}
	if authSecret := viper.GetString("auth-secret"); authSecret != "" {
		env = append(env, "CONDUCTOR_ACCESS_KEY_SECRET="+authSecret)
	}
	if authToken := viper.GetString("auth-token"); authToken != "" {
		env = append(env, "CONDUCTOR_AUTH_TOKEN="+authToken)
	}

	return env
}

func listRemoteWorkers(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	namespace, _ := cmd.Flags().GetString("namespace")
	if namespace == "" {
		namespace = "default"
	}

	serverUrl := viper.GetString("server")
	if serverUrl == "" {
		serverUrl = "http://localhost:8080/api"
	}
	serverUrl = strings.TrimSuffix(serverUrl, "/")
	if !strings.HasSuffix(serverUrl, "/api") {
		serverUrl = serverUrl + "/api"
	}

	apiUrl := fmt.Sprintf("%s/worker-code?namespace=%s", serverUrl, namespace)

	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if err := addWorkerAuthHeaders(req); err != nil {
		return fmt.Errorf("failed to add auth headers: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("authentication failed (401 Unauthorized) - verify your credentials")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list workers: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var workers []WorkerCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&workers); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if len(workers) == 0 {
		fmt.Printf("No workers found in namespace: %s\n", namespace)
		return nil
	}

	fmt.Printf("\nAvailable Workers in namespace '%s':\n\n", namespace)
	fmt.Printf("%-30s %-12s %-10s %-50s\n", "TASK NAME", "LANGUAGE", "VERSION", "DESCRIPTION")
	fmt.Println(strings.Repeat("-", 105))

	for _, worker := range workers {
		description := worker.Description
		if len(description) > 47 {
			description = description[:47] + "..."
		}
		fmt.Printf("%-30s %-12s %-10d %-50s\n",
			worker.TaskName,
			worker.Language,
			worker.Version,
			description)
	}

	fmt.Printf("\nTotal: %d workers\n", len(workers))
	return nil
}

func runRemoteWorker(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	taskType, _ := cmd.Flags().GetString("type")
	if taskType == "" {
		return fmt.Errorf("--type flag is required")
	}

	refresh, _ := cmd.Flags().GetBool("refresh")

	// Get or download worker code
	workerFile, language, err := getRemoteWorker(taskType, refresh)
	if err != nil {
		return fmt.Errorf("failed to get worker: %w", err)
	}

	// Execute based on language
	switch language {
	case "NODEJS":
		return executeJsWorkerFromFile(cmd, workerFile, taskType)
	case "PYTHON":
		return executePythonWorkerFromFile(cmd, workerFile, taskType)
	default:
		return fmt.Errorf("unsupported worker language: %s (supported: NODEJS, PYTHON)", language)
	}
}

func getRemoteWorker(taskName string, refresh bool) (string, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to get home directory: %w", err)
	}

	cacheDir := filepath.Join(homeDir, ".conductor-cli", "workers", taskName)
	metadataFile := filepath.Join(cacheDir, ".metadata.json")

	if !refresh {
		if metadata, err := loadMetadata(metadataFile); err == nil {
			workerFile := getWorkerFile(cacheDir, metadata.Language)
			if fileExists(workerFile) {
				log.Infof("Using cached worker '%s' (version %d)", taskName, metadata.Version)
				return workerFile, metadata.Language, nil
			}
		}
	}

	log.Infof("Downloading worker '%s' from registry...", taskName)

	serverUrl := viper.GetString("server")
	if serverUrl == "" {
		serverUrl = "http://localhost:8080/api"
	}
	serverUrl = strings.TrimSuffix(serverUrl, "/")
	if !strings.HasSuffix(serverUrl, "/api") {
		serverUrl = serverUrl + "/api"
	}
	apiUrl := fmt.Sprintf("%s/worker-code/by-name/%s", serverUrl, taskName)

	req, err := http.NewRequest("GET", apiUrl, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	if err := addWorkerAuthHeaders(req); err != nil {
		return "", "", fmt.Errorf("failed to add auth headers: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to connect to registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", "", fmt.Errorf("worker '%s' not found in registry", taskName)
	}
	if resp.StatusCode == 401 {
		return "", "", fmt.Errorf("authentication failed (401 Unauthorized) - verify your credentials")
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("failed to download worker: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var response WorkerCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	workerFile := getWorkerFile(cacheDir, response.Language)
	if err := os.WriteFile(workerFile, []byte(response.Code), 0600); err != nil {
		return "", "", fmt.Errorf("failed to save worker code: %w", err)
	}

	metadata := WorkerMetadata{
		TaskName:     response.TaskName,
		Language:     response.Language,
		Version:      response.Version,
		WorkerCodeId: response.Id,
		CachedAt:     time.Now(),
		UpdatedAt:    response.UpdatedAt,
	}
	metadataJson, _ := json.MarshalIndent(metadata, "", "  ")
	os.WriteFile(metadataFile, metadataJson, 0600)

	log.Infof("Worker downloaded successfully (version %d)", response.Version)

	if response.Language == "PYTHON" {
		log.Infof("Setting up Python environment and installing dependencies...")
		if err := setupPythonEnvironment(cacheDir, response.Dependencies); err != nil {
			log.Warnf("Failed to set up Python environment: %v", err)
			log.Warnf("You may need to manually install Python dependencies")
		} else {
			log.Infof("Python environment ready")
		}
	}

	return workerFile, response.Language, nil
}

func addWorkerAuthHeaders(req *http.Request) error {
	authToken := viper.GetString("auth-token")
	if authToken != "" {
		req.Header.Set("X-Authorization", authToken)
		return nil
	}

	cachedToken := viper.GetString("cached-token")
	if cachedToken != "" {
		req.Header.Set("X-Authorization", cachedToken)
		return nil
	}

	authKey := viper.GetString("auth-key")
	authSecret := viper.GetString("auth-secret")
	if authKey != "" && authSecret != "" {
		token, err := exchangeKeySecretForToken(authKey, authSecret)
		if err != nil {
			return fmt.Errorf("failed to get token from key+secret: %w", err)
		}
		req.Header.Set("X-Authorization", token)
		return nil
	}

	return nil
}

func exchangeKeySecretForToken(key, secret string) (string, error) {
	serverUrl := viper.GetString("server")
	if serverUrl == "" {
		serverUrl = "http://localhost:8080/api"
	}

	serverUrl = strings.TrimSuffix(serverUrl, "/")
	if !strings.HasSuffix(serverUrl, "/api") {
		serverUrl = serverUrl + "/api"
	}

	httpSettings := settings.NewHttpSettings(serverUrl)
	authSettings := settings.NewAuthenticationSettings(key, secret)

	tokenResponse, _, err := authentication.GetToken(*authSettings, httpSettings, &http.Client{Timeout: 30 * time.Second})
	if err != nil {
		return "", fmt.Errorf("authentication failed: %w", err)
	}

	return tokenResponse.Token, nil
}

func getWorkerFile(cacheDir, language string) string {
	ext := map[string]string{
		"NODEJS": ".js",
		"PYTHON": ".py",
		"JAVA":   ".java",
		"GO":     ".go",
	}
	extension := ext[language]
	if extension == "" {
		extension = ".txt"
	}
	return filepath.Join(cacheDir, "worker"+extension)
}

func loadMetadata(metadataFile string) (*WorkerMetadata, error) {
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		return nil, err
	}

	var metadata WorkerMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return &metadata, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func executeJsWorkerFromFile(cmd *cobra.Command, workerFile, taskType string) error {
	scriptContent, err := os.ReadFile(workerFile)
	if err != nil {
		return fmt.Errorf("error reading worker file: %v", err)
	}

	pollOpts, _ := workerPollFlags(cmd)

	log.Infof("Starting JavaScript worker for task type: %s", taskType)
	if pollOpts.WorkerID != "" {
		log.Infof("Worker ID: %s", pollOpts.WorkerID)
	}

	handler, err := taskworker.NewGojaHandler(string(scriptContent), workerFile)
	if err != nil {
		return err
	}

	return runWorkerLoop(cmd, taskType, handler, jsRunnerOptions(pollOpts))
}

func setupPythonEnvironment(cacheDir string, dependencies []string) error {
	venvDir := filepath.Join(cacheDir, "venv")
	requirementsFile := filepath.Join(cacheDir, "requirements.txt")

	allDependencies := append([]string{"conductor-python"}, dependencies...)

	venvExists := fileExists(venvDir)
	requirementsExists := fileExists(requirementsFile)

	var existingRequirements []string
	if requirementsExists {
		data, err := os.ReadFile(requirementsFile)
		if err == nil {
			existingRequirements = strings.Split(strings.TrimSpace(string(data)), "\n")
		}
	}

	requirementsChanged := !equalStringSlices(allDependencies, existingRequirements)

	requirementsContent := strings.Join(allDependencies, "\n")
	if err := os.WriteFile(requirementsFile, []byte(requirementsContent), 0644); err != nil {
		return fmt.Errorf("failed to write requirements.txt: %w", err)
	}

	if !venvExists {
		log.Infof("Creating Python virtual environment...")
		log.Debugf("Running: python3 -m venv %s", venvDir)
		createVenvCmd := exec.Command("python3", "-m", "venv", venvDir)
		createVenvCmd.Dir = cacheDir
		if output, err := createVenvCmd.CombinedOutput(); err != nil {
			log.Errorf("Failed to create venv. Output:\n%s", string(output))
			return fmt.Errorf("failed to create venv: %w", err)
		}
		log.Infof("Virtual environment created successfully")
	}

	if !venvExists || requirementsChanged {
		log.Infof("Installing dependencies: %v", allDependencies)

		pipPath := filepath.Join(venvDir, "bin", "pip")
		if !fileExists(pipPath) {
			return fmt.Errorf("pip not found in venv: %s", pipPath)
		}

		log.Debugf("Running: %s install -r %s", pipPath, requirementsFile)
		installCmd := exec.Command(pipPath, "install", "-r", requirementsFile)
		installCmd.Dir = cacheDir

		output, err := installCmd.CombinedOutput()
		log.Debugf("Pip output:\n%s", string(output))

		if err != nil {
			log.Errorf("Failed to install dependencies. Pip output:\n%s", string(output))
			return fmt.Errorf("failed to install dependencies: %w", err)
		}

		log.Infof("Dependencies installed successfully")
	} else {
		log.Debugf("Virtual environment already configured, skipping dependency installation")
	}

	return nil
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func executePythonWorkerFromFile(cmd *cobra.Command, workerFile, taskType string) error {
	pollOpts, execTimeout := workerPollFlags(cmd)

	pythonCmd := "python3"
	cacheDir := filepath.Dir(workerFile)
	venvPython := filepath.Join(cacheDir, "venv", "bin", "python")

	if fileExists(venvPython) {
		pythonCmd = venvPython
		log.Infof("Using virtual environment Python: %s", venvPython)
	} else {
		log.Infof("Using system Python: python3")
	}

	log.Infof("Starting Python worker for task type: %s", taskType)
	if pollOpts.WorkerID != "" {
		log.Infof("Worker ID: %s", pollOpts.WorkerID)
	}

	handler := taskworker.NewStdioHandler(taskworker.StdioOptions{
		Command:     pythonCmd,
		Args:        []string{workerFile},
		Env:         workerChildEnv(),
		Domain:      pollOpts.Domain,
		ExecTimeout: execTimeout,
	})

	return runWorkerLoop(cmd, taskType, handler, pollOpts)
}

// workerPollFlags reads the poll and execution flags shared by the worker subcommands.
//
// --poll-timeout and --exec-timeout are the canonical names. --timeout is a deprecated
// alias for --poll-timeout, kept because `worker js` and `worker remote` shipped with it.
// Previously `worker remote` fed one --timeout value to both, so the same number meant
// milliseconds of poll wait and seconds of execution budget at once (issue #91).
func workerPollFlags(cmd *cobra.Command) (taskworker.RunnerOptions, time.Duration) {
	opts := taskworker.RunnerOptions{}
	opts.WorkerID, _ = cmd.Flags().GetString("worker-id")
	opts.Domain, _ = cmd.Flags().GetString("domain")
	opts.Count, _ = cmd.Flags().GetInt32("count")

	opts.PollTimeoutMs, _ = cmd.Flags().GetInt32("poll-timeout")
	if cmd.Flags().Changed("timeout") && !cmd.Flags().Changed("poll-timeout") {
		legacy, _ := cmd.Flags().GetInt32("timeout")
		opts.PollTimeoutMs = legacy
	}

	execSeconds, _ := cmd.Flags().GetInt32("exec-timeout")
	return opts, time.Duration(execSeconds) * time.Second
}

// addPollTimeoutFlags registers the two timeout flags on a worker subcommand, plus the
// deprecated --timeout alias that `worker js` and `worker remote` shipped with.
//
// The alias is hidden rather than removed so existing invocations keep working; it maps
// to --poll-timeout only. See workerPollFlags and issue #91.
func addPollTimeoutFlags(cmd *cobra.Command, execTimeout bool, execTimeoutDefault int32) {
	cmd.Flags().Int32("poll-timeout", 100, "Poll timeout in milliseconds")
	if execTimeout {
		cmd.Flags().Int32("exec-timeout", execTimeoutDefault, "Worker execution timeout in seconds (0 = no timeout)")
	}
	cmd.Flags().Int32("timeout", 100, "Deprecated: use --poll-timeout")
	_ = cmd.Flags().MarkHidden("timeout")
	_ = cmd.Flags().MarkDeprecated("timeout", "use --poll-timeout instead")
}

func init() {
	workerJsCmd.Flags().String("type", "", "Task type to poll for (required)")
	workerJsCmd.MarkFlagRequired("type")
	workerJsCmd.Flags().Int32("count", 1, "Number of tasks to poll in each batch")
	workerJsCmd.Flags().String("worker-id", "", "Worker ID")
	workerJsCmd.Flags().String("domain", "", "Domain")
	addPollTimeoutFlags(workerJsCmd, false, 0)

	workerStdioCmd.Flags().String("type", "", "Task type to poll for (required)")
	workerStdioCmd.MarkFlagRequired("type")
	workerStdioCmd.Flags().String("worker-id", "", "Worker ID")
	workerStdioCmd.Flags().String("domain", "", "Domain")
	workerStdioCmd.Flags().Int32("count", 1, "Number of tasks to poll in each batch")
	workerStdioCmd.Flags().Bool("verbose", false, "Print task and result JSON to stdout")
	addPollTimeoutFlags(workerStdioCmd, true, 0)

	workerRemoteCmd.Flags().String("type", "", "Task type to poll for (required)")
	workerRemoteCmd.MarkFlagRequired("type")
	workerRemoteCmd.Flags().Int32("count", 1, "Number of tasks to poll in each batch")
	workerRemoteCmd.Flags().String("worker-id", "", "Worker ID")
	workerRemoteCmd.Flags().String("domain", "", "Domain")
	workerRemoteCmd.Flags().Bool("refresh", false, "Force refresh worker from registry (ignore cache)")
	// Remote workers previously derived their execution timeout from --timeout, whose
	// default was 100. Defaulting --exec-timeout to 100s keeps a hanging remote worker
	// bounded as it was before the two timeouts were separated.
	addPollTimeoutFlags(workerRemoteCmd, true, 100)

	workerListRemoteCmd.Flags().String("namespace", "default", "Namespace to list workers from")

	workerCmd.AddCommand(workerJsCmd)
	workerCmd.AddCommand(workerStdioCmd)
	workerCmd.AddCommand(workerRemoteCmd)
	workerCmd.AddCommand(workerListRemoteCmd)
	rootCmd.AddCommand(workerCmd)
}
