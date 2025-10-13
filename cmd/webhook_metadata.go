package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/orkes-io/conductor-cli/internal"
	"github.com/spf13/cobra"
	"os"
	"strconv"
	"strings"
)

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "Webhook management",
}

var (
	listWebHookMetadataCmd = &cobra.Command{
		Use:          "list",
		Short:        "List Webhooks",
		RunE:         list,
		SilenceUsage: true,
	}

	deleteWebHookMetadataCmd = &cobra.Command{
		Use:          "delete <webhook_id>",
		Short:        "Delete Webhook",
		RunE:         delete,
		SilenceUsage: true,
		Example:      "webhook delete <webhook_id>",
	}

	getWebHookMetadataCmd = &cobra.Command{
		Use:          "get <webhook_id>",
		Short:        "Get Webhook",
		RunE:         get,
		SilenceUsage: true,
		Example:      "webhook get <webhook_id>",
	}

	createWebHookMetadataCmd = &cobra.Command{
		Use:          "create",
		Short:        "Create Webhook",
		RunE:         create,
		SilenceUsage: true,
		Example:      "webhook create --name my-webhook --file webhook.json",
	}

	updateWebHookMetadataCmd = &cobra.Command{
		Use:          "update <webhook_id>",
		Short:        "Update Webhook",
		RunE:         updateWebhook,
		SilenceUsage: true,
		Example:      "webhook update <webhook_id> --file webhook.json",
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
	var id string

	// Check if reading from stdin (pipe)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Reading from pipe
		data, err := os.ReadFile("/dev/stdin")
		if err != nil {
			return fmt.Errorf("error reading from stdin: %v", err)
		}
		id = strings.TrimSpace(string(data))
	} else if len(args) > 0 {
		id = args[0]
	} else {
		return cmd.Usage()
	}

	// Confirm deletion
	if !confirmDeletion("webhook", id) {
		fmt.Println("Deletion cancelled")
		return nil
	}

	webhookClient := internal.GetWebhooksConfigClient()
	_, err := webhookClient.DeleteWebhook(context.Background(), id)
	if err != nil {
		return err
	}

	fmt.Printf("Webhook %s deleted successfully\n", id)
	return nil
}

func get(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Usage()
	}

	webhookClient := internal.GetWebhooksConfigClient()
	webhook, _, err := webhookClient.GetWebhook(context.Background(), args[0])
	if err != nil {
		return err
	}

	data, _ := json.MarshalIndent(webhook, "", "   ")
	fmt.Println(string(data))
	return nil
}

func create(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	file, _ := cmd.Flags().GetString("file")
	workflowsToStart, _ := cmd.Flags().GetString("workflows-to-start")
	receiverWorkflows, _ := cmd.Flags().GetString("receiver-workflows")
	sourcePlatform, _ := cmd.Flags().GetString("source-platform")
	verifier, _ := cmd.Flags().GetString("verifier")
	headers, _ := cmd.Flags().GetString("headers")

	var webhookConfig model.WebhookConfig
	var data []byte
	var err error

	// Check if reading from stdin (pipe)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Reading from pipe
		data, err = os.ReadFile("/dev/stdin")
		if err != nil {
			return fmt.Errorf("error reading from stdin: %v", err)
		}
		err = json.Unmarshal(data, &webhookConfig)
		if err != nil {
			return fmt.Errorf("error parsing JSON from stdin: %v", err)
		}
	} else if file != "" {
		// Read from file
		data, err = os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("error reading file: %v", err)
		}
		err = json.Unmarshal(data, &webhookConfig)
		if err != nil {
			return fmt.Errorf("error parsing JSON: %v", err)
		}
	} else {
		// Build from flags
		webhookConfig = model.WebhookConfig{
			Name: name,
		}

		if sourcePlatform != "" {
			webhookConfig.SourcePlatform = sourcePlatform
		}

		if verifier != "" {
			webhookConfig.Verifier = verifier
		}

		if headers != "" {
			webhookConfig.Headers = parseHeaderMap(headers)
		}

		if workflowsToStart != "" {
			webhookConfig.WorkflowsToStart = parseWorkflowMap(workflowsToStart)
		}

		if receiverWorkflows != "" {
			webhookConfig.ReceiverWorkflowNamesToVersions = parseWorkflowMap(receiverWorkflows)
		}
	}

	webhookClient := internal.GetWebhooksConfigClient()
	result, _, err := webhookClient.CreateWebhook(context.Background(), webhookConfig)
	if err != nil {
		return fmt.Errorf("error creating webhook: %v", err)
	}

	data, _ = json.MarshalIndent(result, "", "   ")
	fmt.Println(string(data))
	return nil
}

