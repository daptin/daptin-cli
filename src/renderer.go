package src

import (
	"bufio"
	"encoding/json"
	"fmt"
	"golang.org/x/term"
	"os"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
)

func readStringsAsArgs(countOfaArgs int) ([]string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter Username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	fmt.Print("Enter Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, err
	}

	password := string(bytePassword)
	return []string{strings.TrimSpace(username), strings.TrimSpace(password)}, nil
}

type Renderer interface {
	RenderArray(array []map[string]interface{}) error
	RenderObject(object map[string]interface{}) error
}

type TableRenderer struct {
}

func NewTableRenderer() *TableRenderer {
	return &TableRenderer{}
}

func (t *TableRenderer) RenderObject(data map[string]interface{}) error {
	var err error
	for header, val := range data {
		valString := fmt.Sprintf("%v", val)
		if len(valString) > 50 {
			valString = valString[:50] + "..."
		}
		_, err = fmt.Fprintf(os.Stdout, "[%s]: %s\n", header, valString)
	}
	return err
}

func (t *TableRenderer) RenderArray(data []map[string]interface{}) error {
	if len(data) == 0 {
		fmt.Println("No data to print")
		return nil
	}

	// Extract headers
	headers := make([]string, 0, len(data[0]))
	for header := range data[0] {
		headers = append(headers, header)
	}

	sort.Strings(headers)

	// Create a tabwriter
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
	var err error

	// Print headers
	for _, header := range headers {
		_, err = fmt.Fprintf(tw, "%s\t", header)
	}
	_, err = fmt.Fprintln(tw)

	// Print data
	for _, row := range data {
		for _, header := range headers {
			val := row[header]
			valString := fmt.Sprintf("%v", val)
			if len(valString) > 50 {
				valString = valString[:50] + "..."
			}
			_, err = fmt.Fprintf(tw, "%s\t", valString)
		}
		_, err = fmt.Fprintln(tw)
	}

	// Flush the tabwriter
	err = tw.Flush()
	return err
}

type JsonRenderer struct {
}

func NewJsonRenderer() *JsonRenderer {
	return &JsonRenderer{}
}

func (j JsonRenderer) RenderObject(data map[string]interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonData))
	return nil
}

func (j JsonRenderer) RenderArray(data []map[string]interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonData))
	return nil
}
