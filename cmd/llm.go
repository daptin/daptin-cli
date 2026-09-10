package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/daptin/daptin-cli/client"
	"github.com/urfave/cli/v2"
)

const maxLLMRequestBytes int64 = 16 << 20

type llmRequestOptions struct {
	model      string
	shorthand  string
	inputJSON  string
	inputFile  string
	inputStdin bool
	stream     bool
}

func llmCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "llm",
		Usage: "Call Daptin's OpenAI-compatible LLM gateway",
		Description: "Calls the active Daptin context only. Model permission and metering are independent gates; " +
			"HTTP/SSE insufficient_quota and rate_limit_error failures remain distinguishable.",
		Subcommands: []*cli.Command{
			llmModelsCommand(appCtx),
			llmChatCommand(appCtx),
			llmResponsesCommand(appCtx),
			llmEmbeddingsCommand(appCtx),
		},
	}
}

func llmModelsCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "models",
		Usage: "List models visible to the active Daptin account",
		Action: func(c *cli.Context) error {
			response, err := appCtx.Client.LLMModels(c.Context)
			if err != nil {
				return llmCommandError(c.Context, err)
			}
			defer response.Body.Close()
			if err := validateLLMContentType(response.Header, false); err != nil {
				return err
			}
			if c.String("output") == "json" {
				_, err = io.Copy(os.Stdout, response.Body)
				return llmCommandError(c.Context, err)
			}
			return renderLLMModels(appCtx, response.Body)
		},
	}
}

func llmChatCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "chat",
		Usage: "Create a chat completion through Daptin",
		UsageText: `daptin llm chat --model assistant --prompt "Hello"
   daptin llm chat --model assistant --input-file request.json
   daptin llm chat --input-stdin --stream`,
		Flags: append(llmRequestFlags(),
			&cli.StringFlag{Name: "prompt", Usage: "Build a single user message from `TEXT`"},
			&cli.BoolFlag{Name: "stream", Usage: "Request and consume an SSE stream"},
		),
		Action: func(c *cli.Context) error {
			body, streaming, err := buildLLMRequest("chat", llmOptions(c, c.String("prompt")), os.Stdin)
			if err != nil {
				return err
			}
			return executeLLMPost(c, appCtx, "/v1/chat/completions", body, streaming, "chat")
		},
	}
}

func llmResponsesCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "responses",
		Usage: "Create a stateless Responses result through Daptin",
		UsageText: `daptin llm responses --model assistant --input "Hello"
   daptin llm responses --model assistant --input-file request.json
   daptin llm responses --input-stdin --stream`,
		Flags: append(llmRequestFlags(),
			&cli.StringFlag{Name: "input", Usage: "Use `TEXT` as the Responses input"},
			&cli.BoolFlag{Name: "stream", Usage: "Request and consume an SSE stream"},
		),
		Action: func(c *cli.Context) error {
			body, streaming, err := buildLLMRequest("responses", llmOptions(c, c.String("input")), os.Stdin)
			if err != nil {
				return err
			}
			return executeLLMPost(c, appCtx, "/v1/responses", body, streaming, "responses")
		},
	}
}

func llmEmbeddingsCommand(appCtx *AppContext) *cli.Command {
	return &cli.Command{
		Name:  "embeddings",
		Usage: "Create embeddings through Daptin",
		UsageText: `daptin llm embeddings --model embedding-model --input "Hello"
   daptin llm embeddings --model embedding-model --input-file request.json`,
		Flags: append(llmRequestFlags(),
			&cli.StringFlag{Name: "input", Usage: "Use `TEXT` as the embedding input"},
		),
		Action: func(c *cli.Context) error {
			body, streaming, err := buildLLMRequest("embeddings", llmOptions(c, c.String("input")), os.Stdin)
			if err != nil {
				return err
			}
			if streaming {
				return fmt.Errorf("embeddings do not support streaming")
			}
			return executeLLMPost(c, appCtx, "/v1/embeddings", body, false, "embeddings")
		},
	}
}

func llmRequestFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "model", Usage: "Daptin public model `NAME`"},
		&cli.StringFlag{Name: "input-json", Usage: "Complete JSON request object (maximum 16 MiB)"},
		&cli.StringFlag{Name: "input-file", Usage: "Read a complete JSON request object from `FILE` (maximum 16 MiB)"},
		&cli.BoolFlag{Name: "input-stdin", Usage: "Read a complete JSON request object from stdin (maximum 16 MiB)"},
	}
}

func llmOptions(c *cli.Context, shorthand string) llmRequestOptions {
	return llmRequestOptions{
		model: c.String("model"), shorthand: shorthand, inputJSON: c.String("input-json"),
		inputFile: c.String("input-file"), inputStdin: c.Bool("input-stdin"), stream: c.Bool("stream"),
	}
}

func buildLLMRequest(operation string, options llmRequestOptions, stdin io.Reader) ([]byte, bool, error) {
	provided := 0
	if options.inputJSON != "" {
		provided++
	}
	if options.inputFile != "" {
		provided++
	}
	if options.inputStdin {
		provided++
	}
	if provided > 1 {
		return nil, false, fmt.Errorf("provide only one of --input-json, --input-file, or --input-stdin")
	}
	if provided > 0 && options.shorthand != "" {
		return nil, false, fmt.Errorf("the shorthand input flag cannot be combined with a complete JSON request")
	}

	request := map[string]json.RawMessage{}
	var err error
	switch {
	case options.inputJSON != "":
		request, err = decodeBoundedLLMRequest(strings.NewReader(options.inputJSON))
	case options.inputFile != "":
		var file *os.File
		file, err = os.Open(options.inputFile)
		if err == nil {
			defer file.Close()
			request, err = decodeBoundedLLMRequest(file)
		} else {
			err = fmt.Errorf("open input file: %w", err)
		}
	case options.inputStdin:
		request, err = decodeBoundedLLMRequest(stdin)
	}
	if err != nil {
		return nil, false, err
	}

	if options.model != "" {
		if raw, exists := request["model"]; exists {
			var bodyModel string
			if json.Unmarshal(raw, &bodyModel) != nil || bodyModel == "" {
				return nil, false, fmt.Errorf("request model must be a non-empty string")
			}
			if bodyModel != options.model {
				return nil, false, fmt.Errorf("--model conflicts with request model %q", bodyModel)
			}
		} else {
			request["model"] = mustJSONRaw(options.model)
		}
	}
	var model string
	if raw, exists := request["model"]; !exists || json.Unmarshal(raw, &model) != nil || strings.TrimSpace(model) == "" {
		return nil, false, fmt.Errorf("a non-empty model is required in --model or the JSON request")
	}

	if provided == 0 {
		if options.shorthand == "" {
			return nil, false, fmt.Errorf("provide a shorthand input or a complete JSON request")
		}
		switch operation {
		case "chat":
			request["messages"] = mustJSONRaw([]map[string]string{{"role": "user", "content": options.shorthand}})
		case "responses", "embeddings":
			request["input"] = mustJSONRaw(options.shorthand)
		}
	}

	streaming := false
	if raw, exists := request["stream"]; exists {
		if err := json.Unmarshal(raw, &streaming); err != nil {
			return nil, false, fmt.Errorf("request stream must be a boolean")
		}
	}
	if options.stream {
		if raw, exists := request["stream"]; exists {
			var bodyStream bool
			_ = json.Unmarshal(raw, &bodyStream)
			if !bodyStream {
				return nil, false, fmt.Errorf("--stream conflicts with request stream=false")
			}
		}
		streaming = true
		request["stream"] = json.RawMessage("true")
	}
	if operation == "embeddings" && streaming {
		return nil, false, fmt.Errorf("embeddings do not support streaming")
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, false, fmt.Errorf("encode LLM request: %w", err)
	}
	return body, streaming, nil
}

func decodeBoundedLLMRequest(reader io.Reader) (map[string]json.RawMessage, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxLLMRequestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read JSON request: %w", err)
	}
	if int64(len(data)) > maxLLMRequestBytes {
		return nil, fmt.Errorf("JSON request exceeds %d-byte limit", maxLLMRequestBytes)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("invalid JSON request object")
	}
	if trimmed[0] != '{' {
		return nil, fmt.Errorf("JSON request must be an object")
	}
	var request map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &request); err != nil {
		return nil, fmt.Errorf("invalid JSON request object")
	}
	if request == nil {
		return nil, fmt.Errorf("JSON request must be an object")
	}
	return request, nil
}

