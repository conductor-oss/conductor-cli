package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/orkes-io/conductor-cli/internal"
	"github.com/spf13/cobra"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Webhooks Management",
}

var (
	listWebHookMetadataCmd = &cobra.Command{
		Use:          "list",
		Short:        "List Webhooks",
		RunE:         list,
		SilenceUsage: true,
	}

	deleteWebHookMetadataCmd = &cobra.Command{
		Use:          "delete",
		Short:        "Delete Webhook",
		RunE:         delete,
		SilenceUsage: true,
	}
)

func list(cmd *cobra.Command, args []string) error {

	webhookClient := internal.GetWebhooksConfigClient()
	tasks, _, err := webhookClient.GetAllWebhook(context.Background())
	verbose, _ := cmd.Flags().GetBool("json")
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if verbose {
			bytes, _ := json.MarshalIndent(task, "", "   ")
			fmt.Println(string(bytes))
		} else {
			fmt.Printf("%s,%s\n", task.Name, task.Id)
		}

	}
	return nil
}

func delete(cmd *cobra.Command, args []string) error {
	webhookClient := internal.GetWebhooksConfigClient()

	if len(args) == 0 {
		return cmd.Usage()
	}
	for i := 0; i < len(args); i++ {
		id := args[i]
		fmt.Println(id)
		_, err := webhookClient.DeleteWebhook(context.Background(), id)
		if err != nil {
			return err
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(webhookCmd)
	listWebHookMetadataCmd.Flags().Bool("json", false, "print json")
	webhookCmd.AddCommand(
		listWebHookMetadataCmd,
		deleteWebHookMetadataCmd,
	)
}
