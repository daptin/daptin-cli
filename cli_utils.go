package main

import (
	"bufio"
	"fmt"
	"golang.org/x/term"
	"os"
	"strings"
	"syscall"
	"text/tabwriter"
)

func PrintTable(data []map[string]interface{}) {
	if len(data) == 0 {
		fmt.Println("No data to print")
		return
	}

	// Extract headers
	headers := make([]string, 0, len(data[0]))
	for header := range data[0] {
		headers = append(headers, header)
	}

	// Create a tabwriter
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.AlignRight|tabwriter.Debug)

	// Print headers
	for _, header := range headers {
		_, _ = fmt.Fprintf(tw, "%s\t", header)
	}
	_, _ = fmt.Fprintln(tw)

	// Print data
	for _, row := range data {
		for _, header := range headers {
			val := row[header]
			valString := fmt.Sprintf("%v", val)
			if len(valString) > 50 {
				valString = valString[:50] + "..."
			}
			_, _ = fmt.Fprintf(tw, "%s\t", valString)
		}
		_, _ = fmt.Fprintln(tw)
	}

	// Flush the tabwriter
	_ = tw.Flush()
}

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
