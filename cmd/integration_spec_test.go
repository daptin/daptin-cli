package cmd

import (
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestValidateIntegrationSpecCatchesGraphQLWithoutDocument(t *testing.T) {
	doc, err := parseYAMLOrJSONMap(`
openapi: 3.0.0
paths:
  /issues:
    post:
      operationId: listIssues
      x-daptin-transport: graphql
      responses:
        "200":
          description: OK
`)
	if err != nil {
		t.Fatal(err)
	}
	result := validateIntegrationSpec(doc)
	if result.Valid {
		t.Fatal("expected validation failure")
	}
	if result.Errors != 1 {
		t.Fatalf("expected 1 error, got %d: %#v", result.Errors, result.Findings)
	}
}

func TestValidateIntegrationSpecInfersGraphQLFromDocument(t *testing.T) {
	doc, err := parseYAMLOrJSONMap(`
openapi: 3.0.0
paths:
  /issues:
    post:
      operationId: listIssues
      x-daptin-graphql-document: query ListIssues { issues { nodes { id } } }
      responses:
        "200":
          description: OK
`)
	if err != nil {
		t.Fatal(err)
	}
	result := validateIntegrationSpec(doc)
	if !result.Valid {
		t.Fatalf("expected valid spec, got %#v", result.Findings)
	}
}

func TestApplyStringOperationAssignments(t *testing.T) {
	doc, err := parseYAMLOrJSONMap(testOpenAPISpec)
	if err != nil {
		t.Fatal(err)
	}
	operations, err := collectOpenAPIOperations(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyStringOperationAssignments(operations, []string{"getWorkspaces=grpc"}, "x-daptin-transport"); err != nil {
		t.Fatal(err)
	}
	if operations["getWorkspaces"]["x-daptin-transport"] != "grpc" {
		t.Fatalf("transport was not patched: %#v", operations["getWorkspaces"])
	}
}

func TestParseOperationAssignmentAllowsEqualsInValue(t *testing.T) {
	operationID, value, err := parseOperationAssignment("listIssues=query A { node(id: \"a=b\") { id } }")
	if err != nil {
		t.Fatal(err)
	}
	if operationID != "listIssues" {
		t.Fatalf("unexpected operation id %q", operationID)
	}
	if value != "query A { node(id: \"a=b\") { id } }" {
		t.Fatalf("unexpected value %q", value)
	}
}

func TestValidateGRPCDescriptorForOperation(t *testing.T) {
	descriptorBytes := testGRPCDescriptor(t, false)
	operation := map[string]interface{}{
		"x-daptin-grpc-service": "grpc.testing.SearchService",
		"x-daptin-grpc-method":  "Search",
	}
	if err := validateGRPCDescriptorForOperation("Search", operation, descriptorBytes); err != nil {
		t.Fatal(err)
	}
}

func TestValidateGRPCDescriptorRejectsStreaming(t *testing.T) {
	descriptorBytes := testGRPCDescriptor(t, true)
	operation := map[string]interface{}{
		"x-daptin-grpc-service": "grpc.testing.SearchService",
		"x-daptin-grpc-method":  "Search",
	}
	if err := validateGRPCDescriptorForOperation("Search", operation, descriptorBytes); err == nil {
		t.Fatal("expected streaming method error")
	}
}

func testGRPCDescriptor(t *testing.T, serverStreaming bool) []byte {
	t.Helper()
	descriptor := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{
			{
				Name:    proto.String("search.proto"),
				Package: proto.String("grpc.testing"),
				MessageType: []*descriptorpb.DescriptorProto{
					{Name: proto.String("SearchRequest")},
					{Name: proto.String("SearchResponse")},
				},
				Service: []*descriptorpb.ServiceDescriptorProto{
					{
						Name: proto.String("SearchService"),
						Method: []*descriptorpb.MethodDescriptorProto{
							{
								Name:            proto.String("Search"),
								InputType:       proto.String(".grpc.testing.SearchRequest"),
								OutputType:      proto.String(".grpc.testing.SearchResponse"),
								ServerStreaming: proto.Bool(serverStreaming),
							},
						},
					},
				},
			},
		},
	}
	data, err := proto.Marshal(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
