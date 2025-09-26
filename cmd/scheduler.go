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
	"time"
)

var schedulerCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Schedule Management",
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
		RunE:         createSchedule,
		SilenceUsage: true,
		Example: `
create [json_file]
create [json_stream]
create -n [name] -c [cron_expression] -w [workflow_to_start] -i [input_json_for_workflow]`,
	}

	updateSchedulerCmd = &cobra.Command{
		Use:          "update",
		Short:        "Update existing schedule",
		RunE:         updateSchedule,
		SilenceUsage: true,
		Example: `
update [json_file]
update [json_stream]
update -n [name] -c [cron_expression] -w [workflow_to_start] -i [input_json_for_workflow]`,
	}
)

func listSchedules(cmd *cobra.Command, args []string) error {

	schedulerClient := internal.GetSchedulerClient()
	var workflowName optional.String
	if len(args) == 1 {
		workflowName = optional.NewString(args[0])
	}
	options := client.SchedulerResourceApiGetAllSchedulesOpts{WorkflowName: workflowName}
	schedules, _, err := schedulerClient.GetAllSchedules(context.Background(), &options)
	verbose, _ := cmd.Flags().GetBool("json")
	cron, _ := cmd.Flags().GetBool("cron")
	pretty, _ := cmd.Flags().GetBool("pretty")
	if err != nil {
		return err
	}
	for _, schedule := range schedules {
		var bytes []byte
		if verbose {
			if pretty {
				bytes, _ = json.MarshalIndent(schedule, "", "   ")
			} else {
				bytes, _ = json.Marshal(schedule)
			}

			fmt.Println(string(bytes))

		} else {
			if cron {
				fmt.Println(schedule.Name, schedule.CronExpression)
			} else {
				fmt.Println(schedule.Name)
			}

		}

	}
	return nil
}

func getSchedule(cmd *cobra.Command, args []string) error {

	schedulerClient := internal.GetSchedulerClient()
	if len(args) == 0 {
		return cmd.Usage()
	}

	for i := 0; i < len(args); i++ {
		schedule, _, err := schedulerClient.GetSchedule(context.Background(), args[i])
		if err != nil {
			return err
		}
		bytes, _ := json.MarshalIndent(schedule, "", "   ")
		fmt.Println(string(bytes))
	}
	return nil
}

func deleteSchedule(cmd *cobra.Command, args []string) error {

	schedulerClient := internal.GetSchedulerClient()
	if len(args) == 0 {
		return cmd.Usage()
	}

	for i := 0; i < len(args); i++ {
		_, _, err := schedulerClient.DeleteSchedule(context.Background(), args[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func pauseSchedule(cmd *cobra.Command, args []string) error {

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

	schedulerClient := internal.GetSchedulerClient()
	var request model.SaveScheduleRequest
	var err error
	cron, _ := cmd.Flags().GetString("cron")
	if cron != "" {
		name, _ := cmd.Flags().GetString("name")
		workflow, _ := cmd.Flags().GetString("workflow")
		version, _ := cmd.Flags().GetInt32("version")
		inputJson, _ := cmd.Flags().GetString("input")
		paused, _ := cmd.Flags().GetBool("paused")
		if workflow == "" {
			return errors.New("missing workflow name")
		}

		var input map[string]interface{}
		err = json.Unmarshal([]byte(inputJson), &input)
		if err != nil {
			log.Error("input string MUST be valid JSON")
			return err
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
		var bytes []byte

		if len(args) == 1 {
			file := args[0]
			bytes, err = os.ReadFile(file)
			if err != nil {
				return err
			}
		} else {
			bytes = read()
		}
		err = json.Unmarshal(bytes, &request)
		if err != nil {
			return err
		}
	}
	var exists bool
	//Let's check if there is an existing schedule
	_, res, err := schedulerClient.GetSchedule(context.Background(), request.Name)
	if err != nil {
		if res.StatusCode == 404 {
			exists = false
		} else {
			return err
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

	createSchedulerCmd.Flags().StringP("workflow", "w", "", "Workflow to start")
	createSchedulerCmd.Flags().StringP("cron", "c", "", "Cron Schedule")
	createSchedulerCmd.Flags().StringP("name", "n", "", "Name of the schedule")
	createSchedulerCmd.Flags().StringP("input", "i", "{}", "Workflow Input")
	createSchedulerCmd.Flags().BoolP("paused", "p", false, "Pause schedule when created")
	createSchedulerCmd.Flags().Int32P("version", "v", 0, "Workflow Version")

	updateSchedulerCmd.Flags().StringP("workflow", "w", "", "Workflow to start")
	updateSchedulerCmd.Flags().StringP("cron", "c", "", "Cron Schedule")
	updateSchedulerCmd.Flags().StringP("name", "n", "", "Name of the schedule")
	updateSchedulerCmd.Flags().StringP("input", "i", "{}", "Workflow Input")
	updateSchedulerCmd.Flags().BoolP("paused", "p", false, "Pause schedule when created")
	updateSchedulerCmd.Flags().Int32P("version", "v", 0, "Workflow Version")

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
