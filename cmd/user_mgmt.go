package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/orkes-io/conductor-cli/internal"
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
	Short: "User Management",
}

var (

	//List all the users in the system
	listUsersCmd = &cobra.Command{
		Use:          "list",
		Short:        "List Users",
		RunE:         listUsers,
		SilenceUsage: true,
	}
)

func listUsers(cmd *cobra.Command, args []string) error {

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

func init() {
	rootCmd.AddCommand(userCmd)
	taskCmd.AddCommand(
		listUsersCmd,
	)
}
