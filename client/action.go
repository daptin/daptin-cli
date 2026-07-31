package client

import (
	"fmt"

	daptinClient "github.com/daptin/daptin-go-client"
)

// Execute overrides the upstream implementation so HTTP failures are checked
// before attempting to decode Daptin action responses.
func (e *ExtendedClient) Execute(actionName, tableName string, attributes daptinClient.JsonApiObject) ([]daptinClient.DaptinActionResponse, error) {
	body := map[string]interface{}{
		"Name":       actionName,
		"OnType":     tableName,
		"Attributes": attributes,
	}
	resp, err := e.nextRequest().SetBody(body).Post(e.Endpoint + "/action/" + tableName + "/" + actionName)
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}
	responses, err := ParseActionResponses(resp.Body())
	if err != nil {
		return nil, fmt.Errorf("execute %s.%s: %w", tableName, actionName, err)
	}
	result := make([]daptinClient.DaptinActionResponse, 0, len(responses))
	for _, response := range responses {
		result = append(result, daptinClient.DaptinActionResponse{
			ResponseType: response.ResponseType,
			Attributes:   response.Attributes,
		})
	}
	return result, nil
}
