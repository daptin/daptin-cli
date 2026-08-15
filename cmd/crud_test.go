package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/daptin/daptin-cli/config"
)

func TestListCommandOutputContracts(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		args       []string
		wantOutput string
		wantJSON   interface{}
	}{
		{
			name:     "empty JSON is an array",
			response: `{"data":[]}`,
			args:     []string{"--output", "json"},
			wantJSON: []interface{}{},
		},
		{
			name:       "empty table retains a human-readable message",
			response:   `{"data":[]}`,
			wantOutput: "No data to print\n",
		},
		{
			name:       "empty quiet output is silent",
			response:   `{"data":[]}`,
			args:       []string{"--quiet"},
			wantOutput: "",
		},
		{
			name:     "non-empty JSON remains an array of attributes",
			response: `{"data":[{"id":"1","type":"document","attributes":{"document_name":"guide","reference_id":"ref-1"}}]}`,
			args:     []string{"--output", "json"},
			wantJSON: []interface{}{map[string]interface{}{"document_name": "guide", "reference_id": "ref-1"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/document" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			args := []string{"daptin", "--endpoint", server.URL}
			args = append(args, tt.args...)
			args = append(args, "list", "document")

			var runErr error
			stdout := captureCRUDStdout(t, func() {
				runErr = NewApp(&config.Config{}, "test").Run(args)
			})
			if runErr != nil {
				t.Fatalf("run list: %v", runErr)
			}

			if tt.wantJSON != nil {
				var got interface{}
				if err := json.Unmarshal([]byte(stdout), &got); err != nil {
					t.Fatalf("output is not valid JSON: %v; output: %q", err, stdout)
				}
				want, _ := json.Marshal(tt.wantJSON)
				gotJSON, _ := json.Marshal(got)
				if string(gotJSON) != string(want) {
					t.Fatalf("unexpected JSON: got %s, want %s", gotJSON, want)
				}
			} else if stdout != tt.wantOutput {
				t.Fatalf("unexpected output: got %q, want %q", stdout, tt.wantOutput)
			}
		})
	}
}

func captureCRUDStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = oldStdout
	}()

	output := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		output <- buf.String()
	}()

	fn()
	_ = writer.Close()
	os.Stdout = oldStdout
	return <-output
}

