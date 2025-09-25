package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/antihax/optional"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/orkes-io/conductor-cli/internal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"html"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Workflow Management",
}

var (

	//Workflow Metadata Management
	listWorkflowsMetadataCmd = &cobra.Command{
		Use:          "list",
		Short:        "List workflows",
		RunE:         listWorkflow,
		SilenceUsage: true,
		GroupID:      "metadata",
	}
	getWorkflowMetadataCmd = &cobra.Command{
		Use:          "get <workflow_name> <version>",
		Short:        "Get Workflow",
		RunE:         getWorkflowMetadata,
		SilenceUsage: true,
		GroupID:      "metadata",
	}

	getAllWorkflowMetadataCmd = &cobra.Command{
		Use:          "get_all",
		Short:        "Get All Workflows",
		RunE:         getAllWorkflowMetadata,
		SilenceUsage: true,
		GroupID:      "metadata",
	}

	updateWorkflowMetadataCmd = &cobra.Command{
		Use:          "update <workflow_definition.json>",
		Short:        "Update Workflow",
		RunE:         updateWorkflowMetadata,
		SilenceUsage: true,
		GroupID:      "metadata",
	}

	createWorkflowMetadataCmd = &cobra.Command{
		Use:          "create <workflow_definition.json>",
		Short:        "Create Workflow",
		RunE:         createWorkflowMetadata,
		SilenceUsage: true,
		GroupID:      "metadata",
	}
	deleteWorkflowMetadataCmd = &cobra.Command{
		Use:          "delete <workflow_name> <version>",
		Short:        "Delete Workflow",
		RunE:         deleteWorkflowMetadata,
		SilenceUsage: true,
		GroupID:      "metadata",
	}
)

func listWorkflow(cmd *cobra.Command, args []string) error {

	metadataClient := internal.GetMetadataClient()
	workflows, _, err := metadataClient.GetAll(context.Background())
	if err != nil {
		return err
	}
	for _, workflow := range workflows {
		workflowName := workflow.Name
		if strings.Contains(workflow.Name, " ") {
			workflowName = fmt.Sprintf("\"%s\"", strings.ReplaceAll(workflow.Name, "\"", "\\"))
		}
		fmt.Println(fmt.Sprintf("%s %d", workflowName, workflow.Version))
	}
	return nil
}

func getWorkflowMetadata(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return cmd.Usage()
	}
	
	metadataClient := internal.GetMetadataClient()
	name := args[0]
	version, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("invalid version '%s': must be a number", args[1])
	}
	
	versionOpts := &client.MetadataResourceApiGetOpts{Version: optional.NewInt32(int32(version))}
	metadata, _, err := metadataClient.Get(context.Background(), html.EscapeString(name), versionOpts)
	if err != nil {
		return parseAPIError(err, fmt.Sprintf("Failed to get workflow '%s' version %d", name, version))
	}
	
	bytes, _ := json.MarshalIndent(metadata, "", "   ")
	fmt.Println(string(bytes))
	return nil
}

func getAllWorkflowMetadata(cmd *cobra.Command, args []string) error {
	metadataClient := internal.GetMetadataClient()
	var expr string
	var regex *regexp.Regexp
	if len(args) == 1 {
		var err error
		expr = args[0]
		regex, err = regexp.Compile(expr)
		if err != nil {
			return err
		}

	}

	metadata, _, err := metadataClient.GetAll(context.Background())
	if err != nil {
		return err
	}
	fmt.Println("[")
	for i, data := range metadata {
		if regex != nil {
			if regex.Match([]byte(data.Name)) {
				bytes, _ := json.MarshalIndent(data, "", "   ")
				fmt.Println(string(bytes))
			}
		} else {
			bytes, _ := json.MarshalIndent(data, "", "   ")
			fmt.Println(string(bytes))
		}
		if i < len(metadata)-1 {
			fmt.Print(",")
		}
	}
	fmt.Println("]")

	return nil
}

func updateWorkflowMetadata(cmd *cobra.Command, args []string) error {
	metadataClient := internal.GetMetadataClient()
	var data []byte
	var err error
	if len(args) == 1 {
		data, err = os.ReadFile(args[0])
		if err != nil {
			return err
		}
	} else {
		// Check if stdin has data available
		stat, err := os.Stdin.Stat()
		if err != nil {
			return fmt.Errorf("error checking stdin: %v", err)
		}
		
		// If running interactively (no pipe/redirect), show usage
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return cmd.Usage()
		}
		
		data = read()
		if len(data) == 0 {
			return fmt.Errorf("no workflow data received from stdin")
		}
	}
	var workflowDefs []model.WorkflowDef

	javascript, _ := cmd.Flags().GetBool("js")
	if javascript {
		js := string(data)
		workflowJson, conversionError := getWorkflowFromJS(js)
		if conversionError != nil {
			return conversionError
		}
		data = []byte(workflowJson)
	}

	var workflowDef model.WorkflowDef
	err = json.Unmarshal(data, &workflowDef)
	if err != nil {
		return parseJSONError(err, string(data), "workflow definition")
	}
	workflowDefs = append(workflowDefs, workflowDef)
	_, err = metadataClient.Update(context.Background(), workflowDefs)
	if err != nil {
		return parseAPIError(err, "Failed to update workflow")
	}
	return nil
}

