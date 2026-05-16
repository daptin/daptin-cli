package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type openAPIOperation struct {
	OperationID string
	Method      string
	Path        string
	Object      map[string]interface{}
}

type specValidationFinding struct {
	Level       string `json:"level"`
	OperationID string `json:"operation_id,omitempty"`
	Path        string `json:"path,omitempty"`
	Message     string `json:"message"`
}

type specValidationResult struct {
	Valid    bool                    `json:"valid"`
	Errors   int                     `json:"errors"`
	Warnings int                     `json:"warnings"`
	Findings []specValidationFinding `json:"findings"`
}

func integrationValidateSpecCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:      "validate-spec",
		Usage:     "Validate Daptin transport extensions in an OpenAPI spec",
		ArgsUsage: "",
		UsageText: `daptin integration validate-spec --spec-file ./provider.yaml
   daptin integration validate-spec --spec-file ./provider.json`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "spec-file", Usage: "Read OpenAPI spec from file"},
			&cli.StringFlag{Name: "spec-url", Usage: "Read OpenAPI spec from URL"},
			&cli.BoolFlag{Name: "spec-stdin", Usage: "Read OpenAPI spec from stdin"},
		},
		Action: func(c *cli.Context) error {
			specContent, err := readSpecInput(c)
			if err != nil {
				return err
			}
			doc, err := parseYAMLOrJSONMap(specContent)
			if err != nil {
				return fmt.Errorf("parse spec: %w", err)
			}
			result := validateIntegrationSpec(doc)
			if err := renderSpecValidationResult(appCtx, result); err != nil {
				return err
			}
			if !result.Valid {
				return cli.Exit("integration spec validation failed", 1)
			}
			return nil
		},
	}
}

func prepareIntegrationSpecContent(c *cli.Context, specContent, specFormat string) (string, error) {
	var doc map[string]interface{}
	if integrationSpecPatchRequested(c) {
		parsed, err := parseYAMLOrJSONMap(specContent)
		if err != nil {
			return "", fmt.Errorf("parse spec: %w", err)
		}
		doc = parsed
		if err := applyIntegrationSpecPatches(c, doc); err != nil {
			return "", err
		}
		encoded, err := encodeIntegrationSpec(doc, specFormat)
		if err != nil {
			return "", err
		}
		specContent = encoded
	}
	if c.Bool("validate") {
		if doc == nil {
			parsed, err := parseYAMLOrJSONMap(specContent)
			if err != nil {
				return "", fmt.Errorf("parse spec: %w", err)
			}
			doc = parsed
		}
		result := validateIntegrationSpec(doc)
		if !result.Valid {
			return "", result.validationError()
		}
	}
	return specContent, nil
}

func integrationSpecPatchRequested(c *cli.Context) bool {
	flagNames := []string{
		"set-operation-transport",
		"set-operation-upstream-path",
		"set-operation-timeout-ms",
		"set-graphql-document",
		"set-graphql-document-file",
		"set-graphql-operation-name",
		"set-websocket-message-template",
		"set-websocket-response-selector",
		"set-grpc-service",
		"set-grpc-method",
		"grpc-descriptor-set",
		"grpc-proto",
		"grpc-proto-path",
	}
	for _, name := range flagNames {
		if len(c.StringSlice(name)) > 0 {
			return true
		}
	}
	return false
}

func applyIntegrationSpecPatches(c *cli.Context, doc map[string]interface{}) error {
	operations, err := collectOpenAPIOperations(doc)
	if err != nil {
		return err
	}
	patches := []struct {
		flag string
		key  string
	}{
		{"set-operation-transport", "x-daptin-transport"},
		{"set-operation-upstream-path", "x-daptin-upstream-path"},
		{"set-graphql-document", "x-daptin-graphql-document"},
		{"set-graphql-operation-name", "x-daptin-graphql-operation-name"},
		{"set-websocket-message-template", "x-daptin-websocket-message-template"},
		{"set-websocket-response-selector", "x-daptin-websocket-response-selector"},
		{"set-grpc-service", "x-daptin-grpc-service"},
		{"set-grpc-method", "x-daptin-grpc-method"},
	}
	for _, patch := range patches {
		if err := applyStringOperationAssignments(operations, c.StringSlice(patch.flag), patch.key); err != nil {
			return err
		}
	}
	if err := applyGraphQLDocumentFileAssignments(operations, c.StringSlice("set-graphql-document-file")); err != nil {
		return err
	}
	if err := applyTimeoutAssignments(operations, c.StringSlice("set-operation-timeout-ms")); err != nil {
		return err
	}
	if err := applyGRPCDescriptorSetAssignments(operations, c.StringSlice("grpc-descriptor-set")); err != nil {
		return err
	}
	if err := applyGRPCProtoAssignments(operations, c.StringSlice("grpc-proto"), c.StringSlice("grpc-proto-path"), c.String("protoc")); err != nil {
		return err
	}
	return nil
}

