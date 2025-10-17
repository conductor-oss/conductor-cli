package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/model/gateway"
	"github.com/orkes-io/conductor-cli/internal"
	"github.com/spf13/cobra"
)

var (
	apiGatewayCmd = &cobra.Command{
		Use:   "api-gateway",
		Short: "API Gateway management commands",
		Long:  "Manage API Gateway services, routes, and authentication configurations.",
	}

	// Service commands
	serviceCmd = &cobra.Command{
		Use:   "service",
		Short: "Manage API Gateway services",
		Long:  "Create, read, update, and delete API Gateway services.",
	}

	serviceListCmd = &cobra.Command{
		Use:          "list",
		Short:        "List all API Gateway services",
		RunE:         listServices,
		SilenceUsage: true,
	}

	serviceGetCmd = &cobra.Command{
		Use:          "get <service_id>",
		Short:        "Get an API Gateway service by ID",
		RunE:         getService,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
	}

	serviceCreateCmd = &cobra.Command{
		Use:          "create <file>",
		Short:        "Create an API Gateway service from a JSON file",
		RunE:         createService,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
	}

	serviceUpdateCmd = &cobra.Command{
		Use:          "update <service_id> <file>",
		Short:        "Update an API Gateway service",
		RunE:         updateService,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
	}

	serviceDeleteCmd = &cobra.Command{
		Use:          "delete <service_id>",
		Short:        "Delete an API Gateway service",
		RunE:         deleteService,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
	}

	// Auth config commands
	authConfigCmd = &cobra.Command{
		Use:   "auth",
		Short: "Manage API Gateway authentication configurations",
		Long:  "Create, read, update, and delete API Gateway authentication configurations.",
	}

	authConfigListCmd = &cobra.Command{
		Use:          "list",
		Short:        "List all authentication configurations",
		RunE:         listAuthConfigs,
		SilenceUsage: true,
	}

	authConfigGetCmd = &cobra.Command{
		Use:          "get <auth_config_id>",
		Short:        "Get an authentication configuration by ID",
		RunE:         getAuthConfig,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
	}

	authConfigCreateCmd = &cobra.Command{
		Use:          "create <file>",
		Short:        "Create an authentication configuration from a JSON file",
		RunE:         createAuthConfig,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
	}

	authConfigUpdateCmd = &cobra.Command{
		Use:          "update <auth_config_id> <file>",
		Short:        "Update an authentication configuration",
		RunE:         updateAuthConfig,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
	}

	authConfigDeleteCmd = &cobra.Command{
		Use:          "delete <auth_config_id>",
		Short:        "Delete an authentication configuration",
		RunE:         deleteAuthConfig,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
	}

	// Route commands
	routeCmd = &cobra.Command{
		Use:   "route",
		Short: "Manage API Gateway routes",
		Long:  "Create, read, update, and delete API Gateway routes within services.",
	}

	routeListCmd = &cobra.Command{
		Use:          "list <service_id>",
		Short:        "List all routes for a service",
		RunE:         listRoutes,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
	}

	routeCreateCmd = &cobra.Command{
		Use:          "create <service_id> <file>",
		Short:        "Create a route for a service from a JSON file",
		RunE:         createRoute,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(2),
	}

	routeUpdateCmd = &cobra.Command{
		Use:          "update <service_id> <route_path> <file>",
		Short:        "Update a route for a service",
		RunE:         updateRoute,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(3),
	}

	routeDeleteCmd = &cobra.Command{
		Use:          "delete <service_id> <http_method> <route_path>",
		Short:        "Delete a route from a service",
		RunE:         deleteRoute,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(3),
	}
)

// ==================== Service Functions ====================

func getGatewayClient() client.ApiGatewayClient {
	return internal.GetGatewayClient()
}

func listServices(cmd *cobra.Command, args []string) error {
	complete, _ := cmd.Flags().GetBool("complete")

	gatewayClient := getGatewayClient()
	services, _, err := gatewayClient.GetAllServices(context.Background())
	if err != nil {
		return fmt.Errorf("error listing services: %v", err)
	}

	if complete {
		data, err := json.MarshalIndent(services, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling services: %v", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Print as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tPATH\tENABLED\tAUTH CONFIG\tROUTES")
	for _, service := range services {
		enabled := "false"
		if service.Enabled {
			enabled = "true"
		}
		routeCount := len(service.Routes)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n",
			service.Id,
			service.Name,
			service.Path,
			enabled,
			service.AuthConfigId,
			routeCount,
		)
	}
	w.Flush()

	return nil
}

