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
	webhookConfigClient := client.NewWebhooksConfigClient(apiClient)
	return webhookConfigClient

}

func GetSchedulerClient() client.SchedulerClient {
	return client.NewSchedulerClient(apiClient)
}

func GetTaskClient() *client.TaskResourceApiService {
	return &client.TaskResourceApiService{APIClient: apiClient}
}

func GetSecretsClient() client.SecretsClient {
	return client.NewSecretsClient(apiClient)
}

func GetGatewayClient() client.ApiGatewayClient {
	return client.NewApiGatewayClient(apiClient)
}

func SetAPIClient(client *client.APIClient) {
	apiClient = client
}
