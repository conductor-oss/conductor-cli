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
		Use:          "get",
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
		Use:          "update",
		Short:        "Update Workflow",
		RunE:         updateWorkflowMetadata,
		SilenceUsage: true,
		GroupID:      "metadata",
	}

	createWorkflowMetadataCmd = &cobra.Command{
		Use:          "create",
		Short:        "Create Workflow",
		RunE:         createWorkflowMetadata,
		SilenceUsage: true,
		GroupID:      "metadata",
	}
	deleteWorkflowMetadataCmd = &cobra.Command{
		Use:          "delete",
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
	metadataClient := internal.GetMetadataClient()
	for i := 0; i < len(args); i++ {
		var version *client.MetadataResourceApiGetOpts
		nameAndVersion := strings.Split(args[i], ",")
		name := nameAndVersion[0]
		if len(nameAndVersion) > 1 {
			ver, _ := strconv.Atoi(nameAndVersion[1])
			version = &client.MetadataResourceApiGetOpts{Version: optional.NewInt32(int32(ver))}
		}
		metadata, _, err := metadataClient.Get(context.Background(), html.EscapeString(name), version)
		if err != nil {
			return err
		}
		bytes, _ := json.MarshalIndent(metadata, "", "   ")
		fmt.Println(string(bytes))
	}
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
		return err
	}
	workflowDefs = append(workflowDefs, workflowDef)
	_, err = metadataClient.Update(context.Background(), workflowDefs)
	return err
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
		fmt.Println("Error parsing", string(data))
		return errors.New("Input is not a valid workflow definition: " + err.Error())
	}
	_, err = metadataClient.RegisterWorkflowDef(context.Background(), force, workflowDef)
	return err
}

func deleteWorkflowMetadata(cmd *cobra.Command, args []string) error {
	err := _deleteWorkflowMetadata(cmd, args)
	if err != nil {
		log.Error("Got error ", err)
		return err
	}
	return nil
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
			fmt.Println("OK to delete ", len(workflows), "? [Y]es, [N]o: ")
			answer := "n"
			for {
				answer = readString()
				answer = strings.TrimSpace(answer)
				answer = strings.ToLower(answer)
				log.Info("Got answer: ", answer)
				if answer == "y" || answer == "n" {
					break
				}
			}
			log.Info("Got final answer: ", answer)
		}
		for _, workflow := range workflows {
			a := r.FindAllString(workflow, -1)
			if len(a) != 2 {
				return errors.New("no version specified")
			}
			log.Info(a[0], ",", a[1])
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
			return err
		}
		return nil
	}

	return cmd.Usage()

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
