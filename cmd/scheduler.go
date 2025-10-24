package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/antihax/optional"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/orkes-io/conductor-cli/internal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

var schedulerCmd = &cobra.Command{
	Use:     "schedule",
	Short:   "Schedule management",
	GroupID: "conductor",
}

var (
	//Schedule Management
	listSchedulerCmd = &cobra.Command{
		Use:          "list",
		Short:        "List Schedules",
		RunE:         listSchedules,
		SilenceUsage: true,
	}
	getSchedulerCmd = &cobra.Command{
		Use:          "get",
		Short:        "Get Schedule Details",
		RunE:         getSchedule,
		SilenceUsage: true,
		Example:      "get [schedule_name] [schedule_name] ...",
	}

	deleteSchedulerCmd = &cobra.Command{
		Use:          "delete",
		Short:        "Delete schedule",
		RunE:         deleteSchedule,
		SilenceUsage: true,
		Example:      "delete [schedule_name] [schedule_name] ...",
	}
	pauseSchedulerCmd = &cobra.Command{
		Use:          "pause",
		Short:        "Pause Schedule",
		RunE:         pauseSchedule,
		SilenceUsage: true,
		Example:      "pause [schedule_name] [schedule_name] ...",
	}
	resumeSchedulerCmd = &cobra.Command{
		Use:          "resume",
		Short:        "Resume Schedule",
		RunE:         resumeSchedule,
		SilenceUsage: true,
		Example:      "resume [schedule_name] [schedule_name] ...",
	}

	searchSchedulerCmd = &cobra.Command{
		Use:          "search",
		Short:        "Search for executions of the schedules",
		RunE:         searchScheduledExecutions,
		Example:      "search -w workflow_name -s status -c count",
		SilenceUsage: true,
	}

	createSchedulerCmd = &cobra.Command{
		Use:          "create",
		Short:        "Create a new schedule",
		Long:         "Create a schedule from a JSON file, stdin, or using flags",
		RunE:         createSchedule,
		SilenceUsage: true,
		Example: `  # Create from JSON file
  orkes schedule create schedule.json

  # Create using flags
  orkes schedule create -n my_schedule -c "0 0 * ? * *" -w hello_world

  # With input
  orkes schedule create -n daily_task -c "0 0 * ? * *" -w my_workflow -i '{"key":"value"}'

  # Create paused schedule
  orkes schedule create -n my_schedule -c "0 0 * ? * *" -w hello_world -p`,
	}

	updateSchedulerCmd = &cobra.Command{
		Use:          "update",
		Short:        "Update existing schedule",
		Long:         "Update a schedule from a JSON file, stdin, or using flags",
		RunE:         updateSchedule,
		SilenceUsage: true,
		Example: `  # Update from JSON file
  orkes schedule update schedule.json

  # Update using flags
  orkes schedule update -n my_schedule -c "0 0 12 ? * *" -w hello_world

  # Update with new input
  orkes schedule update -n my_schedule -c "0 0 * ? * *" -w my_workflow -i '{"updated":"data"}'`,
	}
)

