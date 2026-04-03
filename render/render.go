package render

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
)

type Renderer interface {
	RenderArray(array []map[string]interface{}) error
	RenderObject(object map[string]interface{}) error
}

// FilterColumns keeps only the named columns in each row.
func FilterColumns(array []map[string]interface{}, columns []string) []map[string]interface{} {
	for i, row := range array {
		array[i] = IncludeColumns(row, columns)
	}
	return array
}

// IncludeColumns returns a new map with only the named keys.
func IncludeColumns(row map[string]interface{}, columns []string) map[string]interface{} {
	result := make(map[string]interface{}, len(columns))
	for _, col := range columns {
		if val, ok := row[col]; ok {
			result[col] = val
		}
	}
	return result
}

// ExcludeColumns returns a new map without the named keys.
func ExcludeColumns(row map[string]interface{}, columns []string) map[string]interface{} {
	exclude := make(map[string]bool, len(columns))
	for _, col := range columns {
		exclude[col] = true
	}
	result := make(map[string]interface{}, len(row))
	for k, v := range row {
		if !exclude[k] {
			result[k] = v
		}
	}
	return result
}

// TableRenderer outputs data as an aligned text table.
type TableRenderer struct {
	MaxWidth int // 0 means no truncation
}

func NewTableRenderer() *TableRenderer {
	return &TableRenderer{MaxWidth: 50}
}

func NewTableRendererNoTruncate() *TableRenderer {
	return &TableRenderer{MaxWidth: 0}
}

func (t *TableRenderer) truncate(s string) string {
	if t.MaxWidth > 0 && len(s) > t.MaxWidth {
		return s[:t.MaxWidth] + "..."
	}
	return s
}

func (t *TableRenderer) RenderObject(data map[string]interface{}) error {
	for header, val := range data {
		fmt.Fprintf(os.Stdout, "[%s]: %s\n", header, t.truncate(fmt.Sprintf("%v", val)))
	}
	return nil
}

func (t *TableRenderer) RenderArray(data []map[string]interface{}) error {
	if len(data) == 0 {
		fmt.Println("No data to print")
		return nil
	}

	headers := make([]string, 0, len(data[0]))
	for header := range data[0] {
		headers = append(headers, header)
	}
	sort.Strings(headers)

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)

	for _, header := range headers {
		fmt.Fprintf(tw, "%s\t", header)
	}
	fmt.Fprintln(tw)

	for _, row := range data {
		for _, header := range headers {
			fmt.Fprintf(tw, "%s\t", t.truncate(fmt.Sprintf("%v", row[header])))
		}
		fmt.Fprintln(tw)
	}

	return tw.Flush()
}

// JsonRenderer outputs data as indented JSON.
type JsonRenderer struct{}

func NewJsonRenderer() *JsonRenderer {
	return &JsonRenderer{}
}

func (j *JsonRenderer) RenderObject(data map[string]interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonData))
	return nil
}

func (j *JsonRenderer) RenderArray(data []map[string]interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonData))
	return nil
}
