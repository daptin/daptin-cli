package client

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	daptinClient "github.com/daptin/daptin-go-client"
)

// RelationTargetType derives the related entity type from a Daptin relation column.
func RelationTargetType(relationColumn string) string {
	return strings.TrimSuffix(relationColumn, "_id")
}

// RelationshipJoinTableName returns the generated Daptin join table for a relation.
func RelationshipJoinTableName(entityName, relationColumn string) string {
	targetType := RelationTargetType(relationColumn)
	sourceColumn := entityName + "_id"
	targetColumn := targetType + "_id"
	return entityName + "_" + sourceColumn + "_has_" + targetType + "_" + targetColumn
}

// FindRelated fetches related rows via a relationship column.
// GET /api/{entity}/{referenceId}/{relationColumn}
func (e *ExtendedClient) FindRelated(entityName, referenceId, relationColumn string, parameters daptinClient.DaptinQueryParameters) ([]daptinClient.JsonApiObject, error) {
	req := e.nextRequest()

	u := e.Endpoint + "/api/" + entityName + "/" + referenceId + "/" + relationColumn
	slog.Debug("FindRelated", "url", u)

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

// AddRelation associates a target entity with a source via JSON:API relationships endpoint.
// PATCH /api/{entity}/{referenceId}/relationships/{relationColumn}
func (e *ExtendedClient) AddRelation(entityName, referenceId, relationColumn, targetType, targetRefId string) error {
	slog.Debug("AddRelation", "entity", entityName, "ref", referenceId, "relation", relationColumn, "target_type", targetType, "target_ref", targetRefId)
	body := map[string]interface{}{
		"data": []map[string]interface{}{
			{"type": targetType, "id": targetRefId},
		},
	}

	resp, err := e.nextRequest().SetBody(body).Patch(
		e.Endpoint + "/api/" + entityName + "/" + referenceId + "/relationships/" + relationColumn,
	)
	return e.checkResponse(resp, err)
}

// UpdateRelationAttributes updates attributes on the generated relationship row.
func (e *ExtendedClient) UpdateRelationAttributes(entityName, referenceId, relationColumn, targetRefId string, attributes map[string]interface{}) (daptinClient.JsonApiObject, error) {
	if len(attributes) == 0 {
		return nil, nil
	}

	rows, err := e.FindRelated(entityName, referenceId, relationColumn, nil)
	if err != nil {
		return nil, err
	}

	relationRef, err := findRelationReference(rows, entityName, referenceId, relationColumn, targetRefId)
	if err != nil {
		return nil, err
	}

	joinTable := RelationshipJoinTableName(entityName, relationColumn)
	body := daptinClient.JsonApiObject{
		"data": map[string]interface{}{
			"type":       joinTable,
			"id":         relationRef,
			"attributes": attributes,
		},
	}

	u := e.Endpoint + "/api/" + joinTable + "/" + relationRef
	slog.Debug("UpdateRelationAttributes", "url", u, "relation_ref", relationRef)

	resp, err := e.nextRequest().SetBody(body).Patch(u)
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}
	if len(resp.Body()) == 0 {
		return nil, nil
	}

	result, err := ParseSingleResponse(resp.Body())
	if err != nil {
		return nil, err
	}
	return daptinClient.JsonApiObject(result), nil
}

func findRelationReference(rows []daptinClient.JsonApiObject, entityName, referenceId, relationColumn, targetRefId string) (string, error) {
	for _, row := range rows {
		if !relationRowMatches(row, entityName, referenceId, relationColumn, targetRefId) {
			continue
		}
		if ref := relationRowReference(row); ref != "" {
			return ref, nil
		}
		return "", fmt.Errorf("relationship row for %s/%s %s -> %s does not expose a reference_id", entityName, referenceId, relationColumn, targetRefId)
	}
	return "", fmt.Errorf("relationship row not found for %s/%s %s -> %s", entityName, referenceId, relationColumn, targetRefId)
}

func relationRowMatches(row daptinClient.JsonApiObject, entityName, referenceId, relationColumn, targetRefId string) bool {
	targetType := RelationTargetType(relationColumn)
	targetKeys := []string{relationColumn, "relation_reference_id", targetType, targetType + "_id", targetType + "_reference_id"}
	_, targetMatches := relationFieldMatch(row, targetKeys, targetRefId)
	if !targetMatches {
		return false
	}

	sourceKeys := []string{entityName, entityName + "_id", entityName + "_reference_id"}
	sourceFound, sourceMatches := relationFieldMatch(row, sourceKeys, referenceId)
	return !sourceFound || sourceMatches
}

func relationFieldMatch(row daptinClient.JsonApiObject, keys []string, ref string) (bool, bool) {
	foundAny := false
	for _, source := range []map[string]interface{}{
		map[string]interface{}(row),
		nestedMap(row, "attributes"),
		nestedMap(row, "relationships"),
	} {
		if source == nil {
			continue
		}
		if found, matched := mapFieldMatch(source, keys, ref); found {
			foundAny = true
			if matched {
				return true, true
			}
		}
	}
	return foundAny, false
}

func mapFieldMatch(values map[string]interface{}, keys []string, ref string) (bool, bool) {
	foundAny := false
	for _, key := range keys {
		value, ok := values[key]
		if !ok {
			continue
		}
		if !valueHasAnyReference(value) {
			continue
		}
		foundAny = true
		if valueHasReference(value, ref) {
			return true, true
		}
	}
	return foundAny, false
}

func valueHasAnyReference(value interface{}) bool {
	switch v := value.(type) {
	case string:
		return v != ""
	case map[string]interface{}:
		for _, key := range []string{"reference_id", "id", "value", "data", "attributes"} {
			if valueHasAnyReference(v[key]) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if valueHasAnyReference(item) {
				return true
			}
		}
	}
	return false
}

func valueHasReference(value interface{}, ref string) bool {
	switch v := value.(type) {
	case string:
		return v == ref
	case map[string]interface{}:
		for _, key := range []string{"reference_id", "id", "value", "data", "attributes"} {
			if valueHasReference(v[key], ref) {
				return true
			}
		}
	case []interface{}:
		for _, item := range v {
			if valueHasReference(item, ref) {
				return true
			}
		}
	}
	return false
}

func relationRowReference(row daptinClient.JsonApiObject) string {
	if attrs := nestedMap(row, "attributes"); attrs != nil {
		if ref, ok := attrs["reference_id"].(string); ok {
			return ref
		}
	}
	if ref, ok := row["reference_id"].(string); ok {
		return ref
	}
	if ref, ok := row["id"].(string); ok {
		return ref
	}
	return ""
}

func nestedMap(row daptinClient.JsonApiObject, key string) map[string]interface{} {
	value, ok := row[key].(map[string]interface{})
	if !ok {
		return nil
	}
	return value
}

// RemoveRelation removes a relationship association via JSON:API relationships endpoint.
// DELETE /api/{entity}/{referenceId}/relationships/{relationColumn}
func (e *ExtendedClient) RemoveRelation(entityName, referenceId, relationColumn, targetType, targetRefId string) error {
	slog.Debug("RemoveRelation", "entity", entityName, "ref", referenceId, "relation", relationColumn, "target_type", targetType, "target_ref", targetRefId)
	body := map[string]interface{}{
		"data": []map[string]interface{}{
			{"type": targetType, "id": targetRefId},
		},
	}

	resp, err := e.nextRequest().SetBody(body).Delete(
		e.Endpoint + "/api/" + entityName + "/" + referenceId + "/relationships/" + relationColumn,
	)
	return e.checkResponse(resp, err)
}