func createWorkflowMetadata(cmd *cobra.Command, args []string) error {
	var data []byte
	var err error
	if len(args) == 1 {
		data, err = os.ReadFile(args[0])
		if err != nil {
			fmt.Println("Error reading file ", err.Error())
			return err
		}
	} else {
		// Check if stdin has data available
		stat, err := os.Stdin.Stat()
		if err != nil {
			return fmt.Errorf("error checking stdin: %v", err)
		}
		
		// If running interactively (no pipe/redirect), show usage
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return cmd.Usage()
		}
		
		data = read()
		if len(data) == 0 {
			return fmt.Errorf("no workflow data received from stdin")
		}
	}
	js := string(data)
	var json string
	javascript, _ := cmd.Flags().GetBool("js")
	if javascript {
		json, err = getWorkflowFromJS(js)
		data = []byte(json)
		if err != nil {
			return err
		}
	}
	err = registerWorkflow(data, true)
	return err
}

func registerWorkflow(data []byte, force bool) error {
	metadataClient := internal.GetMetadataClient()
	var workflowDef model.WorkflowDef
	err := json.Unmarshal(data, &workflowDef)
	if err != nil {
		return parseJSONError(err, string(data), "workflow definition")
	}
	_, err = metadataClient.RegisterWorkflowDef(context.Background(), force, workflowDef)
	if err != nil {
		return parseAPIError(err, "Failed to create workflow")
	}
	return nil
}

func deleteWorkflowMetadata(cmd *cobra.Command, args []string) error {
	return _deleteWorkflowMetadata(cmd, args)
}
func _deleteWorkflowMetadata(cmd *cobra.Command, args []string) error {
	metadataClient := internal.GetMetadataClient()
	if len(args) == 0 {
		// Check if stdin has data available
		stat, err := os.Stdin.Stat()
		if err != nil {
			return fmt.Errorf("error checking stdin: %v", err)
		}
		
		// If running interactively (no pipe/redirect), show usage
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			return cmd.Usage()
		}
		
		workflows := readLines()
		if len(workflows) == 0 {
			return fmt.Errorf("no workflow data received from stdin")
		}
		log.Info("Read ", len(workflows), " from console")
		r := regexp.MustCompile(`[^\s"]+|"([^"]*)"`)
		if !yes {
			// When piped input, auto-confirm since user can't interact
			fmt.Printf("Auto-confirming deletion of %d workflow(s) from piped input. Use --yes to skip confirmation.\n", len(workflows))
		}
		for _, workflow := range workflows {
			a := r.FindAllString(workflow, -1)
			if len(a) != 2 {
				return errors.New("no version specified")
			}
			name := a[0]
			version, err := strconv.Atoi(a[1])
			if err != nil {
				return fmt.Errorf("invalid version '%s' for workflow '%s': %v", a[1], name, err)
			}
			log.Info("Deleting workflow: ", name, " version: ", version)
			_, err = metadataClient.UnregisterWorkflowDef(context.Background(), name, int32(version))
			if err != nil {
				return parseAPIError(err, fmt.Sprintf("Failed to delete workflow '%s' version %d", name, version))
			}
		}
		return nil
	} else if len(args) == 2 {
		name := args[0]
		version, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}
		_, err = metadataClient.UnregisterWorkflowDef(context.Background(), name, int32(version))
		if err != nil {
			return parseAPIError(err, fmt.Sprintf("Failed to delete workflow '%s' version %d", name, version))
		}
		return nil
	}

	return cmd.Usage()

}

// parseJSONError provides helpful error messages for JSON parsing failures
func parseJSONError(err error, jsonContent string, contextName string) error {
	errStr := err.Error()
	
	// Common JSON syntax error patterns
	if strings.Contains(errStr, "invalid character") && strings.Contains(errStr, "in string literal") {
		// Find the approximate line number by counting newlines
		lines := strings.Split(jsonContent, "\n")
		
		// Look for unterminated strings (missing quotes)
		for i, line := range lines {
			// Simple heuristic: look for lines with odd number of quotes
			quoteCount := strings.Count(line, "\"") - strings.Count(line, "\\\"")
			if quoteCount%2 != 0 && strings.Contains(line, ":") {
				return fmt.Errorf("JSON syntax error in %s: unterminated string on line %d\nLine content: %s\nHint: Check for missing closing quote (\") on this line", contextName, i+1, strings.TrimSpace(line))
			}
		}
		
		return fmt.Errorf("JSON syntax error in %s: %s\nHint: Check for unterminated strings (missing quotes)", contextName, errStr)
	}
	
	if strings.Contains(errStr, "unexpected end of JSON input") {
		return fmt.Errorf("JSON syntax error in %s: unexpected end of file\nHint: Check for missing closing braces } or brackets ]", contextName)
	}
	
	if strings.Contains(errStr, "invalid character") {
		return fmt.Errorf("JSON syntax error in %s: %s\nHint: Check for invalid characters, missing commas, or malformed values", contextName, errStr)
	}
	
	// Fallback for other JSON errors
	return fmt.Errorf("Invalid %s format: %s", contextName, errStr)
}