func listSchedules(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	schedulerClient := internal.GetSchedulerClient()
	var workflowName optional.String
	if len(args) == 1 {
		workflowName = optional.NewString(args[0])
	}
	options := client.SchedulerResourceApiGetAllSchedulesOpts{WorkflowName: workflowName}
	schedules, _, err := schedulerClient.GetAllSchedules(context.Background(), &options)
	jsonOutput, _ := cmd.Flags().GetBool("json")
	if err != nil {
		return parseAPIError(err, "Failed to list schedules")
	}

	if jsonOutput {
		data, err := json.MarshalIndent(schedules, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling schedules: %v", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Print as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tCRON\tWORKFLOW\tSTATUS\tCREATED TIME")
	for _, schedule := range schedules {
		status := "active"
		if schedule.Paused {
			status = "paused"
		}
		workflowName := schedule.StartWorkflowRequest.Name

		// Format create time
		createdTime := "-"
		if schedule.CreateTime > 0 {
			t := time.UnixMilli(schedule.CreateTime)
			createdTime = t.Format("2006-01-02 15:04:05")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			schedule.Name,
			schedule.CronExpression,
			workflowName,
			status,
			createdTime,
		)
	}
	w.Flush()

	return nil
}

func getSchedule(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	schedulerClient := internal.GetSchedulerClient()
	if len(args) == 0 {
		return cmd.Usage()
	}

	for i := 0; i < len(args); i++ {
		schedule, _, err := schedulerClient.GetSchedule(context.Background(), args[i])
		if err != nil {
			return parseAPIError(err, fmt.Sprintf("Failed to get schedule '%s'", args[i]))
		}
		bytes, _ := json.MarshalIndent(schedule, "", "   ")
		fmt.Println(string(bytes))
	}
	return nil
}

func deleteSchedule(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	schedulerClient := internal.GetSchedulerClient()
	if len(args) == 0 {
		return cmd.Usage()
	}

	for i := 0; i < len(args); i++ {
		name := args[i]

		// Confirm deletion
		if !confirmDeletion("schedule", name) {
			fmt.Printf("Skipping deletion of schedule '%s'\n", name)
			continue
		}

		_, _, err := schedulerClient.DeleteSchedule(context.Background(), name)
		if err != nil {
			return err
		}
		fmt.Printf("Schedule '%s' deleted successfully\n", name)
	}
	return nil
}

func pauseSchedule(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	schedulerClient := internal.GetSchedulerClient()
	if len(args) == 0 {
		return cmd.Usage()
	}

	for i := 0; i < len(args); i++ {
		_, _, err := schedulerClient.PauseSchedule(context.Background(), args[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func resumeSchedule(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	schedulerClient := internal.GetSchedulerClient()
	if len(args) == 0 {
		return cmd.Usage()
	}

	for i := 0; i < len(args); i++ {
		_, _, err := schedulerClient.ResumeSchedule(context.Background(), args[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func searchScheduledExecutions(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	schedulerClient := internal.GetSchedulerClient()
	if len(args) == 0 {
		return cmd.Usage()
	}
	count, _ := cmd.Flags().GetInt32("count")
	if count > 1000 {
		log.Info("count exceeds max allowed 1000.  Will only show the first 1000 matching results")
		count = 1000
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
	searchOpts := client.SchedulerSearchOpts{
		Start:    optional.NewInt32(0),
		Size:     optional.NewInt32(count),
		FreeText: optional.NewString("*"),
		Query:    optional.NewString(query),
		Sort:     optional.NewString("startTime:DESC"),
	}
	for i := 0; i < len(args); i++ {
		results, _, err := schedulerClient.SearchV2(context.Background(), &searchOpts)
		if err != nil {
			return err
		}
		items := results.Results
		for _, item := range items {
			execTime := time.UnixMilli(item.ExecutionTime).Format(time.UnixDate)
			fmt.Println(strings.Join([]string{item.State, item.WorkflowName, execTime, item.WorkflowId, item.Reason}, ","))
		}
	}
	return nil
}

func createSchedule(cmd *cobra.Command, args []string) error {
	return createOrUpdateSchedule(false, cmd, args)
}

func updateSchedule(cmd *cobra.Command, args []string) error {
	return createOrUpdateSchedule(true, cmd, args)
}
func createOrUpdateSchedule(update bool, cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	schedulerClient := internal.GetSchedulerClient()
	var request model.SaveScheduleRequest
	var err error

	// Check if using flags or file/stdin
	name, _ := cmd.Flags().GetString("name")
	cron, _ := cmd.Flags().GetString("cron")
	workflow, _ := cmd.Flags().GetString("workflow")

	// If flags are provided, use flag-based creation
	if name != "" || cron != "" || workflow != "" {
		// Validate required flags
		if name == "" {
			return errors.New("--name is required")
		}
		if cron == "" {
			return errors.New("--cron is required")
		}
		if workflow == "" {
			return errors.New("--workflow is required")
		}

		version, _ := cmd.Flags().GetInt32("version")
		inputJson, _ := cmd.Flags().GetString("input")
		paused, _ := cmd.Flags().GetBool("paused")

		var input map[string]interface{}
		if inputJson != "" {
			err = json.Unmarshal([]byte(inputJson), &input)
			if err != nil {
				return fmt.Errorf("input string must be valid JSON: %w", err)
			}
		}

		workflowRequest := model.StartWorkflowRequest{
			Name:    workflow,
			Version: version,
			Input:   input,
		}
		request = model.SaveScheduleRequest{
			CronExpression:       cron,
			Name:                 name,
			StartWorkflowRequest: &workflowRequest,
			Paused:               paused,
		}
	} else {
		// File or stdin based creation
		var bytes []byte

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
			if len(bytes) == 0 {
				return fmt.Errorf("no schedule data received from stdin")
			}
		}
		err = json.Unmarshal(bytes, &request)
		if err != nil {
			return fmt.Errorf("failed to parse schedule JSON: %w", err)
		}
	}
	var exists bool
	//Let's check if there is an existing schedule
	_, _, err = schedulerClient.GetSchedule(context.Background(), request.Name)
	if err != nil {
		var swaggerErr client.GenericSwaggerError
		if errors.As(err, &swaggerErr) && swaggerErr.StatusCode() == 404 {
			exists = false
		}
	} else {
		exists = true
	}

	if update && !exists {
		return errors.New("no such schedule by name " + request.Name)
	}
	if !update && exists {
		return errors.New("a schedule already exists by this name " + request.Name + ". " +
			"(hint: use update command to update the existing schedule) ")
	}
	if update {

	}
	_, _, err = schedulerClient.SaveSchedule(context.Background(), request)
	if err != nil {
		return err
	}
	return nil
}

func init() {
	rootCmd.AddCommand(schedulerCmd)

	listSchedulerCmd.Flags().BoolP("json", "j", false, "Print full json")
	listSchedulerCmd.Flags().BoolP("cron", "c", false, "Print cron expression")
	listSchedulerCmd.Flags().BoolP("pretty", "p", false, "Print formatted json")

	createSchedulerCmd.Flags().StringP("name", "n", "", "Name of the schedule (required)")
	createSchedulerCmd.Flags().StringP("cron", "c", "", "Cron expression (required)")
	createSchedulerCmd.Flags().StringP("workflow", "w", "", "Workflow to start (required)")
	createSchedulerCmd.Flags().StringP("input", "i", "", "Workflow input as JSON string")
	createSchedulerCmd.Flags().BoolP("paused", "p", false, "Create schedule in paused state")
	createSchedulerCmd.Flags().Int32("version", 0, "Workflow version (0 for latest)")

	updateSchedulerCmd.Flags().StringP("name", "n", "", "Name of the schedule (required)")
	updateSchedulerCmd.Flags().StringP("cron", "c", "", "Cron expression (required)")
	updateSchedulerCmd.Flags().StringP("workflow", "w", "", "Workflow to start (required)")
	updateSchedulerCmd.Flags().StringP("input", "i", "", "Workflow input as JSON string")
	updateSchedulerCmd.Flags().BoolP("paused", "p", false, "Pause schedule")
	updateSchedulerCmd.Flags().Int32("version", 0, "Workflow version (0 for latest)")

	searchSchedulerCmd.Flags().Int32P("count", "c", 10, "No of workflows to return (max 1000)")
	searchSchedulerCmd.Flags().StringP("status", "s", "", "Filter by status one of (COMPLETED, FAILED, PAUSED, RUNNING, TERMINATED, TIMED_OUT)")

	schedulerCmd.AddCommand(
		listSchedulerCmd,
		getSchedulerCmd,
		createSchedulerCmd,
		updateSchedulerCmd,
		deleteSchedulerCmd,
		pauseSchedulerCmd,
		resumeSchedulerCmd,
		searchSchedulerCmd,
	)
}
