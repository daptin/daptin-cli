package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	grpc_testing "google.golang.org/grpc/reflection/grpc_testing"
)

func main() {
	cli := getenvRequired("DAPTIN_CLI")
	repo := os.Getenv("DAPTIN_CLI_REPO")
	if repo == "" {
		repo = mustGetwd()
	}
	workDir, err := os.MkdirTemp("", "daptin-cli-transport-e2e-*")
	check(err)
	defer os.RemoveAll(workDir)

	httpUpstream := startHTTPUpstream()
	defer httpUpstream.Close()

	grpcAddress, stopGRPC := startGRPCUpstream()
	defer stopGRPC()

	port := freePort()
	httpsPort := freePort()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	stopDaptin := startDaptin(repo, workDir, port, httpsPort, baseURL)
	defer stopDaptin()

	configPath := filepath.Join(workDir, "daptin-cli.yaml")
	env := append(os.Environ(), "DAPTIN_CLI_CONFIG="+configPath)

	run(env, cli, "context", "add", "transport-e2e", baseURL)
	run(env, cli, "context", "set", "transport-e2e")
	email := fmt.Sprintf("transport-e2e-%d@test.local", time.Now().UnixNano())
	password := "TransportE2E123"
	run(env, cli, "execute", "user_account", "signup", "name=Transport E2E", "email="+email, "password="+password, "passwordConfirm="+password)
	run(env, cli, "execute", "user_account", "signin", "email="+email, "password="+password)
	run(env, cli, "execute", "world", "become_an_administrator")
	run(env, cli, "table", "defaults", "get", "integration")
	run(env, cli, "table", "defaults", "ensure", "integration", "--permission", "1618275", "--group", "users:1618275")

	credentialRef := strings.TrimSpace(run(env, cli, "--quiet", "create", "credential", "name=e2e-owner-token", `content={"token":"owner-token"}`))
	if credentialRef == "" {
		log.Fatal("credential create did not return a reference id")
	}

	httpSpec := writeJSON(workDir, "http-protocols.json", httpSpec(httpUpstream.URL))
	grpcSpec := writeJSON(workDir, "grpc-protocols.json", grpcSpec(grpcAddress))
	authSpec := `{"scheme":"bearer","token_field":"token"}`

	run(env, cli, "integration", "import", "--provider", "e2e-http-protocols", "--spec-file", httpSpec, "--auth", "custom_credentials", "--auth-spec-json", authSpec, "--validate")
	run(env, cli, "integration", "install", "e2e-http-protocols")
	run(env, cli, "integration", "import", "--provider", "e2e-grpc-protocols", "--spec-file", grpcSpec, "--auth", "custom_credentials", "--auth-spec-json", authSpec, "--validate")
	run(env, cli, "integration", "install", "e2e-grpc-protocols")

	ops := runJSON(env, cli, "--output", "json", "integration", "operations", "e2e-http-protocols", "--columns", "operation_id,method,path,transport")
	assertArrayHas(ops, "operation_id", "listIssues", "transport", "graphql")
	assertArrayHas(ops, "operation_id", "wsSearch", "transport", "websocket")

	graphQLDescribe := runJSON(env, cli, "--output", "json", "integration", "describe", "e2e-http-protocols", "listIssues")
	assertPath(graphQLDescribe, "transport", "graphql")
	assertPath(graphQLDescribe, "upstream_path", "/graphql")
	grpcDescribe := runJSON(env, cli, "--output", "json", "integration", "describe", "e2e-grpc-protocols", "Search")
	assertPath(grpcDescribe, "transport", "grpc")
	assertPath(grpcDescribe, "grpc_service", "grpc.testing.SearchService")

	rest := runJSON(env, cli, "--output", "json", "integration", "execute", "e2e-http-protocols", "getTask", "--credential-id", credentialRef, "task_gid=TASK-123", "opt_fields=gid,name")
	assertPath(rest, "transport", "rest")
	assertPath(rest, "authorization", "Bearer owner-token")
	assertPath(rest, "task_gid", "TASK-123")

	graphQL := runJSON(env, cli, "--output", "json", "integration", "execute", "e2e-http-protocols", "listIssues", "--credential-id", credentialRef, "--input-json", `{"first":2,"after":"cursor-1"}`)
	assertPath(graphQL, "transport", "graphql")
	assertPath(graphQL, "authorization", "Bearer owner-token")
	assertPath(graphQL, "operationName", "ListIssues")

	ws := runJSON(env, cli, "--output", "json", "integration", "execute", "e2e-http-protocols", "wsSearch", "--credential-id", credentialRef, "query=tickets")
	assertPath(ws, "transport", "websocket")
	assertPath(ws, "authorization", "Bearer owner-token")
	assertPath(ws, "query", "tickets")

	grpcResult := runJSON(env, cli, "--output", "json", "integration", "execute", "e2e-grpc-protocols", "Search", "--credential-id", credentialRef, "query=daptin")
	assertPath(grpcResult, "results.0.title", "daptin")
	assertPath(grpcResult, "results.0.snippets.0", "authorization:ok")

	expectFailure(env, cli, "credential_id is required", "integration", "execute", "e2e-http-protocols", "getTask", "task_gid=TASK-123")
	expectFailure(env, cli, "credential_id is required", "integration", "execute", "e2e-http-protocols", "getTask", "--input-json", fmt.Sprintf(`{"credential_id":%q,"task_gid":"TASK-123"}`, credentialRef))
	expectFailure(env, cli, "credential", "integration", "execute", "e2e-http-protocols", "getTask", "--credential-id", "00000000-0000-0000-0000-000000000000", "task_gid=TASK-123")

	guestEnv := append(os.Environ(), "DAPTIN_CLI_CONFIG="+filepath.Join(workDir, "guest.yaml"))
	expectFailure(guestEnv, cli, "authenticated user", "--endpoint", baseURL, "integration", "execute", "e2e-http-protocols", "getTask", "--credential-id", credentialRef, "task_gid=TASK-123")

	fmt.Println("PASS: daptin-cli integration transport E2E")
}