// parseAPIError extracts useful error information from API responses
func parseAPIError(err error, defaultMsg string) error {
	errStr := err.Error()
	
	// Try to extract JSON from error message
	// Error format: "error: {...}, body: {...}"
	var jsonStr string
	if strings.Contains(errStr, "body: {") {
		// Extract the body part
		parts := strings.Split(errStr, "body: ")
		if len(parts) > 1 {
			jsonStr = parts[1]
		}
	} else if strings.Contains(errStr, "error: {") {
		// Extract the error part  
		parts := strings.Split(errStr, "error: ")
		if len(parts) > 1 {
			jsonStr = strings.Split(parts[1], ", body:")[0]
		}
	}
	
	if jsonStr != "" {
		// Try to parse the JSON with validation errors
		var errorResponse struct {
			Status           int    `json:"status"`
			Message          string `json:"message"`
			ValidationErrors []struct {
				Path    string `json:"path"`
				Message string `json:"message"`
			} `json:"validationErrors"`
		}
		
		if json.Unmarshal([]byte(jsonStr), &errorResponse) == nil {
			if errorResponse.Message != "" {
				message := fmt.Sprintf("%s: %s", defaultMsg, errorResponse.Message)
				
				// Add validation error details if available
				if len(errorResponse.ValidationErrors) > 0 {
					message += "\nValidation errors:"
					for _, validationErr := range errorResponse.ValidationErrors {
						if validationErr.Path != "" {
							message += fmt.Sprintf("\n  - %s: %s", validationErr.Path, validationErr.Message)
						} else {
							message += fmt.Sprintf("\n  - %s", validationErr.Message)
						}
					}
				}
				
				if errorResponse.Status > 0 {
					message += fmt.Sprintf(" (status: %d)", errorResponse.Status)
				}
				
				return fmt.Errorf(message)
			}
		}
	}
	
	// Fallback to original error if parsing fails
	return fmt.Errorf("%s: %v", defaultMsg, err)
}

func read() []byte {
	var b bytes.Buffer
	in := bufio.NewReader(os.Stdin)
	defer os.Stdin.Close()
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			// If we have a partial line (no newline at EOF), include it
			if len(line) > 0 {
				b.WriteString(line)
			}
			break
		}
		b.WriteString(line)
	}
	return b.Bytes()
}

func readString() string {
	var b bytes.Buffer
	in := bufio.NewReader(os.Stdin)
	defer os.Stdin.Close()
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			// If we have a partial line (no newline at EOF), include it
			if len(line) > 0 {
				b.WriteString(line)
			}
			break
		}
		b.WriteString(line)
	}
	return b.String()
}

func readLines() []string {
	var lines = make([]string, 0)
	in := bufio.NewReader(os.Stdin)
	defer os.Stdin.Close()
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			// If we have a partial line (no newline at EOF), include it
			if len(line) > 0 {
				line = strings.TrimSpace(line)
				if len(line) > 0 {
					lines = append(lines, line)
				}
			}
			break
		}
		line = strings.TrimSpace(line)
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}

var script = `
const wf = workflow();
wf.tasks.forEach(t => {
    if(t.function != null) {
        t.type = "INLINE";
        t.inputParameters["expression"] = t.function.toString() + "\n" + t.function.name + "();";
        t.inputParameters["evaluatorType"] = "graaljs";
    } else if (t.type == "WAIT") {
		if(t.name == null) t.name = "WAIT";
		if(t.duration != null) {
			if(t.inputParameters == null) t.inputParameters = {};
			t.inputParameters["duration"] = t.duration;
		}
	}
    if(t.taskReferenceName == null) {
        t.taskReferenceName = t.name;
    }

});
JSON.stringify(wf);
`

func getWorkflowFromJS(js string) (string, error) {
	vm := goja.New()
	new(require.Registry).Enable(vm)
	console.Enable(vm)

	updatedScript := js + "\n" + script
	prog, err := goja.Compile("", updatedScript, true)
	if err != nil {
		fmt.Printf("Error compiling the script %v ", err)
		return "", err
	}
	value, err := vm.RunProgram(prog)
	return value.String(), err
}

func init() {
	rootCmd.AddCommand(workflowCmd)
	createWorkflowMetadataCmd.Flags().Bool("force", false, "--force overwrite existing workflow")
	createWorkflowMetadataCmd.Flags().Bool("js", false, "Input is javascript file")
	createWorkflowMetadataCmd.Flags().Bool("json", true, "Input is json file")
	createWorkflowMetadataCmd.MarkFlagsMutuallyExclusive("js", "json")
	workflowCmd.AddGroup(&cobra.Group{
		ID:    "metadata",
		Title: "Metadata Commands",
	})
	workflowCmd.AddCommand(
		listWorkflowsMetadataCmd,
		getWorkflowMetadataCmd,
		getAllWorkflowMetadataCmd,
		updateWorkflowMetadataCmd,
		createWorkflowMetadataCmd,
		deleteWorkflowMetadataCmd,
	)
}