func mustJSONRaw(value interface{}) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func executeLLMPost(c *cli.Context, appCtx *AppContext, route string, body []byte, streaming bool, operation string) error {
	response, err := appCtx.Client.LLMPost(c.Context, route, body)
	if err != nil {
		return llmCommandError(c.Context, err)
	}
	defer response.Body.Close()
	if err := validateLLMContentType(response.Header, streaming); err != nil {
		return err
	}
	if streaming {
		return consumeLLMStream(c, response.Body, operation)
	}
	if c.String("output") == "json" {
		_, err = io.Copy(os.Stdout, response.Body)
		return llmCommandError(c.Context, err)
	}
	switch operation {
	case "chat":
		return renderLLMChat(response.Body)
	case "responses":
		return renderLLMResponses(response.Body)
	case "embeddings":
		return renderLLMEmbeddings(appCtx, response.Body)
	default:
		return fmt.Errorf("unsupported LLM operation %q", operation)
	}
}

func validateLLMContentType(header http.Header, streaming bool) error {
	value := header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return fmt.Errorf("LLM gateway returned invalid Content-Type %q", value)
	}
	want := "application/json"
	if streaming {
		want = "text/event-stream"
	}
	if mediaType != want {
		return fmt.Errorf("LLM gateway returned Content-Type %q; expected %s", mediaType, want)
	}
	return nil
}

func llmCommandError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return cli.Exit("", 130)
	}
	return err
}

func consumeLLMStream(c *cli.Context, reader io.Reader, operation string) error {
	decoder := client.NewSSEDecoder(reader)
	jsonOutput := c.String("output") == "json"
	wroteText := false
	lineEnded := false
	finishTextLine := func() {
		if wroteText && !lineEnded {
			_, _ = fmt.Fprintln(os.Stdout)
			lineEnded = true
		}
	}
	defer finishTextLine()
	for {
		event, err := decoder.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return fmt.Errorf("LLM stream ended before a terminal event")
			}
			return llmCommandError(c.Context, err)
		}
		if operation == "chat" && bytes.Equal(bytes.TrimSpace(event.Data), []byte("[DONE]")) {
			finishTextLine()
			return nil
		}
		if len(bytes.TrimSpace(event.Data)) == 0 {
			continue
		}

		var payload map[string]json.RawMessage
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			return fmt.Errorf("invalid JSON in LLM SSE event %q", event.Name)
		}
		if _, failed := payload["error"]; event.Name == "error" || failed {
			if jsonOutput {
				_, _ = os.Stdout.Write(event.Data)
				_, _ = os.Stdout.Write([]byte("\n"))
			}
			return client.ParseLLMError(event.Data, 0, "")
		}
		if jsonOutput {
			if _, err := os.Stdout.Write(event.Data); err != nil {
				return err
			}
			if _, err := os.Stdout.Write([]byte("\n")); err != nil {
				return err
			}
		}

		if operation == "chat" {
			if !jsonOutput {
				if text := chatDeltaText(payload); text != "" {
					if _, err := io.WriteString(os.Stdout, text); err != nil {
						return err
					}
					wroteText = true
				}
			}
			continue
		}

		eventType := event.Name
		if raw, exists := payload["type"]; exists {
			_ = json.Unmarshal(raw, &eventType)
		}
		if !jsonOutput && eventType == "response.output_text.delta" {
			var delta string
			_ = json.Unmarshal(payload["delta"], &delta)
			if _, err := io.WriteString(os.Stdout, delta); err != nil {
				return err
			}
			wroteText = wroteText || delta != ""
		}
		switch eventType {
		case "response.completed":
			finishTextLine()
			return nil
		case "response.incomplete":
			finishTextLine()
			fmt.Fprintln(os.Stderr, "Warning: response completed without reaching status completed")
			return nil
		case "response.failed":
			return responseFailure(payload)
		}
	}
}

