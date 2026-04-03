package client

import (
	"encoding/json"
	"fmt"

	daptinClient "github.com/daptin/daptin-go-client"
)

// FindRelated fetches related rows via a relationship column.
// GET /api/{entity}/{referenceId}/{relationColumn}
func (e *ExtendedClient) FindRelated(entityName, referenceId, relationColumn string, parameters daptinClient.DaptinQueryParameters) ([]daptinClient.JsonApiObject, error) {
	req := e.nextRequest()

	u := e.Endpoint + "/api/" + entityName + "/" + referenceId + "/" + relationColumn

	resp, err := req.Get(u)
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}

	var responseObject map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &responseObject); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	data, ok := responseObject["data"].([]interface{})
	if !ok {
		return nil, nil
	}

	result := make([]daptinClient.JsonApiObject, 0, len(data))
	for _, item := range data {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result, nil
}

// AddRelation associates a target entity with a source via a relationship.
// POST /api/{entity}/{referenceId}/{relationColumn}
func (e *ExtendedClient) AddRelation(entityName, referenceId, relationColumn, targetType, targetRefId string) error {
	body := map[string]interface{}{
		"data": map[string]interface{}{
			"type": targetType,
			"id":   targetRefId,
		},
	}

	resp, err := e.nextRequest().SetBody(body).Post(
		e.Endpoint + "/api/" + entityName + "/" + referenceId + "/" + relationColumn,
	)
	return e.checkResponse(resp, err)
}

// RemoveRelation removes a relationship association.
// DELETE /api/{entity}/{referenceId}/{relationColumn}/{targetRefId}
func (e *ExtendedClient) RemoveRelation(entityName, referenceId, relationColumn, targetRefId string) error {
	resp, err := e.nextRequest().Delete(
		e.Endpoint + "/api/" + entityName + "/" + referenceId + "/" + relationColumn + "/" + targetRefId,
	)
	return e.checkResponse(resp, err)
}