func startHTTPUpstream() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/rest/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if !requireBearer(w, r) {
			return
		}
		writeJSONResponse(w, map[string]interface{}{
			"transport":     "rest",
			"authorization": r.Header.Get("Authorization"),
			"task_gid":      strings.TrimPrefix(r.URL.Path, "/rest/tasks/"),
			"opt_fields":    r.URL.Query().Get("opt_fields"),
		})
	})
	mux.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		if !requireBearer(w, r) {
			return
		}
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSONResponse(w, map[string]interface{}{
			"transport":     "graphql",
			"authorization": r.Header.Get("Authorization"),
			"operationName": body["operationName"],
			"variables":     body["variables"],
			"data": map[string]interface{}{
				"issues": map[string]interface{}{"nodes": []map[string]interface{}{{"id": "ISS-1", "title": "Issue 1"}}},
			},
		})
	})
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		if !requireBearer(w, r) {
			return
		}
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var message map[string]interface{}
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		_ = conn.WriteJSON(map[string]interface{}{
			"transport":     "websocket",
			"authorization": r.Header.Get("Authorization"),
			"query":         message["query"],
		})
	})
	return httptest.NewServer(mux)
}

func requireBearer(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Authorization") != "Bearer owner-token" {
		http.Error(w, "missing credential authorization", http.StatusUnauthorized)
		return false
	}
	return true
}

type searchServer struct {
	grpc_testing.UnimplementedSearchServiceServer
}

func (searchServer) Search(ctx context.Context, req *grpc_testing.SearchRequest) (*grpc_testing.SearchResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	authOK := "authorization:missing"
	if values := md.Get("authorization"); len(values) > 0 && values[0] == "Bearer owner-token" {
		authOK = "authorization:ok"
	}
	return &grpc_testing.SearchResponse{
		Results: []*grpc_testing.SearchResponse_Result{{
			Url:      "grpc://search/" + req.GetQuery(),
			Title:    req.GetQuery(),
			Snippets: []string{authOK},
		}},
	}, nil
}

func startGRPCUpstream() (string, func()) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	check(err)
	server := grpc.NewServer()
	grpc_testing.RegisterSearchServiceServer(server, searchServer{})
	reflection.Register(server)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	return listener.Addr().String(), func() {
		server.Stop()
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			log.Println("grpc upstream did not stop within timeout")
		}
	}
}