func getService(cmd *cobra.Command, args []string) error {
	serviceId := args[0]
	gatewayClient := getGatewayClient()
	service, _, err := gatewayClient.GetService(context.Background(), serviceId)
	if err != nil {
		return fmt.Errorf("error getting service: %v", err)
	}

	data, err := json.MarshalIndent(service, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling service: %v", err)
	}

	fmt.Println(string(data))
	return nil
}

func createService(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	var service gateway.ApiGatewayService
	if err := json.Unmarshal(fileData, &service); err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	gatewayClient := getGatewayClient()
	_, err = gatewayClient.CreateService(context.Background(), service)
	if err != nil {
		return fmt.Errorf("error creating service: %v", err)
	}

	fmt.Printf("Service created successfully: %s\n", service.Name)
	return nil
}

func updateService(cmd *cobra.Command, args []string) error {
	serviceId := args[0]
	filePath := args[1]

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	var service gateway.ApiGatewayService
	if err := json.Unmarshal(fileData, &service); err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	gatewayClient := getGatewayClient()
	_, err = gatewayClient.UpdateService(context.Background(), serviceId, service)
	if err != nil {
		return fmt.Errorf("error updating service: %v", err)
	}

	fmt.Printf("Service updated successfully: %s\n", serviceId)
	return nil
}

func deleteService(cmd *cobra.Command, args []string) error {
	serviceId := args[0]

	if !confirmDeletion("service", serviceId) {
		fmt.Println("Deletion cancelled")
		return nil
	}

	gatewayClient := getGatewayClient()
	_, err := gatewayClient.DeleteService(context.Background(), serviceId)
	if err != nil {
		return fmt.Errorf("error deleting service: %v", err)
	}

	fmt.Printf("Service deleted successfully: %s\n", serviceId)
	return nil
}

// ==================== Auth Config Functions ====================

func listAuthConfigs(cmd *cobra.Command, args []string) error {
	complete, _ := cmd.Flags().GetBool("complete")

	gatewayClient := getGatewayClient()
	authConfigs, _, err := gatewayClient.GetAllAuthConfigs(context.Background())
	if err != nil {
		return fmt.Errorf("error listing auth configs: %v", err)
	}

	if complete {
		data, err := json.MarshalIndent(authConfigs, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling auth configs: %v", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Print as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tAUTH TYPE\tAPPLICATION ID\tAPI KEYS")
	for _, config := range authConfigs {
		apiKeyCount := len(config.ApiKeys)
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\n",
			config.Id,
			config.AuthType,
			config.ApplicationId,
			apiKeyCount,
		)
	}
	w.Flush()

	return nil
}

func getAuthConfig(cmd *cobra.Command, args []string) error {
	authConfigId := args[0]
	gatewayClient := getGatewayClient()
	authConfig, _, err := gatewayClient.GetAuthConfig(context.Background(), authConfigId)
	if err != nil {
		return fmt.Errorf("error getting auth config: %v", err)
	}

	data, err := json.MarshalIndent(authConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling auth config: %v", err)
	}

	fmt.Println(string(data))
	return nil
}

func createAuthConfig(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	var authConfig gateway.ApiGatewayAuthConfig
	if err := json.Unmarshal(fileData, &authConfig); err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	gatewayClient := getGatewayClient()
	_, err = gatewayClient.CreateAuthConfig(context.Background(), authConfig)
	if err != nil {
		return fmt.Errorf("error creating auth config: %v", err)
	}

	fmt.Printf("Auth config created successfully: %s\n", authConfig.Id)
	return nil
}

func updateAuthConfig(cmd *cobra.Command, args []string) error {
	authConfigId := args[0]
	filePath := args[1]

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	var authConfig gateway.ApiGatewayAuthConfig
	if err := json.Unmarshal(fileData, &authConfig); err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	gatewayClient := getGatewayClient()
	_, err = gatewayClient.UpdateAuthConfig(context.Background(), authConfigId, authConfig)
	if err != nil {
		return fmt.Errorf("error updating auth config: %v", err)
	}

	fmt.Printf("Auth config updated successfully: %s\n", authConfigId)
	return nil
}

func deleteAuthConfig(cmd *cobra.Command, args []string) error {
	authConfigId := args[0]

	if !confirmDeletion("auth config", authConfigId) {
		fmt.Println("Deletion cancelled")
		return nil
	}

	gatewayClient := getGatewayClient()
	_, err := gatewayClient.DeleteAuthConfig(context.Background(), authConfigId)
	if err != nil {
		return fmt.Errorf("error deleting auth config: %v", err)
	}

	fmt.Printf("Auth config deleted successfully: %s\n", authConfigId)
	return nil
}

