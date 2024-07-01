package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/antihax/optional"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/google/uuid"
	"github.com/orkes-io/conductor-cli/internal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
)

var (

	//Workflow Runtime
	searchWorkflowCmd = &cobra.Command{
		Use:          "search",
		Short:        "Search for workflows",
		RunE:         searchWorkflowExecutions,
		SilenceUsage: true,
		Example:      "search [flags] search_text",
	}

	getWorkflowCmd = &cobra.Command{
		Use:          "status",
		Short:        "Get Workflow Status",
		RunE:         getWorkflowExecution,
		SilenceUsage: true,
		Example:      "status [flags] [workflow_id] [workflow_id2]...",
	}

	startWorkflowCmd = &cobra.Command{
		Use:          "start",
		Short:        "Start Workflow",
		RunE:         startWorkflow,
		SilenceUsage: true,
		Example:      "start [flags]",
	}

	executeWorkflowCmd = &cobra.Command{
		Use:          "execute",
		Short:        "Execute Workflow and get output",
		RunE:         startWorkflow,
		SilenceUsage: true,
		Example:      "execute [flags]",
	}

	terminateWorkflowCmd = &cobra.Command{
		Use:          "terminate",
		Short:        "Terminates a running workflow",
		RunE:         terminateWorkflow,
		SilenceUsage: true,
		Example:      "terminate [flags]",
	}
)

func searchWorkflowExecutions(cmd *cobra.Command, args []string) error {

	workflowClient := internal.GetWorkflowClient()

	freeText := "*"
	if len(args) == 1 {
		freeText = args[0]
	}

	count, _ := cmd.Flags().GetInt32("count")
	if count > 1000 {
		//fmt.Println("count exceeds max allowed 1000.  Will only show the first 1000 matching results")
		//count = 1000
	} else if count == 0 {
		count = 10
	}

	query := ""
	workflowName, _ := cmd.Flags().GetString("workflow")
	if workflowName != "" {
		query = query + " workflowType IN (" + workflowName + ") "
	}
	status, _ := cmd.Flags().GetString("status")
	if status != "" {
		if query != "" {
			query = query + " AND status IN (" + status + ") "
		} else {
			query = query + " status IN (" + status + ") "
		}
	}

	query = query + " AND startTime>1716706800000 AND startTime<1716965999000 "

	searchOpts := client.WorkflowResourceApiSearchOpts{
		Start:    optional.NewInt32(0),
		Size:     optional.NewInt32(count),
		FreeText: optional.NewString(freeText),
		Query:    optional.NewString(query),
		Sort:     optional.NewString("startTime:DESC"),
	}

	results, _, err := workflowClient.Search(context.Background(), &searchOpts)
	for _, item := range results.Results {
		fmt.Println(item.WorkflowId)
	}
	if err != nil {
		return err
	}
	return nil
}

func getWorkflowExecution(cmd *cobra.Command, args []string) error {

	if len(args) == 0 {
		return cmd.Usage()
	}
	workflowClient := internal.GetWorkflowClient()

	full, _ := cmd.Flags().GetBool("complete")
	for i := 0; i < len(args); i++ {
		id := args[i]
		if full {

			options := &client.WorkflowResourceApiGetExecutionStatusOpts{IncludeTasks: optional.NewBool(true)}
			status, _, err := workflowClient.GetExecutionStatus(context.Background(), id, options)
			if err != nil {
				return err
			}
			data, marshallError := json.MarshalIndent(status, "", "   ")
			if marshallError != nil {
				return marshallError
			}
			fmt.Println(string(data))

		} else {
			status, _, getStateErr := workflowClient.GetWorkflowState(context.Background(), id, true, true)
			if getStateErr != nil {
				return getStateErr
			}
			data, marshallError := json.MarshalIndent(status, "", "   ")
			if marshallError != nil {
				return marshallError
			}
			fmt.Println(string(data))
		}

	}
	return nil
}

func terminateWorkflow(cmd *cobra.Command, args []string) error {

	if len(args) == 0 {
		return cmd.Usage()
	}
	workflowClient := internal.GetWorkflowClient()
	for i := 0; i < len(args); i++ {
		id := args[i]
		fmt.Println(id)
		options := &client.WorkflowResourceApiTerminateOpts{
			Reason: optional.NewString("Terminated by background process"),
		}
		_, err := workflowClient.Terminate(context.Background(), id, options)
		if err != nil {
			fmt.Println("error terminating", id, err.Error())
		}
	}

	return nil
}