func updateWebhook(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Usage()
	}

	id := args[0]
	file, _ := cmd.Flags().GetString("file")

	var data []byte
	var err error

	// Check if reading from stdin (pipe)
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Reading from pipe
		data, err = os.ReadFile("/dev/stdin")
		if err != nil {
			return fmt.Errorf("error reading from stdin: %v", err)
		}
	} else if file != "" {
		// Read from file
		data, err = os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("error reading file: %v", err)
		}
	} else {
		return fmt.Errorf("--file is required or pipe JSON to stdin")
	}

	var webhookConfig model.WebhookConfig
	err = json.Unmarshal(data, &webhookConfig)
	if err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	webhookClient := internal.GetWebhooksConfigClient()
	result, _, err := webhookClient.UpdateWebhook(context.Background(), webhookConfig, id)
	if err != nil {
		return fmt.Errorf("error updating webhook: %v", err)
	}

	data, _ = json.MarshalIndent(result, "", "   ")
	fmt.Println(string(data))
	return nil
}

// parseWorkflowMap parses a string like "workflow1:1,workflow2:2" into a map
func parseWorkflowMap(input string) map[string]int32 {
	result := make(map[string]int32)
	if input == "" {
		return result
	}

	pairs := strings.Split(input, ",")
	for _, pair := range pairs {
		parts := strings.Split(strings.TrimSpace(pair), ":")
		if len(parts) == 2 {
			version, err := strconv.Atoi(parts[1])
			if err == nil {
				result[parts[0]] = int32(version)
			}
		}
	}
	return result
}

// parseHeaderMap parses a string like "key1:value1,key2:value2" into a map
func parseHeaderMap(input string) map[string]string {
	result := make(map[string]string)
	if input == "" {
		return result
	}

	pairs := strings.Split(input, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

func init() {
	rootCmd.AddCommand(webhookCmd)

	listWebHookMetadataCmd.Flags().Bool("json", false, "print json")

	createWebHookMetadataCmd.Flags().String("name", "", "Webhook name")
	createWebHookMetadataCmd.Flags().String("file", "", "JSON file containing webhook configuration")
	createWebHookMetadataCmd.Flags().String("workflows-to-start", "", "Workflows to start (format: workflow1:version1,workflow2:version2)")
	createWebHookMetadataCmd.Flags().String("receiver-workflows", "", "Receiver workflows (format: workflow1:version1,workflow2:version2)")
	createWebHookMetadataCmd.Flags().String("source-platform", "", "Source platform (e.g., Custom, GitHub, Slack)")
	createWebHookMetadataCmd.Flags().String("verifier", "", "Verifier type (e.g., HEADER_BASED)")
	createWebHookMetadataCmd.Flags().String("headers", "", "Headers as key:value pairs (format: key1:value1,key2:value2)")

	updateWebHookMetadataCmd.Flags().String("file", "", "JSON file containing webhook configuration (required)")
	updateWebHookMetadataCmd.MarkFlagRequired("file")

	webhookCmd.AddCommand(
		listWebHookMetadataCmd,
		getWebHookMetadataCmd,
		createWebHookMetadataCmd,
		updateWebHookMetadataCmd,
		deleteWebHookMetadataCmd,
	)
}