func applyStringOperationAssignments(operations map[string]map[string]interface{}, assignments []string, key string) error {
	for _, assignment := range assignments {
		operationID, value, err := parseOperationAssignment(assignment)
		if err != nil {
			return err
		}
		operation, ok := operations[operationID]
		if !ok {
			return fmt.Errorf("operation %q not found", operationID)
		}
		operation[key] = value
	}
	return nil
}

func applyGraphQLDocumentFileAssignments(operations map[string]map[string]interface{}, assignments []string) error {
	for _, assignment := range assignments {
		operationID, filePath, err := parseOperationAssignment(assignment)
		if err != nil {
			return err
		}
		operation, ok := operations[operationID]
		if !ok {
			return fmt.Errorf("operation %q not found", operationID)
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		operation["x-daptin-graphql-document"] = string(data)
	}
	return nil
}

func applyTimeoutAssignments(operations map[string]map[string]interface{}, assignments []string) error {
	for _, assignment := range assignments {
		operationID, value, err := parseOperationAssignment(assignment)
		if err != nil {
			return err
		}
		timeout, err := strconv.Atoi(value)
		if err != nil || timeout <= 0 {
			return fmt.Errorf("invalid timeout for operation %q: %q", operationID, value)
		}
		operation, ok := operations[operationID]
		if !ok {
			return fmt.Errorf("operation %q not found", operationID)
		}
		operation["x-daptin-timeout-ms"] = timeout
	}
	return nil
}

func applyGRPCDescriptorSetAssignments(operations map[string]map[string]interface{}, assignments []string) error {
	for _, assignment := range assignments {
		operationID, filePath, err := parseOperationAssignment(assignment)
		if err != nil {
			return err
		}
		operation, ok := operations[operationID]
		if !ok {
			return fmt.Errorf("operation %q not found", operationID)
		}
		descriptorBytes, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		if err := validateGRPCDescriptorForOperation(operationID, operation, descriptorBytes); err != nil {
			return err
		}
		operation["x-daptin-grpc-descriptor-base64"] = base64.StdEncoding.EncodeToString(descriptorBytes)
	}
	return nil
}

func applyGRPCProtoAssignments(operations map[string]map[string]interface{}, protoAssignments, pathAssignments []string, protocPath string) error {
	if len(protoAssignments) == 0 {
		return nil
	}
	protosByOperation, err := groupedOperationAssignments(protoAssignments)
	if err != nil {
		return err
	}
	pathsByOperation, err := groupedOperationAssignments(pathAssignments)
	if err != nil {
		return err
	}
	if strings.TrimSpace(protocPath) == "" {
		protocPath = "protoc"
	}
	for operationID, protoFiles := range protosByOperation {
		operation, ok := operations[operationID]
		if !ok {
			return fmt.Errorf("operation %q not found", operationID)
		}
		descriptorBytes, err := buildGRPCDescriptorSet(protocPath, protoFiles, pathsByOperation[operationID])
		if err != nil {
			return fmt.Errorf("generate grpc descriptor for %q: %w", operationID, err)
		}
		if err := validateGRPCDescriptorForOperation(operationID, operation, descriptorBytes); err != nil {
			return err
		}
		operation["x-daptin-grpc-descriptor-base64"] = base64.StdEncoding.EncodeToString(descriptorBytes)
	}
	return nil
}

func groupedOperationAssignments(assignments []string) (map[string][]string, error) {
	grouped := map[string][]string{}
	for _, assignment := range assignments {
		operationID, value, err := parseOperationAssignment(assignment)
		if err != nil {
			return nil, err
		}
		grouped[operationID] = append(grouped[operationID], value)
	}
	return grouped, nil
}

func buildGRPCDescriptorSet(protocPath string, protoFiles, protoPaths []string) ([]byte, error) {
	if len(protoFiles) == 0 {
		return nil, fmt.Errorf("at least one proto file is required")
	}
	tmp, err := os.CreateTemp("", "daptin-cli-*.protoset")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(tmpPath)

	args := []string{"--include_imports", "--descriptor_set_out=" + tmpPath}
	for _, includePath := range descriptorIncludePaths(protoFiles, protoPaths) {
		args = append(args, "-I", includePath)
	}
	args = append(args, protoFiles...)
	cmd := exec.Command(protocPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, strings.TrimSpace(string(output)))
	}
	return os.ReadFile(tmpPath)
}

