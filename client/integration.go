package client

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// ExecuteIntegrationOperation calls Daptin's provider-scoped integration route.
func (e *ExtendedClient) ExecuteIntegrationOperation(providerName, operationName string, body map[string]interface{}) (interface{}, error) {
	u := fmt.Sprintf("%s/integration/%s/%s",
		e.Endpoint,
		url.PathEscape(providerName),
		url.PathEscape(operationName),
	)

	resp, err := e.nextRequest().SetBody(body).Post(u)
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("parse integration operation response: %w", err)
	}
	return result, nil
}

// IntegrationOperations fetches compact operation discovery for an installed provider.
func (e *ExtendedClient) IntegrationOperations(providerName string) (map[string]interface{}, error) {
	u := fmt.Sprintf("%s/integration/%s/operations", e.Endpoint, url.PathEscape(providerName))
	resp, err := e.nextRequest().Get(u)
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("parse integration operations response: %w", err)
	}
	return result, nil
}

// IntegrationOperationDescription fetches method/path/input/response hints for one installed provider operation.
func (e *ExtendedClient) IntegrationOperationDescription(providerName, operationName string) (map[string]interface{}, error) {
	u := fmt.Sprintf("%s/integration/%s/operations/%s",
		e.Endpoint,
		url.PathEscape(providerName),
		url.PathEscape(operationName),
	)
	resp, err := e.nextRequest().Get(u)
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, fmt.Errorf("parse integration operation description response: %w", err)
	}
	return result, nil
}
