package condition

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/zlovtnik/ggt/pkg/event"
)

// Condition represents an evaluable predicate against an event payload.
type Condition interface {
	Evaluate(e event.Event) (bool, error)
}

// Parse converts the expression string into a Condition implementation.
// Supported expressions mirror the filter.condition transform syntax.
func Parse(expr string) (Condition, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("condition expression cannot be empty")
	}

	// Handle OR (lowest precedence)
	if parts := splitOutsideParens(expr, " OR "); len(parts) > 1 {
		conditions := make([]Condition, len(parts))
		for i, part := range parts {
			cond, err := Parse(part)
			if err != nil {
				return nil, err
			}
			conditions[i] = cond
		}
		return &OrCondition{Conditions: conditions}, nil
	}

	// Handle AND (higher precedence than OR)
	if parts := splitOutsideParens(expr, " AND "); len(parts) > 1 {
		conditions := make([]Condition, len(parts))
		for i, part := range parts {
			cond, err := Parse(part)
			if err != nil {
				return nil, err
			}
			conditions[i] = cond
		}
		return &AndCondition{Conditions: conditions}, nil
	}

	return parseUnary(expr)
}

func parseUnary(expr string) (Condition, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("condition expression cannot be empty")
	}

	if strings.HasPrefix(expr, "NOT ") {
		operand := strings.TrimSpace(expr[len("NOT "):])
		if operand == "" {
			return nil, fmt.Errorf("NOT operator requires an operand")
		}
		cond, err := parseUnary(operand)
		if err != nil {
			return nil, err
		}
		return &NotCondition{Cond: cond}, nil
	}

	stripped := stripOuterParens(expr)
	if stripped != expr {
		return Parse(stripped)
	}

	return parseComparisonCondition(expr)
}

// AndCondition evaluates to true when all nested conditions are true.
type AndCondition struct {
	Conditions []Condition
}

func (a *AndCondition) Evaluate(e event.Event) (bool, error) {
	for _, cond := range a.Conditions {
		match, err := cond.Evaluate(e)
		if err != nil {
			return false, err
		}
		if !match {
			return false, nil
		}
	}
	return true, nil
}

// OrCondition evaluates to true when any nested condition is true.
type OrCondition struct {
	Conditions []Condition
}