func descriptorIncludePaths(protoFiles, protoPaths []string) []string {
	seen := map[string]bool{}
	var paths []string
	for _, includePath := range protoPaths {
		includePath = strings.TrimSpace(includePath)
		if includePath == "" || seen[includePath] {
			continue
		}
		seen[includePath] = true
		paths = append(paths, includePath)
	}
	for _, protoFile := range protoFiles {
		dir := filepath.Dir(protoFile)
		if dir == "" || dir == "." || seen[dir] {
			continue
		}
		seen[dir] = true
		paths = append(paths, dir)
	}
	if len(paths) == 0 {
		return []string{"."}
	}
	return paths
}

func validateGRPCDescriptorForOperation(operationID string, operation map[string]interface{}, descriptorBytes []byte) error {
	serviceName := strings.TrimSpace(integrationSpecString(operation, "x-daptin-grpc-service"))
	if serviceName == "" {
		return fmt.Errorf("%s: x-daptin-grpc-service is required before embedding a grpc descriptor", operationID)
	}
	methodName := strings.Trim(strings.TrimSpace(integrationSpecString(operation, "x-daptin-grpc-method")), "/")
	if methodName == "" {
		methodName = operationID
	}

	var descriptorSet descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(descriptorBytes, &descriptorSet); err != nil {
		return fmt.Errorf("%s: parse grpc descriptor set: %w", operationID, err)
	}
	for _, file := range descriptorSet.File {
		pkg := file.GetPackage()
		for _, service := range file.Service {
			fullServiceName := service.GetName()
			if pkg != "" {
				fullServiceName = pkg + "." + fullServiceName
			}
			if fullServiceName != serviceName {
				continue
			}
			for _, method := range service.Method {
				if method.GetName() != methodName {
					continue
				}
				if method.GetClientStreaming() || method.GetServerStreaming() {
					return fmt.Errorf("%s: grpc method %s/%s uses streaming; Daptin currently supports unary grpc only", operationID, serviceName, methodName)
				}
				return nil
			}
			return fmt.Errorf("%s: grpc method %q not found in service %q", operationID, methodName, serviceName)
		}
	}
	return fmt.Errorf("%s: grpc service %q not found in descriptor set", operationID, serviceName)
}

func parseOperationAssignment(value string) (string, string, error) {
	operationID, assignmentValue, ok := strings.Cut(value, "=")
	if !ok {
		return "", "", fmt.Errorf("expected operation=value assignment, got %q", value)
	}
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return "", "", fmt.Errorf("operation id is required in assignment %q", value)
	}
	return operationID, assignmentValue, nil
}

func collectOpenAPIOperations(doc map[string]interface{}) (map[string]map[string]interface{}, error) {
	paths, ok := doc["paths"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("spec has no paths object")
	}
	operations := map[string]map[string]interface{}{}
	for path, rawPathItem := range paths {
		pathItem, ok := rawPathItem.(map[string]interface{})
		if !ok {
			continue
		}
		for method, rawOperation := range pathItem {
			if !isOpenAPIMethod(method) {
				continue
			}
			operation, ok := rawOperation.(map[string]interface{})
			if !ok {
				continue
			}
			operationID, _ := operation["operationId"].(string)
			operationID = strings.TrimSpace(operationID)
			if operationID == "" {
				continue
			}
			if _, exists := operations[operationID]; exists {
				return nil, fmt.Errorf("duplicate operationId %q", operationID)
			}
			_ = path
			operations[operationID] = operation
		}
	}
	return operations, nil
}

func collectOpenAPIOperationRecords(doc map[string]interface{}) ([]openAPIOperation, error) {
	paths, ok := doc["paths"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("spec has no paths object")
	}
	var operations []openAPIOperation
	for path, rawPathItem := range paths {
		pathItem, ok := rawPathItem.(map[string]interface{})
		if !ok {
			continue
		}
		for method, rawOperation := range pathItem {
			if !isOpenAPIMethod(method) {
				continue
			}
			operation, ok := rawOperation.(map[string]interface{})
			if !ok {
				continue
			}
			operationID, _ := operation["operationId"].(string)
			operations = append(operations, openAPIOperation{
				OperationID: strings.TrimSpace(operationID),
				Method:      strings.ToUpper(method),
				Path:        path,
				Object:      operation,
			})
		}
	}
	return operations, nil
}

func isOpenAPIMethod(method string) bool {
	switch strings.ToLower(method) {
	case "get", "put", "post", "delete", "options", "head", "patch", "trace":
		return true
	default:
		return false
	}
}

func validateIntegrationSpec(doc map[string]interface{}) specValidationResult {
	result := specValidationResult{Valid: true}
	operations, err := collectOpenAPIOperationRecords(doc)
	if err != nil {
		result.add("error", "", "", err.Error())
		return result
	}
	for _, operation := range operations {
		validateIntegrationOperation(operation, &result)
	}
	return result
}

