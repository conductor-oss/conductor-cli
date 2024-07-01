package internal

import (
	"github.com/conductor-sdk/conductor-go/sdk/client"
)

var (
	workflowClient *client.WorkflowResourceApiService = nil
	apiClient      *client.APIClient
)

func GetWorkflowClient() *client.WorkflowResourceApiService {
	return &client.WorkflowResourceApiService{APIClient: apiClient}
}

func GetMetadataClient() *client.MetadataResourceApiService {
	metadataClient := &client.MetadataResourceApiService{
		APIClient: apiClient,
	}
	return metadataClient
}

func GetWebhooksConfigClient() client.WebhooksConfigClient {
	webhookConfigClient := client.GetWebhooksConfigService(apiClient)
	return webhookConfigClient

}

func GetSchedulerClient() client.SchedulerClient {
	return client.GetSchedulerService(apiClient)
}

func SetAPIClient(client *client.APIClient) {
	apiClient = client
}