func (o *OrCondition) Evaluate(e event.Event) (bool, error) {
	for _, cond := range o.Conditions {
		match, err := cond.Evaluate(e)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

// NotCondition negates the result of an inner condition.
type NotCondition struct {
	Cond Condition
}

func (n *NotCondition) Evaluate(e event.Event) (bool, error) {
	if n.Cond == nil {
		return false, fmt.Errorf("not condition missing operand")
	}
	match, err := n.Cond.Evaluate(e)
	if err != nil {
		return false, err
	}
	return !match, nil
}

// ComparisonCondition compares a field value to a target value.
type ComparisonCondition struct {
	Field    string
	Operator string
	Value    string
	regex    *regexp.Regexp
}

func (c *ComparisonCondition) Evaluate(e event.Event) (bool, error) {
	if c.Field == "" {
		return false, fmt.Errorf("comparison missing field")
	}
	fieldValue, exists := e.GetField(c.Field)
	if !exists {
		switch c.Operator {
		case "ne", "notmatch":
			return true, nil
		default:
			return false, nil
		}
	}

	fieldStr := fmt.Sprintf("%v", fieldValue)

	switch c.Operator {
	case "eq":
		return fieldStr == c.Value, nil
	case "ne":
		return fieldStr != c.Value, nil
	case "lt":
		return compareNumbers(fieldStr, c.Value, func(a, b float64) bool { return a < b })
	case "lte":
		return compareNumbers(fieldStr, c.Value, func(a, b float64) bool { return a <= b })
	case "gt":
		return compareNumbers(fieldStr, c.Value, func(a, b float64) bool { return a > b })
	case "gte":
		return compareNumbers(fieldStr, c.Value, func(a, b float64) bool { return a >= b })
	case "match":
		if c.regex == nil {
			return false, fmt.Errorf("regex not compiled for match operator")
		}
		return c.regex.MatchString(fieldStr), nil
	case "notmatch":
		if c.regex == nil {
			return false, fmt.Errorf("regex not compiled for notmatch operator")
		}
		return !c.regex.MatchString(fieldStr), nil
	case "contains":
		return strings.Contains(fieldStr, c.Value), nil
	case "startswith":
		return strings.HasPrefix(fieldStr, c.Value), nil
	case "endswith":
		return strings.HasSuffix(fieldStr, c.Value), nil
	case "in":
		values := strings.Split(c.Value, ",")
		for _, v := range values {
			candidate := strings.TrimSpace(v)
			if candidate == "" {
				continue
			}
			candidate = strings.ReplaceAll(candidate, `\"`, `"`)
			candidate = strings.ReplaceAll(candidate, `\'`, `'`)
			candidate = unquoteValue(candidate)
			if candidate == fieldStr {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", c.Operator)
	}
}

func parseComparisonCondition(expr string) (Condition, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, fmt.Errorf("condition expression cannot be empty")
	}

	operators := []struct {
		op     string
		opType string
	}{
		{"!~", "notmatch"},
		{"=~", "match"},
		{"!=", "ne"},
		{"<=", "lte"},
		{">=", "gte"},
		{"<", "lt"},
		{">", "gt"},
		{"==", "eq"},
		{"=", "eq"},
	}

	for _, o := range operators {
		idx := findOperatorOutsideQuotes(expr, o.op)
		if idx == -1 {
			continue
		}
		needleLen := len(o.op) + 2 // account for surrounding spaces
		field := strings.TrimSpace(expr[:idx])
		valueStr := strings.TrimSpace(expr[idx+needleLen:])
		if field == "" || valueStr == "" {
			return nil, fmt.Errorf("invalid condition expression: %s", expr)
		}
		unquoted := unquoteValue(valueStr)
		cc := &ComparisonCondition{Field: field, Operator: o.opType, Value: unquoted}
		if o.opType == "match" || o.opType == "notmatch" {
			re, err := regexp.Compile(unquoted)
			if err != nil {
				return nil, fmt.Errorf("invalid regex pattern: %w", err)
			}
			cc.regex = re
		}
		return cc, nil
	}

	textOperators := []struct {
		op     string
		opType string
	}{
		{"contains", "contains"},
		{"startswith", "startswith"},
		{"endswith", "endswith"},
		{"in", "in"},
	}

	for _, o := range textOperators {
		idx := findOperatorOutsideQuotes(expr, o.op)
		if idx == -1 {
			continue
		}
		needleLen := len(o.op) + 2
		field := strings.TrimSpace(expr[:idx])
		valueStr := strings.TrimSpace(expr[idx+needleLen:])
		if field == "" || valueStr == "" {
			return nil, fmt.Errorf("invalid condition expression: %s", expr)
		}
		unquoted := unquoteValue(valueStr)
		return &ComparisonCondition{Field: field, Operator: o.opType, Value: unquoted}, nil
	}

	return nil, fmt.Errorf("invalid condition expression: %s", expr)
}

func unquoteValue(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func compareNumbers(a, b string, cmp func(float64, float64) bool) (bool, error) {
	numA, errA := strconv.ParseFloat(a, 64)
	numB, errB := strconv.ParseFloat(b, 64)
	if errA != nil || errB != nil {
		return false, fmt.Errorf("non-numeric operands for comparison: %s, %s", a, b)
	}
	return cmp(numA, numB), nil
}

func stripOuterParens(expr string) string {
	expr = strings.TrimSpace(expr)
	if len(expr) < 2 {
		return expr
	}
	if expr[0] != '(' || expr[len(expr)-1] != ')' {
		return expr
	}

	depth := 0
	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && i != len(expr)-1 {
				return expr
			}
		}
	}

	if depth == 0 {
		return expr[1 : len(expr)-1]
	}
	return expr
}

func splitOutsideParens(expr string, sep string) []string {
	depth := 0
	start := 0
	var parts []string

	i := 0
	for i <= len(expr)-len(sep) {
		switch expr[i] {
		case '(':
			depth++
			i++
		case ')':
			if depth > 0 {
				depth--
			}
			i++
		default:
			if depth == 0 && strings.HasPrefix(expr[i:], sep) {
				part := strings.TrimSpace(expr[start:i])
				if part != "" {
					parts = append(parts, part)
				}
				i += len(sep)
				start = i
			} else {
				i++
			}
		}
	}

	tail := strings.TrimSpace(expr[start:])
	if tail != "" {
		parts = append(parts, tail)
	}

	return parts
}

func findOperatorOutsideQuotes(expr, op string) int {
	if op == "" {
		return -1
	}
	needle := " " + op + " "
	if len(expr) < len(needle) {
		return -1
	}

	inQuote := false
	var quoteChar byte
	for i := 0; i <= len(expr)-len(needle); i++ {
		ch := expr[i]
		if (ch == '"' || ch == '\'') && !isEscaped(expr, i) {
			if inQuote && ch == quoteChar {
				inQuote = false
			} else if !inQuote {
				inQuote = true
				quoteChar = ch
			}
		}
		if !inQuote && strings.HasPrefix(expr[i:], needle) {
			return i
		}
	}

	return -1
}

func isEscaped(expr string, idx int) bool {
	count := 0
	for i := idx - 1; i >= 0; i-- {
		if expr[i] != '\\' {
			break
		}
		count++
	}
	return count%2 == 1
}
