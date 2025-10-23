package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/orkes-io/conductor-cli/internal"
	"github.com/spf13/cobra"
	"os"
	"regexp"
	"text/tabwriter"
)

var taskCmd = &cobra.Command{
	Use:     "task",
	Short:   "Task definition management",
	GroupID: "metadata",
}

var (

	//Task Metadata Management
	listTaskMetadataCmd = &cobra.Command{
		Use:          "list",
		Short:        "List Tasks Definitions",
		RunE:         listTasks,
		SilenceUsage: true,
	}

	getTaskMetadataCmd = &cobra.Command{
		Use:          "get",
		Short:        "Get Tasks Definition",
		RunE:         getTask,
		SilenceUsage: true,
		Example:      "get [task_type]",
	}

	deleteTaskMetadataCmd = &cobra.Command{
		Use:          "delete",
		Short:        "Delete Tasks Definition",
		RunE:         deleteTaskMetadata,
		SilenceUsage: true,
		Example:      "delete [task_type]",
	}

	createTaskMetadataCmd = &cobra.Command{
		Use:          "create",
		Short:        "Register Tasks Definition",
		RunE:         createTask,
		SilenceUsage: true,
		Example:      "create [task definition file or stream]",
	}

	updateTaskMetadataCmd = &cobra.Command{
		Use:          "update",
		Short:        "Update Tasks Definition",
		RunE:         updateTask,
		SilenceUsage: true,
		Example:      "update [task definition file or stream]",
	}

	getAllTaskMetadataCmd = &cobra.Command{
		Use:          "get_all",
		Short:        "Get all Tasks definitions",
		RunE:         getAllTasks,
		SilenceUsage: true,
	}
)

func listTasks(cmd *cobra.Command, args []string) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")

	metadataClient := internal.GetMetadataClient()
	tasks, _, err := metadataClient.GetTaskDefs(context.Background())
	if err != nil {
		return err
	}

	if jsonOutput {
		data, err := json.MarshalIndent(tasks, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling tasks: %v", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Print as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tEXECUTABLE\tDESCRIPTION\tOWNER\tTIMEOUT POLICY\tTIMEOUT (s)\tRETRY COUNT\tRESPONSE TIMEOUT (s)")
	for _, task := range tasks {
		// Determine if task is executable (has executionNameSpace or ownerApp)
		executable := "no"
		if task.ExecutionNameSpace != "" || task.OwnerApp != "" {
			executable = "yes"
		}

		description := task.Description
		if description == "" {
			description = "-"
		}
		// Truncate long descriptions
		if len(description) > 30 {
			description = description[:27] + "..."
		}

		ownerEmail := task.OwnerEmail
		if ownerEmail == "" {
			ownerEmail = "-"
		}

		timeoutPolicy := task.TimeoutPolicy
		if timeoutPolicy == "" {
			timeoutPolicy = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\n",
			task.Name,
			executable,
			description,
			ownerEmail,
			timeoutPolicy,
			task.TimeoutSeconds,
			task.RetryCount,
			task.ResponseTimeoutSeconds,
		)
	}
	w.Flush()

	return nil
}

func getTask(cmd *cobra.Command, args []string) error {

	metadataClient := internal.GetMetadataClient()
	if len(args) != 1 {
		return cmd.Usage()
	}
	taskType := args[0]
	task, _, err := metadataClient.GetTaskDef(context.Background(), taskType)
	if err != nil {
		return err
	}
	bytes, _ := json.MarshalIndent(task, "", "   ")
	fmt.Println(string(bytes))
	return nil
}

func createTask(cmd *cobra.Command, args []string) error {

	metadataClient := internal.GetMetadataClient()
	var taskDefs []model.TaskDef
	var bytes []byte
	var err error
	if len(args) == 1 {
		file := args[0]
		bytes, err = os.ReadFile(file)
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

		bytes = read()
	}
	err = json.Unmarshal(bytes, &taskDefs)
	if err != nil {
		var taskDef model.TaskDef
		err = json.Unmarshal(bytes, &taskDef)
		if err != nil {
			return err
		}
		taskDefs = append(taskDefs, taskDef)
	}
	_, err = metadataClient.RegisterTaskDef(context.Background(), taskDefs)
	if err != nil {
		return err
	}
	return nil
}

func updateTask(cmd *cobra.Command, args []string) error {

	metadataClient := internal.GetMetadataClient()
	var taskDef model.TaskDef
	var bytes []byte
	var err error
	if len(args) == 1 {
		file := args[0]
		bytes, err = os.ReadFile(file)
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

		bytes = read()
	}
	err = json.Unmarshal(bytes, &taskDef)
	if err != nil {
		return err
	}
	_, err = metadataClient.UpdateTaskDef(context.Background(), taskDef)
	if err != nil {
		return err
	}
	return nil
}

func getAllTasks(cmd *cobra.Command, args []string) error {

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

	taskDefs, _, err := metadataClient.GetTaskDefs(context.Background())
	if err != nil {
		return err
	}

	fmt.Println("[")
	for _, data := range taskDefs {
		if regex != nil {
			if regex.Match([]byte(data.Name)) {
				bytes, _ := json.MarshalIndent(data, "", "   ")
				fmt.Println(string(bytes))
			}
		} else {
			bytes, _ := json.MarshalIndent(data, "", "   ")
			fmt.Println(string(bytes))
		}

	}
	fmt.Println("]")

	return nil
}

func deleteTaskMetadata(cmd *cobra.Command, args []string) error {
	metadataClient := internal.GetMetadataClient()
	if len(args) == 0 {
		return cmd.Usage()
	}
	for i := 0; i < len(args); i++ {
		name := args[i]

		// Confirm deletion
		if !confirmDeletion("task", name) {
			fmt.Printf("Skipping deletion of task '%s'\n", name)
			continue
		}

		_, err := metadataClient.UnregisterTaskDef(context.Background(), name)
		if err != nil {
			return err
		}
		fmt.Printf("Task '%s' deleted successfully\n", name)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(taskCmd)
	listTaskMetadataCmd.Flags().Bool("json", false, "Print json task listing")
	taskCmd.AddCommand(
		listTaskMetadataCmd,
		getTaskMetadataCmd,
		createTaskMetadataCmd,
		getAllTaskMetadataCmd,
		deleteTaskMetadataCmd,
		updateTaskMetadataCmd,
	)
}
