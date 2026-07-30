package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
