package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/orkes-io/conductor-cli/internal"
	"github.com/spf13/cobra"
)

var secretCmd = &cobra.Command{
	Use:     "secret",
	Short:   "Secret management",
	GroupID: "conductor",
}

var (
	listSecretsCmd = &cobra.Command{
		Use:          "list",
		Short:        "List all secrets",
		RunE:         listSecrets,
		SilenceUsage: true,
		Example:      "secret list --with-tags",
	}

	getSecretCmd = &cobra.Command{
		Use:          "get <key>",
		Short:        "Get secret value",
		RunE:         getSecret,
		SilenceUsage: true,
		Example:      "secret get db_password --show-value",
	}

	putSecretCmd = &cobra.Command{
		Use:          "put <key> [value]",
		Short:        "Create or update a secret",
		Long:         "Create or update a secret. Value can be provided as argument, via --value flag, or from stdin",
		RunE:         putSecret,
		SilenceUsage: true,
		Example: `  # Put secret with value as argument
  orkes secret put db_password mySecretValue

  # Put secret with value from flag
  orkes secret put db_password --value mySecretValue

  # Put secret from stdin
  echo "mySecretValue" | orkes secret put db_password

  # Put secret from file
  cat secret.txt | orkes secret put db_password`,
	}

	deleteSecretCmd = &cobra.Command{
		Use:          "delete <key>",
		Short:        "Delete a secret",
		RunE:         deleteSecret,
		SilenceUsage: true,
		Example:      "secret delete db_password",
	}

	existsSecretCmd = &cobra.Command{
		Use:          "exists <key>",
		Short:        "Check if a secret exists",
		RunE:         existsSecret,
		SilenceUsage: true,
		Example:      "secret exists db_password",
	}

	tagListCmd = &cobra.Command{
		Use:          "tag-list <key>",
		Short:        "List tags for a secret",
		RunE:         tagList,
		SilenceUsage: true,
		Example:      "secret tag-list db_password",
	}

	tagAddCmd = &cobra.Command{
		Use:          "tag-add <key>",
		Short:        "Add tags to a secret",
		RunE:         tagAdd,
		SilenceUsage: true,
		Example: `  # Add single tag
  orkes secret tag-add db_password --tag env:prod

  # Add multiple tags
  orkes secret tag-add db_password --tag env:prod --tag team:backend`,
	}

	tagDeleteCmd = &cobra.Command{
		Use:          "tag-delete <key>",
		Short:        "Delete tags from a secret",
		RunE:         tagDelete,
		SilenceUsage: true,
		Example: `  # Delete single tag
  orkes secret tag-delete db_password --tag env:prod

  # Delete multiple tags
  orkes secret tag-delete db_password --tag env:prod --tag team:backend`,
	}

	cacheClearCmd = &cobra.Command{
		Use:          "cache-clear",
		Short:        "Clear secrets cache",
		Long:         "Clear secrets cache. Use --local or --redis flags to specify which cache to clear. If neither flag is provided, both caches will be cleared.",
		RunE:         cacheClear,
		SilenceUsage: true,
		Example: `  # Clear local cache
  orkes secret cache-clear --local

  # Clear Redis cache
  orkes secret cache-clear --redis

  # Clear both caches
  orkes secret cache-clear`,
	}
)

func listSecrets(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	secretClient := internal.GetSecretsClient()
	withTags, _ := cmd.Flags().GetBool("with-tags")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	ctx := context.Background()

	if withTags {
		// List secrets with tags
		secrets, _, err := secretClient.ListSecretsWithTagsThatUserCanGrantAccessTo(ctx)
		if err != nil {
			return parseAPIError(err, "Failed to list secrets")
		}

		if jsonOutput {
			data, err := json.MarshalIndent(secrets, "", "  ")
			if err != nil {
				return fmt.Errorf("error marshaling secrets: %v", err)
			}
			fmt.Println(string(data))
			return nil
		}

		// Print as table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "KEY\tTAGS")
		for _, secret := range secrets {
			tagStr := "-"
			if len(secret.Tags) > 0 {
				tagPairs := make([]string, 0, len(secret.Tags))
				for _, tag := range secret.Tags {
					tagPairs = append(tagPairs, fmt.Sprintf("%s:%s", tag.Key, tag.Value))
				}
				tagStr = strings.Join(tagPairs, ", ")
				if len(tagStr) > 50 {
					tagStr = tagStr[:47] + "..."
				}
			}
			fmt.Fprintf(w, "%s\t%s\n", secret.Name, tagStr)
		}
		w.Flush()
	} else {
		// List secret names only
		secretNames, _, err := secretClient.ListAllSecretNames(ctx)
		if err != nil {
			return parseAPIError(err, "Failed to list secrets")
		}

		if jsonOutput {
			data, err := json.MarshalIndent(secretNames, "", "  ")
			if err != nil {
				return fmt.Errorf("error marshaling secrets: %v", err)
			}
			fmt.Println(string(data))
			return nil
		}

		// Print as simple list
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "KEY")
		for _, name := range secretNames {
			fmt.Fprintln(w, name)
		}
		w.Flush()
	}

	return nil
}

