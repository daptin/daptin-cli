package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FilterClause represents a single filter condition.
// Pure value type.
type FilterClause struct {
	Column   string `json:"column"`
	Operator string `json:"operator"`
	Value    string `json:"value,omitempty"`
}

// Two-word operators that Daptin supports (order matters: longer matches first).
var twoWordOperators = []string{
	"begins with",
	"ends with",
	"more than",
	"less than",
	"is not",
	"is true",
	"is false",
	"is empty",
}

// Single-word operators.
var singleWordOperators = []string{
	"is", "eq", "contains", "like", "ilike", "neq", "gt", "lt",
	"after", "before", "in", "fuzzy",
}

// No-value operators (the value is implied by the operator itself).
var noValueOperators = map[string]bool{
	"is true":  true,
	"is false": true,
	"is empty": true,
}

// ParseFilter parses a human-readable filter expression or raw JSON into FilterClauses.
// Supports semicolon-separated multiple filters.
// Pure function.
func ParseFilter(input string) ([]FilterClause, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	// If it looks like JSON, parse directly
	if strings.HasPrefix(input, "[") {
		var clauses []FilterClause
		if err := json.Unmarshal([]byte(input), &clauses); err != nil {
			return nil, fmt.Errorf("invalid filter JSON: %w", err)
		}
		return clauses, nil
	}

	// Split on semicolons for multiple filters
	parts := strings.Split(input, ";")
	clauses := make([]FilterClause, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		clause, err := parseSingleFilter(part)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, clause)
	}
	return clauses, nil
}

func parseSingleFilter(expr string) (FilterClause, error) {
	if clause, ok := parseEqualsFilter(expr); ok {
		return clause, nil
	}

	// Format: <column> <operator> [value]
	// Try two-word operators first
	for _, op := range twoWordOperators {
		idx := strings.Index(expr, " "+op)
		if idx < 0 {
			continue
		}
		column := expr[:idx]
		rest := strings.TrimSpace(expr[idx+1+len(op):])

		if noValueOperators[op] {
			return FilterClause{Column: column, Operator: op}, nil
		}
		return FilterClause{Column: column, Operator: op, Value: rest}, nil
	}

	// Try single-word operators
	words := strings.Fields(expr)
	if len(words) < 2 {
		return FilterClause{}, invalidFilterExpression(expr)
	}

	column := words[0]
	operator := words[1]

	// Validate operator
	valid := false
	for _, op := range singleWordOperators {
		if operator == op {
			valid = true
			break
		}
	}
	if !valid {
		return FilterClause{}, fmt.Errorf("unknown filter operator %q in expression %q. Example: --filter 'name is users'", operator, expr)
	}

	value := ""
	if len(words) > 2 {
		value = strings.Join(words[2:], " ")
	}
	return FilterClause{Column: column, Operator: operator, Value: value}, nil
}

func parseEqualsFilter(expr string) (FilterClause, bool) {
	operator := ""
	idx := -1
	if i := strings.Index(expr, "=="); i >= 0 {
		operator = "=="
		idx = i
	} else if i := strings.Index(expr, "="); i >= 0 {
		operator = "="
		idx = i
	}
	if idx < 0 {
		return FilterClause{}, false
	}

	column := strings.TrimSpace(expr[:idx])
	value := strings.TrimSpace(expr[idx+len(operator):])
	if column == "" || value == "" {
		return FilterClause{}, false
	}
	return FilterClause{Column: column, Operator: "is", Value: value}, true
}

func invalidFilterExpression(expr string) error {
	return fmt.Errorf("invalid filter expression: %q. Expected: <column> <operator> [value]. Example: --filter 'name is users'", expr)
}

// FilterToJSON serializes filter clauses to the JSON format Daptin expects.
// Pure function.
func FilterToJSON(clauses []FilterClause) string {
	data, _ := json.Marshal(clauses)
	return string(data)
}