func chatDeltaText(payload map[string]json.RawMessage) string {
	var value struct {
		Choices []struct {
			Index int `json:"index"`
			Delta struct {
				Content json.RawMessage `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	data, _ := json.Marshal(payload)
	if json.Unmarshal(data, &value) != nil {
		return ""
	}
	for _, choice := range value.Choices {
		if choice.Index == 0 {
			return textFromContent(choice.Delta.Content)
		}
	}
	return ""
}

func responseFailure(payload map[string]json.RawMessage) error {
	var value struct {
		Response struct {
			Error *struct {
				Code      string `json:"code"`
				Type      string `json:"type"`
				Message   string `json:"message"`
				RequestID string `json:"request_id"`
			} `json:"error"`
		} `json:"response"`
	}
	data, _ := json.Marshal(payload)
	_ = json.Unmarshal(data, &value)
	if value.Response.Error == nil {
		return fmt.Errorf("Responses stream failed")
	}
	encoded, _ := json.Marshal(map[string]interface{}{"error": value.Response.Error})
	return client.ParseLLMError(encoded, 0, "")
}

func renderLLMModels(appCtx *AppContext, reader io.Reader) error {
	var response struct {
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			OwnedBy string `json:"owned_by"`
			Gateway struct {
				Operations   []string        `json:"operations"`
				Capabilities map[string]bool `json:"capabilities"`
			} `json:"llmgateway"`
		} `json:"data"`
	}
	if err := json.NewDecoder(reader).Decode(&response); err != nil {
		return fmt.Errorf("decode models response: %w", err)
	}
	rows := make([]map[string]interface{}, 0, len(response.Data))
	for _, model := range response.Data {
		caps := make([]string, 0, len(model.Gateway.Capabilities))
		for name, enabled := range model.Gateway.Capabilities {
			if enabled {
				caps = append(caps, name)
			}
		}
		sort.Strings(caps)
		rows = append(rows, map[string]interface{}{
			"id": model.ID, "owned_by": model.OwnedBy,
			"operations": strings.Join(model.Gateway.Operations, ","), "capabilities": strings.Join(caps, ","),
		})
	}
	return appCtx.Renderer.RenderArray(rows)
}

func renderLLMChat(reader io.Reader) error {
	var response struct {
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(reader).Decode(&response); err != nil {
		return fmt.Errorf("decode chat response: %w", err)
	}
	for _, choice := range response.Choices {
		if choice.Index == 0 {
			if text := textFromContent(choice.Message.Content); text != "" {
				_, err := fmt.Fprintln(os.Stdout, text)
				return err
			}
			break
		}
	}
	fmt.Fprintln(os.Stderr, "Notice: chat response contains no primary text; use --output json to inspect the canonical response")
	return nil
}

func textFromContent(raw json.RawMessage) string {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ""
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	var builder strings.Builder
	for _, part := range parts {
		if part.Type == "text" || part.Type == "output_text" {
			builder.WriteString(part.Text)
		}
	}
	return builder.String()
}

func renderLLMResponses(reader io.Reader) error {
	var response struct {
		Status string `json:"status"`
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.NewDecoder(reader).Decode(&response); err != nil {
		return fmt.Errorf("decode Responses result: %w", err)
	}
	var text strings.Builder
	for _, item := range response.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type == "output_text" {
				text.WriteString(part.Text)
			}
		}
	}
	if text.Len() > 0 {
		fmt.Fprintln(os.Stdout, text.String())
	} else {
		fmt.Fprintln(os.Stderr, "Notice: Responses result contains no primary output text; use --output json to inspect the canonical response")
	}
	if response.Status == "incomplete" {
		fmt.Fprintln(os.Stderr, "Warning: response completed without reaching status completed")
	}
	return nil
}

func renderLLMEmbeddings(appCtx *AppContext, reader io.Reader) error {
	var response struct {
		Model string `json:"model"`
		Data  []struct {
			Index     int               `json:"index"`
			Embedding []json.RawMessage `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(reader).Decode(&response); err != nil {
		return fmt.Errorf("decode embeddings response: %w", err)
	}
	rows := make([]map[string]interface{}, 0, len(response.Data))
	for _, item := range response.Data {
		rows = append(rows, map[string]interface{}{"index": item.Index, "dimensions": len(item.Embedding), "model": response.Model})
	}
	return appCtx.Renderer.RenderArray(rows)
}