func getSecret(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	if len(args) == 0 {
		return cmd.Usage()
	}

	key := args[0]
	showValue, _ := cmd.Flags().GetBool("show-value")
	secretClient := internal.GetSecretsClient()

	ctx := context.Background()
	value, _, err := secretClient.GetSecret(ctx, key)
	if err != nil {
		return parseAPIError(err, fmt.Sprintf("Failed to get secret '%s'", key))
	}

	if showValue {
		fmt.Println(value)
	} else {
		fmt.Println("Secret exists. Use --show-value to display the actual value.")
	}

	return nil
}

func putSecret(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	if len(args) == 0 {
		return cmd.Usage()
	}

	key := args[0]
	var value string

	// Get value from flag first
	flagValue, _ := cmd.Flags().GetString("value")
	if flagValue != "" {
		value = flagValue
	} else if len(args) > 1 {
		// Get value from second argument
		value = args[1]
	} else {
		// Check if reading from stdin (pipe)
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Reading from pipe
			data, err := os.ReadFile("/dev/stdin")
			if err != nil {
				return fmt.Errorf("error reading from stdin: %v", err)
			}
			value = strings.TrimSpace(string(data))
		} else {
			return fmt.Errorf("secret value required: provide as argument, --value flag, or via stdin")
		}
	}

	if value == "" {
		return fmt.Errorf("secret value cannot be empty")
	}

	secretClient := internal.GetSecretsClient()
	ctx := context.Background()
	_, _, err := secretClient.PutSecret(ctx, value, key)
	if err != nil {
		return parseAPIError(err, fmt.Sprintf("Failed to put secret '%s'", key))
	}

	fmt.Printf("Secret '%s' saved successfully\n", key)
	return nil
}

func deleteSecret(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	if len(args) == 0 {
		return cmd.Usage()
	}

	key := args[0]

	// Confirm deletion
	if !confirmDeletion("secret", key) {
		fmt.Println("Deletion cancelled")
		return nil
	}

	secretClient := internal.GetSecretsClient()
	ctx := context.Background()
	_, _, err := secretClient.DeleteSecret(ctx, key)
	if err != nil {
		return parseAPIError(err, fmt.Sprintf("Failed to delete secret '%s'", key))
	}

	fmt.Printf("Secret '%s' deleted successfully\n", key)
	return nil
}

func existsSecret(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	if len(args) == 0 {
		return cmd.Usage()
	}

	key := args[0]
	secretClient := internal.GetSecretsClient()

	ctx := context.Background()
	_, resp, err := secretClient.SecretExists(ctx, key)
	if err != nil {
		return parseAPIError(err, fmt.Sprintf("Failed to check if secret '%s' exists", key))
	}

	if resp.StatusCode == 200 {
		fmt.Printf("Secret '%s' exists\n", key)
	} else {
		fmt.Printf("Secret '%s' does not exist\n", key)
	}

	return nil
}