func startDaptin(repo, workDir string, port, httpsPort int, baseURL string) func() {
	daptinBinary := os.Getenv("DAPTIN_BINARY")
	daptinSource := os.Getenv("DAPTIN_SOURCE_DIR")
	if daptinSource == "" {
		daptinSource = filepath.Clean(filepath.Join(repo, "..", "daptin"))
	}

	storageDir := filepath.Join(workDir, "storage")
	check(os.MkdirAll(storageDir, 0o755))
	ctx, cancel := context.WithCancel(context.Background())
	var cmd *exec.Cmd
	args := []string{
		"-port", fmt.Sprintf(":%d", port),
		"-https_port", fmt.Sprintf(":%d", httpsPort),
		"-db_type", "sqlite3",
		"-db_connection_string", filepath.Join(workDir, "daptin.db"),
		"-local_storage_path", storageDir,
		"-runtime", "test",
		"-log_level", "error",
	}
	if daptinBinary != "" {
		cmd = exec.CommandContext(ctx, daptinBinary, args...)
	} else {
		if _, err := os.Stat(filepath.Join(daptinSource, "go.mod")); err != nil {
			cancel()
			log.Fatalf("set DAPTIN_BINARY or DAPTIN_SOURCE_DIR; default source not found at %s", daptinSource)
		}
		cmd = exec.CommandContext(ctx, "go", append([]string{"run", "."}, args...)...)
		cmd.Dir = daptinSource
	}
	logs := &lockedBuffer{}
	cmd.Stdout = logs
	cmd.Stderr = logs
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	check(cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			cancel()
			log.Fatalf("daptin exited before readiness: %v\n%s", err, logs.String())
		default:
		}
		resp, err := http.Get(baseURL + "/api/world")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				return func() {
					if cmd.Process != nil {
						_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
					}
					select {
					case <-done:
					case <-time.After(10 * time.Second):
						if cmd.Process != nil {
							_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
						}
						<-done
					}
					cancel()
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	cancel()
	log.Fatalf("daptin did not become ready\n%s", logs.String())
	return func() {}
}

func httpSpec(serverURL string) map[string]interface{} {
	return baseSpec("E2E HTTP protocols", serverURL, map[string]interface{}{
		"/rest/tasks/{task_gid}": map[string]interface{}{
			"get": map[string]interface{}{
				"operationId": "getTask",
				"parameters": []map[string]interface{}{
					{"name": "task_gid", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					{"name": "opt_fields", "in": "query", "schema": map[string]interface{}{"type": "string"}},
				},
				"responses": jsonResponses(),
			},
		},
		"/linear/listIssues": map[string]interface{}{
			"post": map[string]interface{}{
				"operationId":                     "listIssues",
				"x-daptin-transport":              "graphql",
				"x-daptin-upstream-path":          "/graphql",
				"x-daptin-graphql-operation-name": "ListIssues",
				"x-daptin-graphql-document":       "query ListIssues($first: Int!, $after: String) { issues(first: $first, after: $after) { nodes { id title } } }",
				"requestBody":                     objectRequestBody(map[string]interface{}{"first": map[string]interface{}{"type": "integer"}, "after": map[string]interface{}{"type": "string"}}),
				"responses":                       jsonResponses(),
			},
		},
		"/ws/search": map[string]interface{}{
			"post": map[string]interface{}{
				"operationId":            "wsSearch",
				"x-daptin-transport":     "websocket",
				"x-daptin-upstream-path": "/ws",
				"requestBody":            objectRequestBody(map[string]interface{}{"query": map[string]interface{}{"type": "string"}}),
				"responses":              jsonResponses(),
			},
		},
	})
}

func grpcSpec(grpcAddress string) map[string]interface{} {
	return baseSpec("E2E gRPC protocols", "http://"+grpcAddress, map[string]interface{}{
		"/grpc/search": map[string]interface{}{
			"post": map[string]interface{}{
				"operationId":           "Search",
				"x-daptin-transport":    "grpc",
				"x-daptin-grpc-service": "grpc.testing.SearchService",
				"x-daptin-grpc-method":  "Search",
				"requestBody":           objectRequestBody(map[string]interface{}{"query": map[string]interface{}{"type": "string"}}),
				"responses":             jsonResponses(),
			},
		},
	})
}

func baseSpec(title, serverURL string, paths map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"openapi": "3.0.0",
		"info":    map[string]interface{}{"title": title, "version": "1.0.0"},
		"servers": []map[string]interface{}{{"url": serverURL}},
		"security": []map[string]interface{}{
			{"bearerAuth": []interface{}{}},
		},
		"paths": paths,
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{"type": "http", "scheme": "bearer"},
			},
		},
	}
}

