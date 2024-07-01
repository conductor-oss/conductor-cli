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
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Task Management",
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

	metadataClient := internal.GetMetadataClient()
	tasks, _, err := metadataClient.GetTaskDefs(context.Background())
	verbose, _ := cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if verbose {
			bytes, _ := json.Marshal(task)
			fmt.Println(string(bytes))
		} else {
			fmt.Println(task.Name)
		}

	}
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
		_, err := metadataClient.UnregisterTaskDef(context.Background(), name)
		if err != nil {
			return err
		}
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