func startWorkflow(cmd *cobra.Command, args []string) error {

	workflowName, _ := cmd.Flags().GetString("workflow")
	version, _ := cmd.Flags().GetInt32("version")
	input, _ := cmd.Flags().GetString("input")
	inputFile, _ := cmd.Flags().GetString("file")
	correlationId, _ := cmd.Flags().GetString("correlation")

	if workflowName == "" {
		if len(args) == 1 {
			workflowName = args[0]
		} else {
			return cmd.Usage()
		}
	}

	var inputJson []byte
	var err error

	if input != "" {
		inputJson = []byte(input)
	} else if inputFile != "" {
		inputJson, err = os.ReadFile(inputFile)
		if err != nil {
			return err
		}
	}

	if inputJson == nil {
		inputJson = []byte("{}")
	}

	var inputMap map[string]interface{}
	err = json.Unmarshal(inputJson, &inputMap)
	if err != nil {
		return err
	}

	opts := &client.WorkflowResourceApiStartWorkflowOpts{}
	if version > 0 {
		opts.Version = optional.NewInt32(int32(version))
	}
	if correlationId != "" {
		opts.CorrelationId = optional.NewString(correlationId)
	}

	workflowClient := internal.GetWorkflowClient()
	executeSync, notFound := cmd.Flags().GetBool("sync")
	if notFound != nil && executeSync {
		requestId, _ := uuid.NewRandom()
		request := model.StartWorkflowRequest{
			Name:          workflowName,
			Version:       version,
			CorrelationId: "",
			Input:         inputMap,
			Priority:      0,
		}
		waitUntil, _ := cmd.Flags().GetString("wait-until")
		log.Debug("wait until ", waitUntil)
		run, _, execErr := workflowClient.ExecuteWorkflow(context.Background(), request, requestId.String(), workflowName, version, waitUntil)
		if execErr != nil {
			return execErr
		}
		data, jsonError := json.MarshalIndent(run, "", "   ")
		if jsonError != nil {
			return jsonError
		}
		fmt.Println(string(data))

	} else {
		workflowId, _, startErr := workflowClient.StartWorkflow(cmd.Context(), inputMap, workflowName, opts)
		if startErr != nil {
			return startErr
		}
		fmt.Println("http://localhost:5000/execution/" + workflowId)
	}

	return nil
}

func init() {
	searchWorkflowCmd.Flags().Int32P("count", "c", 10, "No of workflows to return (max 1000)")
	searchWorkflowCmd.Flags().StringP("status", "s", "", "Filter by status one of (EXECUTED, FAILED)")
	searchWorkflowCmd.Flags().StringP("workflow", "w", "", "Workflow name")

	startWorkflowCmd.Flags().StringP("workflow", "w", "", "Workflow name")
	startWorkflowCmd.Flags().StringP("input", "i", "", "Input json")
	startWorkflowCmd.Flags().StringP("file", "f", "", "Input file with json data")
	startWorkflowCmd.Flags().Int32("version", 0, "Workflow version (optional)")
	startWorkflowCmd.MarkFlagsMutuallyExclusive("input", "file")

	executeWorkflowCmd.Flags().StringP("workflow", "w", "", "Workflow name")
	executeWorkflowCmd.Flags().StringP("input", "i", "", "Input json")
	executeWorkflowCmd.Flags().StringP("file", "f", "", "Input file with json data")
	executeWorkflowCmd.Flags().StringP("wait-until", "u", "", "Wait until task completes (instead of entire workflow)")
	executeWorkflowCmd.Flags().Int32("version", 1, "Workflow version (optional)")
	executeWorkflowCmd.Flags().BoolP("sync", "s", true, "Run synchronously")
	executeWorkflowCmd.MarkFlagsMutuallyExclusive("input", "file")

	getWorkflowCmd.Flags().BoolP("complete", "c", false, "Include complete details")
	workflowCmd.AddCommand(
		searchWorkflowCmd,
		getWorkflowCmd,
		startWorkflowCmd,
		executeWorkflowCmd,
		terminateWorkflowCmd,
	)
}
