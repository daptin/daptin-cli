package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	daptinClient "github.com/daptin/daptin-go-client"
)

// ResponseEffect describes a side effect to perform after processing an action response.
// Pure value — no IO happens here.
type ResponseEffect struct {
	Type string // "token", "notify", "redirect", "render_object", "file_download", "noop"

	// For "token"
	Token string

	// For "notify" / "redirect"
	Message string

	// For "render_object" / "file_download"
	Data map[string]interface{}
}

type ActionSchema struct {
	EntityName       string
	ActionName       string
	ReferenceID      string
	InstanceOptional bool
	InFields         []map[string]interface{}
}

// ProcessResponses converts raw action responses into a list of effects.
// Pure function: values in, values out. No IO.
func ProcessResponses(responses []daptinClient.DaptinActionResponse) []ResponseEffect {
	effects := make([]ResponseEffect, 0, len(responses))

	for _, r := range responses {
		slog.Debug("processing response", "type", r.ResponseType)
		switch r.ResponseType {
		case "client.store.set":
			key, _ := r.Attributes["key"].(string)
			if key == "token" {
				token, _ := r.Attributes["value"].(string)
				if token != "" {
					effects = append(effects, ResponseEffect{Type: "token", Token: token})
				}
			}
		case "client.notify":
			msg, _ := r.Attributes["message"].(string)
			effects = append(effects, ResponseEffect{Type: "notify", Message: msg})
		case "client.redirect":
			loc, _ := r.Attributes["location"].(string)
			effects = append(effects, ResponseEffect{Type: "redirect", Message: loc})
		case "client.file.download":
			effects = append(effects, ResponseEffect{Type: "file_download", Data: r.Attributes})
		case "client.cookie.set":
			// noop in CLI
		default:
			if len(r.Attributes) > 0 {
				effects = append(effects, ResponseEffect{Type: "render_object", Data: r.Attributes})
			}
		}
	}
	return effects
}

func BuildActionSuccessEffect(entityName, actionName, referenceID string) ResponseEffect {
	message := fmt.Sprintf("OK: %s.%s executed", entityName, actionName)
	data := map[string]interface{}{
		"ok":     true,
		"entity": entityName,
		"action": actionName,
	}
	if referenceID != "" {
		message = fmt.Sprintf("%s for %s", message, referenceID)
		data["reference_id"] = referenceID
	}
	return ResponseEffect{Type: "success", Message: message, Data: data}
}

// FieldPrompt describes a field that needs user input.
// Pure value — computed from schema + already-provided attrs.
type FieldPrompt struct {
	ColumnName string
	Label      string
	ColumnType string // "password" gets masked input
	IsNullable bool
}

// MissingFields computes which InFields still need values.
// Pure function: schema + existing values in, prompts out.
func MissingFields(inFields []map[string]interface{}, existing map[string]interface{}) []FieldPrompt {
	var prompts []FieldPrompt
	for _, field := range inFields {
		colName, _ := field["ColumnName"].(string)
		if colName == "" {
			continue
		}
		if _, provided := existing[colName]; provided {
			continue
		}

		colType, _ := field["ColumnType"].(string)
		isNullable, _ := field["IsNullable"].(bool)
		label := colName
		if name, ok := field["Name"].(string); ok && name != "" {
			label = name
		}

		prompts = append(prompts, FieldPrompt{
			ColumnName: colName,
			Label:      label,
			ColumnType: colType,
			IsNullable: isNullable,
		})
	}
	return prompts
}

// ParseActionSchema extracts InFields from a raw action_schema JSON string.
// Pure function.
func ParseActionSchema(schemaJson string) ([]map[string]interface{}, error) {
	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(schemaJson), &schema); err != nil {
		return nil, err
	}

	inFields, ok := schema["InFields"].([]interface{})
	if !ok {
		return nil, nil
	}

	result := make([]map[string]interface{}, 0, len(inFields))
	for _, f := range inFields {
		if fm, ok := f.(map[string]interface{}); ok {
			result = append(result, fm)
		}
	}
	return result, nil
}

// FindWorldRefId finds a world's reference_id by table_name from pre-fetched world attributes.
// Pure function.
func FindWorldRefId(worldAttrs []map[string]interface{}, entityName string) string {
	for _, w := range worldAttrs {
		if w["table_name"] == entityName {
			if refId, ok := w["reference_id"].(string); ok {
				return refId
			}
		}
	}
	return ""
}

// FindActionRefId finds an action's reference_id by name and world_id from pre-fetched action attributes.
// Pure function.
func FindActionRefId(actionAttrs []map[string]interface{}, worldRefId, actionName string) string {
	meta := FindActionMetadata(actionAttrs, worldRefId, "", actionName)
	return meta.ReferenceID
}

func FindActionMetadata(actionAttrs []map[string]interface{}, worldRefId, entityName, actionName string) ActionSchema {
	for _, a := range actionAttrs {
		if a["action_name"] == actionName && a["world_id"] == worldRefId {
			meta := ActionSchema{
				EntityName:       entityName,
				ActionName:       actionName,
				InstanceOptional: boolValue(a["instance_optional"]),
			}
			if refID, ok := a["reference_id"].(string); ok {
				meta.ReferenceID = refID
			}
			return meta
		}
	}
	return ActionSchema{}
}

// DecodeActionSchemaResponse extracts and parses InFields from a get_action_schema response.
// The server returns the schema as base64 in a client.file.download response.
// Pure function.
func DecodeActionSchemaResponse(responses []daptinClient.DaptinActionResponse) ([]map[string]interface{}, error) {
	slog.Debug("decoding action schema response", "response_count", len(responses))
	for _, r := range responses {
		if r.ResponseType == "client.file.download" {
			contentB64, _ := r.Attributes["content"].(string)
			if contentB64 == "" {
				continue
			}
			decoded, err := base64.StdEncoding.DecodeString(contentB64)
			if err != nil {
				return nil, fmt.Errorf("decode action schema: %w", err)
			}
			return ParseActionSchema(string(decoded))
		}
	}
	return nil, fmt.Errorf("no schema in response")
}

func boolValue(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	default:
		return false
	}
}