func validateIntegrationOperation(operation openAPIOperation, result *specValidationResult) {
	object := operation.Object
	for _, key := range []string{
		"x-daptin-transport",
		"x-daptin-upstream-path",
		"x-daptin-graphql-document",
		"x-daptin-graphql-operation-name",
		"x-daptin-websocket-message-template",
		"x-daptin-websocket-response-selector",
		"x-daptin-grpc-service",
		"x-daptin-grpc-method",
		"x-daptin-grpc-descriptor-base64",
	} {
		if _, ok := object[key]; ok {
			if _, ok := object[key].(string); !ok {
				result.add("error", operation.OperationID, operation.Path, fmt.Sprintf("%s must be a string", key))
			}
		}
	}
	if timeout, ok := object["x-daptin-timeout-ms"]; ok {
		if _, ok := timeout.(float64); !ok {
			if _, ok := timeout.(int); !ok {
				result.add("error", operation.OperationID, operation.Path, "x-daptin-timeout-ms must be a number")
			}
		}
	}

	transport := integrationSpecString(object, "x-daptin-transport")
	graphqlDocument := strings.TrimSpace(integrationSpecString(object, "x-daptin-graphql-document"))
	if transport == "" {
		transport = "rest"
	}
	transport = strings.ToLower(strings.TrimSpace(transport))
	if transport == "rest" && graphqlDocument != "" {
		transport = "graphql"
	}
	switch transport {
	case "rest", "graphql", "websocket", "grpc":
	default:
		result.add("error", operation.OperationID, operation.Path, fmt.Sprintf("unsupported x-daptin-transport %q", transport))
	}
	if transport == "graphql" && graphqlDocument == "" {
		result.add("error", operation.OperationID, operation.Path, "x-daptin-graphql-document is required for graphql transport")
	}
	if transport == "grpc" && strings.TrimSpace(integrationSpecString(object, "x-daptin-grpc-service")) == "" {
		result.add("error", operation.OperationID, operation.Path, "x-daptin-grpc-service is required for grpc transport")
	}
	if (transport == "websocket" || transport == "grpc") && !operationHasRequestBodySchema(object) {
		result.add("warning", operation.OperationID, operation.Path, fmt.Sprintf("%s operation has no request body schema; input mapping may be limited", transport))
	}
}

func integrationSpecString(operation map[string]interface{}, key string) string {
	value, _ := operation[key].(string)
	return value
}

func operationHasRequestBodySchema(operation map[string]interface{}) bool {
	requestBody, ok := operation["requestBody"].(map[string]interface{})
	if !ok {
		return false
	}
	content, ok := requestBody["content"].(map[string]interface{})
	if !ok {
		return false
	}
	for _, rawMedia := range content {
		media, ok := rawMedia.(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := media["schema"].(map[string]interface{}); ok {
			return true
		}
	}
	return false
}

func (r *specValidationResult) add(level, operationID, path, message string) {
	if level == "error" {
		r.Valid = false
		r.Errors++
	} else if level == "warning" {
		r.Warnings++
	}
	r.Findings = append(r.Findings, specValidationFinding{
		Level:       level,
		OperationID: operationID,
		Path:        path,
		Message:     message,
	})
}

func (r specValidationResult) validationError() error {
	messages := make([]string, 0, r.Errors)
	for _, finding := range r.Findings {
		if finding.Level != "error" {
			continue
		}
		location := finding.OperationID
		if location == "" {
			location = finding.Path
		}
		if location != "" {
			messages = append(messages, fmt.Sprintf("%s: %s", location, finding.Message))
		} else {
			messages = append(messages, finding.Message)
		}
	}
	return fmt.Errorf("integration spec validation failed: %s", strings.Join(messages, "; "))
}

func renderSpecValidationResult(appCtx *AppContext, result specValidationResult) error {
	findings := make([]map[string]interface{}, 0, len(result.Findings))
	for _, finding := range result.Findings {
		row := map[string]interface{}{
			"level":   finding.Level,
			"message": finding.Message,
		}
		if finding.OperationID != "" {
			row["operation_id"] = finding.OperationID
		}
		if finding.Path != "" {
			row["path"] = finding.Path
		}
		findings = append(findings, row)
	}
	return appCtx.Renderer.RenderObject(map[string]interface{}{
		"valid":    result.Valid,
		"errors":   result.Errors,
		"warnings": result.Warnings,
		"findings": findings,
	})
}

func encodeIntegrationSpec(doc map[string]interface{}, specFormat string) (string, error) {
	if specFormat == "json" {
		data, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