// ==================== Route Functions ====================

func listRoutes(cmd *cobra.Command, args []string) error {
	complete, _ := cmd.Flags().GetBool("complete")
	serviceId := args[0]

	gatewayClient := getGatewayClient()
	routes, _, err := gatewayClient.GetRoutes(context.Background(), serviceId)
	if err != nil {
		return fmt.Errorf("error listing routes: %v", err)
	}

	if complete {
		data, err := json.MarshalIndent(routes, "", "  ")
		if err != nil {
			return fmt.Errorf("error marshaling routes: %v", err)
		}
		fmt.Println(string(data))
		return nil
	}

	// Print as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "METHOD\tPATH\tWORKFLOW\tVERSION\tEXECUTION MODE\tDESCRIPTION")
	for _, route := range routes {
		workflowName := ""
		workflowVersion := ""
		if route.MappedWorkflow != nil {
			workflowName = route.MappedWorkflow.Name
			if route.MappedWorkflow.Version > 0 {
				workflowVersion = fmt.Sprintf("%d", route.MappedWorkflow.Version)
			}
		}
		executionMode := route.WorkflowExecutionMode
		if executionMode == "" {
			executionMode = "-"
		}
		description := route.Description
		if description == "" {
			description = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			route.HttpMethod,
			route.Path,
			workflowName,
			workflowVersion,
			executionMode,
			description,
		)
	}
	w.Flush()

	return nil
}

func createRoute(cmd *cobra.Command, args []string) error {
	serviceId := args[0]
	filePath := args[1]

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	var route gateway.ApiGatewayRoute
	if err := json.Unmarshal(fileData, &route); err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	gatewayClient := getGatewayClient()
	_, err = gatewayClient.CreateRoute(context.Background(), serviceId, route)
	if err != nil {
		return fmt.Errorf("error creating route: %v", err)
	}

	fmt.Printf("Route created successfully: %s %s\n", route.HttpMethod, route.Path)
	return nil
}

func updateRoute(cmd *cobra.Command, args []string) error {
	serviceId := args[0]
	routePath := args[1]
	filePath := args[2]

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading file: %v", err)
	}

	var route gateway.ApiGatewayRoute
	if err := json.Unmarshal(fileData, &route); err != nil {
		return fmt.Errorf("error parsing JSON: %v", err)
	}

	gatewayClient := getGatewayClient()
	_, err = gatewayClient.UpdateRoute(context.Background(), serviceId, routePath, route)
	if err != nil {
		return fmt.Errorf("error updating route: %v", err)
	}

	fmt.Printf("Route updated successfully: %s\n", routePath)
	return nil
}

func deleteRoute(cmd *cobra.Command, args []string) error {
	serviceId := args[0]
	httpMethod := args[1]
	routePath := args[2]

	if !confirmDeletion("route", fmt.Sprintf("%s %s", httpMethod, routePath)) {
		fmt.Println("Deletion cancelled")
		return nil
	}

	gatewayClient := getGatewayClient()
	_, err := gatewayClient.DeleteRoute(context.Background(), serviceId, httpMethod, routePath)
	if err != nil {
		return fmt.Errorf("error deleting route: %v", err)
	}

	fmt.Printf("Route deleted successfully: %s %s\n", httpMethod, routePath)
	return nil
}

func init() {
	// Service subcommands
	serviceCmd.AddCommand(
		serviceListCmd,
		serviceGetCmd,
		serviceCreateCmd,
		serviceUpdateCmd,
		serviceDeleteCmd,
	)

	// Auth config subcommands
	authConfigCmd.AddCommand(
		authConfigListCmd,
		authConfigGetCmd,
		authConfigCreateCmd,
		authConfigUpdateCmd,
		authConfigDeleteCmd,
	)

	// Route subcommands
	routeCmd.AddCommand(
		routeListCmd,
		routeCreateCmd,
		routeUpdateCmd,
		routeDeleteCmd,
	)

	// Add to api-gateway command
	apiGatewayCmd.AddCommand(
		serviceCmd,
		authConfigCmd,
		routeCmd,
	)

	// Add flags
	serviceListCmd.Flags().Bool("complete", false, "Print complete JSON output")
	authConfigListCmd.Flags().Bool("complete", false, "Print complete JSON output")
	routeListCmd.Flags().Bool("complete", false, "Print complete JSON output")

	// Add to root
	rootCmd.AddCommand(apiGatewayCmd)
}
