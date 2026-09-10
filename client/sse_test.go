package client

import (
	"io"
	"strings"
	"testing"
)

func TestSSEDecoderHandlesCommentsCRLFAndMultilineData(t *testing.T) {
	input := ": keep-alive\r\nevent: response.output_text.delta\r\ndata: {\"first\":1,\r\ndata: \"second\":2}\r\n\r\n"
	event, err := NewSSEDecoder(strings.NewReader(input)).Next()
	if err != nil {
		t.Fatal(err)
	}
	if event.Name != "response.output_text.delta" || string(event.Data) != "{\"first\":1,\n\"second\":2}" {
		t.Fatalf("event = %#v", event)
	}
}

func TestSSEDecoderReturnsFinalUnterminatedEvent(t *testing.T) {
	decoder := NewSSEDecoder(strings.NewReader("data: {\"ok\":true}"))
	event, err := decoder.Next()
	if err != nil || string(event.Data) != `{"ok":true}` {
		t.Fatalf("event=%#v err=%v", event, err)
	}
	if _, err := decoder.Next(); err != io.EOF {
		t.Fatalf("final error = %v", err)
	}
}

func TestSSEDecoderRejectsOversizedEvent(t *testing.T) {
	_, err := NewSSEDecoder(strings.NewReader("data: " + strings.Repeat("x", MaxLLMSSEEventBytes+1) + "\n\n")).Next()
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v", err)
	}
}
