/*
 * Copyright 2026 Conductor Authors.
 * <p>
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 * <p>
 * http://www.apache.org/licenses/LICENSE-2.0
 * <p>
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */


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
	"text/tabwriter"
)

var webhookCmd = &cobra.Command{
	Use:     "webhook",
	Short:   "Webhook management",
	GroupID: "conductor",
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
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	webhookClient := internal.GetWebhooksConfigClient()
	webhooks, _, err := webhookClient.GetAllWebhook(context.Background())
	jsonOutput, _ := cmd.Flags().GetBool("json")
	if err != nil {
		return parseAPIError(err, "Failed to list webhooks")
	}

	if jsonOutput {
		data, err := json.MarshalIndent(webhooks, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling webhooks: %v", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Print as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tWEBHOOK ID\tWORKFLOWS\tURL")
	for _, webhook := range webhooks {
		// Get workflow names from both fields
		workflows := []string{}
		if webhook.WorkflowsToStart != nil {
			for wf := range webhook.WorkflowsToStart {
				workflows = append(workflows, wf)
			}
		}
		if webhook.ReceiverWorkflowNamesToVersions != nil {
			for wf := range webhook.ReceiverWorkflowNamesToVersions {
				workflows = append(workflows, wf)
			}
		}

		workflowStr := "-"
		if len(workflows) > 0 {
			workflowStr = strings.Join(workflows, ", ")
			if len(workflowStr) > 30 {
				workflowStr = workflowStr[:27] + "..."
			}
		}

		// Construct webhook URL (standard Conductor webhook URL format)
		webhookURL := fmt.Sprintf("/api/webhook/%s", webhook.Id)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			webhook.Name,
			webhook.Id,
			workflowStr,
			webhookURL,
		)
	}
	w.Flush()

	return nil
}

func delete(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	var id string

	// Check args first
	if len(args) > 0 {
		id = args[0]
	} else {
		// Check if reading from stdin (pipe)
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Reading from pipe
			data, err := os.ReadFile("/dev/stdin")
			if err != nil {
				return fmt.Errorf("error reading from stdin: %v", err)
			}
			id = strings.TrimSpace(string(data))
		} else {
			return cmd.Usage()
		}
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
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	if len(args) == 0 {
		return cmd.Usage()
	}

	webhookClient := internal.GetWebhooksConfigClient()
	webhook, _, err := webhookClient.GetWebhook(context.Background(), args[0])
	if err != nil {
		return parseAPIError(err, fmt.Sprintf("Failed to get webhook '%s'", args[0]))
	}

	data, _ := json.MarshalIndent(webhook, "", "   ")
	fmt.Println(string(data))
	return nil
}

func create(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

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

	// Check if using file or flags first
	if file != "" {
		// Read from file
		data, err = os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("error reading file: %v", err)
		}
		err = json.Unmarshal(data, &webhookConfig)
		if err != nil {
			return fmt.Errorf("error parsing JSON: %v", err)
		}
	} else if name != "" || sourcePlatform != "" || verifier != "" {
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
	} else {
		// Reading from stdin (pipe)
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err = os.ReadFile("/dev/stdin")
			if err != nil {
				return fmt.Errorf("error reading from stdin: %v", err)
			}
			err = json.Unmarshal(data, &webhookConfig)
			if err != nil {
				return fmt.Errorf("error parsing JSON from stdin: %v", err)
			}
		} else {
			return cmd.Usage()
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
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	if len(args) == 0 {
		return cmd.Usage()
	}

	id := args[0]
	file, _ := cmd.Flags().GetString("file")

	var data []byte
	var err error

	// Check file flag first
	if file != "" {
		// Read from file
		data, err = os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("error reading file: %v", err)
		}
	} else {
		// Check if reading from stdin (pipe)
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Reading from pipe
			data, err = os.ReadFile("/dev/stdin")
			if err != nil {
				return fmt.Errorf("error reading from stdin: %v", err)
			}
		} else {
			return fmt.Errorf("--file is required or pipe JSON to stdin")
		}
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