func TestParseAttributes_KeyVal(t *testing.T) {
	args := []string{"name=alice", "email=a@b.com", "age=30"}

	result, err := parseAttributes(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["name"] != "alice" {
		t.Errorf("expected alice, got %v", result["name"])
	}
	if result["email"] != "a@b.com" {
		t.Errorf("expected a@b.com, got %v", result["email"])
	}
	if result["age"] != "30" {
		t.Errorf("expected 30, got %v", result["age"])
	}
}

func TestParseAttributes_JSON(t *testing.T) {
	args := []string{`{"name":"alice","count":5}`}

	result, err := parseAttributes(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["name"] != "alice" {
		t.Errorf("expected alice, got %v", result["name"])
	}
	// JSON numbers are float64
	if result["count"] != float64(5) {
		t.Errorf("expected 5, got %v", result["count"])
	}
}

func TestParseAttributes_Empty(t *testing.T) {
	result, err := parseAttributes([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

func TestParseAttributes_InvalidKeyVal(t *testing.T) {
	_, err := parseAttributes([]string{"no-equals-sign"})
	if err == nil {
		t.Fatal("expected error for missing =")
	}
}

func TestParseAttributes_InvalidJSON(t *testing.T) {
	_, err := parseAttributes([]string{"{bad json"})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseAttributes_ValueWithEquals(t *testing.T) {
	args := []string{"query=name=alice"}

	result, err := parseAttributes(args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["query"] != "name=alice" {
		t.Errorf("expected name=alice, got %v", result["query"])
	}
}

func TestReadBoundedJSONObject(t *testing.T) {
	prefix := `{"value":"`
	suffix := `"}`
	padding := strings.Repeat("x", int(maxCRUDPayloadBytes)-len(prefix)-len(suffix))

	result, err := readBoundedJSONObject(strings.NewReader(prefix + padding + suffix))
	if err != nil {
		t.Fatalf("read payload at limit: %v", err)
	}
	if len(result["value"].(string)) != len(padding) {
		t.Fatalf("payload value was truncated")
	}

	_, err = readBoundedJSONObject(strings.NewReader(strings.Repeat("x", int(maxCRUDPayloadBytes)+1)))
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversized payload error, got %v", err)
	}
}

func TestReadBoundedJSONObjectValidationIsSecretSafe(t *testing.T) {
	_, err := readBoundedJSONObject(strings.NewReader(`{"password":"super-secret"`))
	if err == nil || !strings.Contains(err.Error(), "invalid JSON payload") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
	if strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("error exposed payload contents: %v", err)
	}

	_, err = readBoundedJSONObject(strings.NewReader(`[1,2,3]`))
	if err == nil || !strings.Contains(err.Error(), "must be an object") {
		t.Fatalf("expected object validation error, got %v", err)
	}
}

func TestReadCRUDPayloadSources(t *testing.T) {
	path := t.TempDir() + "/payload.json"
	if err := os.WriteFile(path, []byte(`{"name":"from-file"}`), 0600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	fromFile, err := readCRUDPayload(nil, path, false, strings.NewReader(""))
	if err != nil || fromFile["name"] != "from-file" {
		t.Fatalf("unexpected file payload: %#v, %v", fromFile, err)
	}
	fromStdin, err := readCRUDPayload(nil, "", true, strings.NewReader(`{"name":"from-stdin"}`))
	if err != nil || fromStdin["name"] != "from-stdin" {
		t.Fatalf("unexpected stdin payload: %#v, %v", fromStdin, err)
	}

	conflicts := []struct {
		args       []string
		inputFile  string
		inputStdin bool
	}{
		{args: []string{"name=positional"}, inputFile: path},
		{args: []string{"name=positional"}, inputStdin: true},
		{inputFile: path, inputStdin: true},
	}
	for _, conflict := range conflicts {
		if _, err := readCRUDPayload(conflict.args, conflict.inputFile, conflict.inputStdin, strings.NewReader(`{}`)); err == nil {
			t.Fatalf("expected ambiguous source error for %#v", conflict)
		}
	}
}

func TestCreateInputFilePreservesJSONAPIPayload(t *testing.T) {
	payload := `{"data":{"type":"mail_account","attributes":{"username":"probe@example.invalid"},"relationships":{"mail_server_id":{"data":{"type":"mail_server","id":"server-ref"}}}}}`
	path := t.TempDir() + "/payload.json"
	if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"data":{"type":"mail_account","id":"created","attributes":{"reference_id":"created"}}}`))
	}))
	defer server.Close()

	err := NewApp(&config.Config{}, "test").Run([]string{"daptin", "--quiet", "--endpoint", server.URL, "create", "--input-file", path, "mail_account"})
	if err != nil {
		t.Fatalf("run create: %v", err)
	}
	data := received["data"].(map[string]interface{})
	if _, ok := data["relationships"]; !ok {
		t.Fatalf("relationships were not preserved: %#v", received)
	}
}

func TestUpdateInputStdin(t *testing.T) {
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"data":{"type":"document","id":"doc-ref","attributes":{"reference_id":"doc-ref"}}}`))
	}))
	defer server.Close()

	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	if _, err := writer.WriteString(`{"document_name":"updated.pdf"}`); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	_ = writer.Close()
	os.Stdin = reader
	defer func() {
		os.Stdin = oldStdin
		_ = reader.Close()
	}()

	err = NewApp(&config.Config{}, "test").Run([]string{"daptin", "--quiet", "--endpoint", server.URL, "update", "--input-stdin", "document", "doc-ref"})
	if err != nil {
		t.Fatalf("run update: %v", err)
	}
	data := received["data"].(map[string]interface{})
	attrs := data["attributes"].(map[string]interface{})
	if attrs["document_name"] != "updated.pdf" || data["type"] != "document" || data["id"] != "doc-ref" {
		t.Fatalf("unexpected update body: %#v", received)
	}
}

func TestCreateRejectsAmbiguousInputBeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer server.Close()

	path := t.TempDir() + "/payload.json"
	if err := os.WriteFile(path, []byte(`{"name":"file"}`), 0600); err != nil {
		t.Fatalf("write payload: %v", err)
	}
	err := NewApp(&config.Config{}, "test").Run([]string{"daptin", "--endpoint", server.URL, "create", "--input-file", path, "document", "name=positional"})
	if err == nil || !strings.Contains(err.Error(), "provide only one") {
		t.Fatalf("expected ambiguous source error, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("validation failure made %d HTTP requests", requests)
	}
}