func objectRequestBody(properties map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"required": true,
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{"type": "object", "properties": properties},
			},
		},
	}
}

func jsonResponses() map[string]interface{} {
	return map[string]interface{}{
		"200": map[string]interface{}{
			"description": "OK",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{"schema": map[string]interface{}{"type": "object"}},
			},
		},
	}
}

func runJSON(env []string, command string, args ...string) interface{} {
	out := run(env, command, args...)
	var decoded interface{}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		log.Fatalf("invalid JSON from %s %s: %v\n%s", command, strings.Join(args, " "), err, out)
	}
	return decoded
}

func run(env []string, command string, args ...string) string {
	cmd := exec.Command(command, args...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("command failed: %s %s\n%v\nstdout:\n%s\nstderr:\n%s", command, strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func expectFailure(env []string, command string, contains string, args ...string) {
	cmd := exec.Command(command, args...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		log.Fatalf("expected command to fail: %s %s\nstdout:\n%s\nstderr:\n%s", command, strings.Join(args, " "), stdout.String(), stderr.String())
	}
	combined := strings.ToLower(stdout.String() + "\n" + stderr.String())
	if !strings.Contains(combined, strings.ToLower(contains)) {
		log.Fatalf("failure did not contain %q: %s %s\nstdout:\n%s\nstderr:\n%s", contains, command, strings.Join(args, " "), stdout.String(), stderr.String())
	}
}

func writeJSON(dir, name string, value interface{}) string {
	path := filepath.Join(dir, name)
	data, err := json.MarshalIndent(value, "", "  ")
	check(err)
	check(os.WriteFile(path, data, 0o600))
	return path
}

func writeJSONResponse(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func assertArrayHas(value interface{}, matchKey, matchValue, assertKey, assertValue string) {
	items, ok := value.([]interface{})
	if !ok {
		log.Fatalf("expected array, got %#v", value)
	}
	for _, item := range items {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if fmt.Sprint(row[matchKey]) == matchValue {
			if fmt.Sprint(row[assertKey]) != assertValue {
				log.Fatalf("expected %s=%s for %s=%s, got %#v", assertKey, assertValue, matchKey, matchValue, row[assertKey])
			}
			return
		}
	}
	log.Fatalf("no row with %s=%s in %#v", matchKey, matchValue, value)
}

func assertPath(value interface{}, dottedPath, want string) {
	got, ok := pathValue(value, dottedPath)
	if !ok {
		log.Fatalf("path %s not found in %#v", dottedPath, value)
	}
	if fmt.Sprint(got) != want {
		log.Fatalf("path %s: expected %q, got %#v", dottedPath, want, got)
	}
}

func pathValue(value interface{}, dottedPath string) (interface{}, bool) {
	current := value
	for _, part := range strings.Split(dottedPath, ".") {
		if index, err := strconvAtoi(part); err == nil {
			items, ok := current.([]interface{})
			if !ok || index < 0 || index >= len(items) {
				return nil, false
			}
			current = items[index]
			continue
		}
		object, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func strconvAtoi(value string) (int, error) {
	var result int
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not an int")
		}
		result = result*10 + int(r-'0')
	}
	return result, nil
}

func freePort() int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	check(err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func getenvRequired(name string) string {
	value := os.Getenv(name)
	if value == "" {
		log.Fatalf("%s is required", name)
	}
	return value
}

func mustGetwd() string {
	wd, err := os.Getwd()
	check(err)
	return wd
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