func tagList(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	if len(args) == 0 {
		return cmd.Usage()
	}

	key := args[0]
	jsonOutput, _ := cmd.Flags().GetBool("json")
	secretClient := internal.GetSecretsClient()

	ctx := context.Background()
	tags, _, err := secretClient.GetTags(ctx, key)
	if err != nil {
		return parseAPIError(err, fmt.Sprintf("Failed to get tags for secret '%s'", key))
	}

	if jsonOutput {
		data, err := json.MarshalIndent(tags, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling tags: %v", err)
		}
		fmt.Println(string(data))
		return nil
	}

	if len(tags) == 0 {
		fmt.Println("No tags found")
		return nil
	}

	// Print as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "KEY\tVALUE\tTYPE")
	for _, tag := range tags {
		tagType := tag.Type_
		if tagType == "" {
			tagType = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", tag.Key, tag.Value, tagType)
	}
	w.Flush()

	return nil
}

func tagAdd(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	if len(args) == 0 {
		return cmd.Usage()
	}

	key := args[0]
	tagStrings, _ := cmd.Flags().GetStringArray("tag")

	if len(tagStrings) == 0 {
		return fmt.Errorf("at least one --tag flag is required")
	}

	// Parse tags
	tags := make([]model.Tag, 0, len(tagStrings))
	for _, tagStr := range tagStrings {
		parts := strings.SplitN(tagStr, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid tag format '%s': expected key:value", tagStr)
		}
		tags = append(tags, model.Tag{
			Key:   strings.TrimSpace(parts[0]),
			Value: strings.TrimSpace(parts[1]),
			Type_: "metadata",
		})
	}

	secretClient := internal.GetSecretsClient()
	ctx := context.Background()
	_, err := secretClient.PutTagForSecret(ctx, tags, key)
	if err != nil {
		return parseAPIError(err, fmt.Sprintf("Failed to add tags to secret '%s'", key))
	}

	fmt.Printf("Tags added to secret '%s' successfully\n", key)
	return nil
}

func tagDelete(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	if len(args) == 0 {
		return cmd.Usage()
	}

	key := args[0]
	tagStrings, _ := cmd.Flags().GetStringArray("tag")

	if len(tagStrings) == 0 {
		return fmt.Errorf("at least one --tag flag is required")
	}

	// Parse tags
	tags := make([]model.Tag, 0, len(tagStrings))
	for _, tagStr := range tagStrings {
		parts := strings.SplitN(tagStr, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid tag format '%s': expected key:value", tagStr)
		}
		tags = append(tags, model.Tag{
			Key:   strings.TrimSpace(parts[0]),
			Value: strings.TrimSpace(parts[1]),
			Type_: "metadata",
		})
	}

	secretClient := internal.GetSecretsClient()
	ctx := context.Background()
	_, err := secretClient.DeleteTagForSecret(ctx, tags, key)
	if err != nil {
		return parseAPIError(err, fmt.Sprintf("Failed to delete tags from secret '%s'", key))
	}

	fmt.Printf("Tags deleted from secret '%s' successfully\n", key)
	return nil
}

func cacheClear(cmd *cobra.Command, args []string) error {
	if !isEnterpriseServer() {
		return fmt.Errorf("Not supported in OSS Conductor")
	}

	local, _ := cmd.Flags().GetBool("local")
	redis, _ := cmd.Flags().GetBool("redis")

	// If neither flag is set, clear both
	if !local && !redis {
		local = true
		redis = true
	}

	secretClient := internal.GetSecretsClient()
	ctx := context.Background()

	if local {
		result, _, err := secretClient.ClearLocalCache(ctx)
		if err != nil {
			return parseAPIError(err, "Failed to clear local cache")
		}

		fmt.Println("Local cache cleared successfully")
		if len(result) > 0 {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		}
	}

	if redis {
		result, _, err := secretClient.ClearRedisCache(ctx)
		if err != nil {
			return parseAPIError(err, "Failed to clear Redis cache")
		}

		fmt.Println("Redis cache cleared successfully")
		if len(result) > 0 {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		}
	}

	return nil
}

func init() {
	rootCmd.AddCommand(secretCmd)

	// List command flags
	listSecretsCmd.Flags().Bool("with-tags", false, "Include tags in the output")
	listSecretsCmd.Flags().Bool("json", false, "Output as JSON")

	// Get command flags
	getSecretCmd.Flags().Bool("show-value", false, "Display the actual secret value")

	// Put command flags
	putSecretCmd.Flags().String("value", "", "Secret value")

	// Tag list command flags
	tagListCmd.Flags().Bool("json", false, "Output as JSON")

	// Tag add command flags
	tagAddCmd.Flags().StringArray("tag", []string{}, "Tag in key:value format (repeatable)")
	tagAddCmd.MarkFlagRequired("tag")

	// Tag delete command flags
	tagDeleteCmd.Flags().StringArray("tag", []string{}, "Tag in key:value format (repeatable)")
	tagDeleteCmd.MarkFlagRequired("tag")

	// Cache clear command flags
	cacheClearCmd.Flags().Bool("local", false, "Clear local cache only")
	cacheClearCmd.Flags().Bool("redis", false, "Clear Redis cache only")

	secretCmd.AddCommand(
		listSecretsCmd,
		getSecretCmd,
		putSecretCmd,
		deleteSecretCmd,
		existsSecretCmd,
		tagListCmd,
		tagAddCmd,
		tagDeleteCmd,
		cacheClearCmd,
	)
}
