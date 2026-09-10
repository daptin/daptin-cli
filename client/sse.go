package client

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

const MaxLLMSSEEventBytes = 1 << 20

type SSEEvent struct {
	Name string
	Data []byte
}

type SSEDecoder struct {
	reader  *bufio.Reader
	maximum int
}

func NewSSEDecoder(reader io.Reader) *SSEDecoder {
	return &SSEDecoder{reader: bufio.NewReaderSize(reader, 64<<10), maximum: MaxLLMSSEEventBytes}
}

func (d *SSEDecoder) Next() (SSEEvent, error) {
	var name string
	var data bytes.Buffer
	seenField := false

	for {
		line, err := d.readLine()
		if err != nil {
			if err == io.EOF && seenField {
				return buildSSEEvent(name, &data), nil
			}
			return SSEEvent{}, err
		}
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		if line == "" {
			if seenField {
				return buildSSEEvent(name, &data), nil
			}
			if err == io.EOF {
				return SSEEvent{}, io.EOF
			}
			continue
		}
		if strings.HasPrefix(line, ":") {
			if err == io.EOF {
				return SSEEvent{}, io.EOF
			}
			continue
		}

		field, value, found := strings.Cut(line, ":")
		if !found {
			field, value = line, ""
		} else if strings.HasPrefix(value, " ") {
			value = value[1:]
		}
		switch field {
		case "event":
			name = value
			seenField = true
		case "data":
			if data.Len()+len(value)+1 > d.maximum {
				return SSEEvent{}, fmt.Errorf("SSE event exceeds %d-byte limit", d.maximum)
			}
			data.WriteString(value)
			data.WriteByte('\n')
			seenField = true
		}
	}
}

func buildSSEEvent(name string, data *bytes.Buffer) SSEEvent {
	payload := append([]byte(nil), data.Bytes()...)
	if len(payload) > 0 {
		payload = payload[:len(payload)-1]
	}
	return SSEEvent{Name: name, Data: payload}
}

func (d *SSEDecoder) readLine() (string, error) {
	var line bytes.Buffer
	for {
		fragment, prefix, err := d.reader.ReadLine()
		if err != nil {
			return line.String(), err
		}
		if line.Len()+len(fragment)+1 > d.maximum {
			return "", fmt.Errorf("SSE event exceeds %d-byte limit", d.maximum)
		}
		line.Write(fragment)
		if !prefix {
			line.WriteByte('\n')
			return line.String(), nil
		}
	}
}
