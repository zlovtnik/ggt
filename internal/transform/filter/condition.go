package filter

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// ConditionConfig represents the configuration for filter.condition transform
type ConditionConfig struct {
	Expression string `json:"expression"`
}

// conditionTransform implements filter.condition
type conditionTransform struct {
	cfg       ConditionConfig
	condition Condition
}

func (c *conditionTransform) Name() string { return "filter.condition" }

func (c *conditionTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("condition expression required")
	}
	if err := json.Unmarshal(raw, &c.cfg); err != nil {
		return err
	}
	if c.cfg.Expression == "" {
		return fmt.Errorf("expression cannot be empty")
	}
	// Parse and validate the condition expression
	cond, err := ParseCondition(c.cfg.Expression)
	if err != nil {
		return fmt.Errorf("invalid condition expression: %w", err)
	}
	c.condition = cond
	return nil
}

func (c *conditionTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	match, err := c.condition.Evaluate(ev)
	if err != nil {
		return nil, fmt.Errorf("condition evaluation failed: %w", err)
	}

	if !match {
		return nil, transform.ErrDrop
	}

	return ev, nil
}

// NewConditionTransform creates a new condition filter transform
func NewConditionTransform() transform.Transform {
	return &conditionTransform{}
}

// Condition represents a filter condition that can be evaluated against an Event
type Condition interface {
	Evaluate(e event.Event) (bool, error)
}

// ParseCondition parses a condition expression string into a Condition
// Supports expressions like:
//   - field == "value"
//   - field != "value"
//   - field > 10
//   - field < 10
//   - field >= 10
//   - field <= 10
//   - field =~ "regex"
//   - field !~ "regex"
//   - field contains "substring"
//   - field startswith "prefix"
//   - field endswith "suffix"
//   - (condition1) AND (condition2)
//   - (condition1) OR (condition2)
//   - NOT (condition)
func ParseCondition(expr string) (Condition, error) {
	expr = strings.TrimSpace(expr)

	// Handle NOT operator (only consumes the immediately following term)
	if strings.HasPrefix(expr, "NOT ") {
		rest := strings.TrimPrefix(expr, "NOT ")
		rest = strings.TrimSpace(rest)

		// Find the next top-level logical operator
		termEndIdx := findNextLogicalOp(rest)
		var innerExpr string

		if termEndIdx == -1 {
			// No more operators, use entire rest
			innerExpr = rest
		} else {
			innerExpr = strings.TrimSpace(rest[:termEndIdx])
		}

		// Parse the inner expression
		inner, err := ParseCondition(innerExpr)
		if err != nil {
			return nil, err
		}

		return &NotCondition{Cond: inner}, nil
	}

	// Strip outer parentheses if present
	expr = stripOuterParens(expr)

	// Handle parenthesized expressions for logical operators
	if strings.Contains(expr, " AND ") || strings.Contains(expr, " OR ") {
		return parseLogicalCondition(expr)
	}

	// Parse comparison condition (field operator value)
	return parseComparisonCondition(expr)
}

// findNextLogicalOp finds the index of the next AND/OR operator at depth 0
// Returns -1 if none found
func findNextLogicalOp(expr string) int {
	depth := 0
	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '(':
			depth++
		case ')':
			depth--
		default:
			if depth == 0 {
				// Check for AND operator
				if i+5 <= len(expr) && expr[i:i+5] == " AND " {
					return i
				}
				// Check for OR operator
				if i+4 <= len(expr) && expr[i:i+4] == " OR " {
					return i
				}
			}
		}
	}
	return -1
}

func parseLogicalCondition(expr string) (Condition, error) {
	// Split on OR first (lower precedence)
	parts := splitOutsideParens(expr, " OR ")
	if len(parts) > 1 {
		conditions := make([]Condition, len(parts))
		for i, part := range parts {
			part = strings.TrimSpace(part)
			// Strip outer parentheses if present
			part = stripOuterParens(part)
			cond, err := ParseCondition(part)
			if err != nil {
				return nil, err
			}
			conditions[i] = cond
		}
		return &OrCondition{Conditions: conditions}, nil
	}

	// Split on AND (higher precedence)
	parts = splitOutsideParens(expr, " AND ")
	if len(parts) > 1 {
		conditions := make([]Condition, len(parts))
		for i, part := range parts {
			part = strings.TrimSpace(part)
			// Strip outer parentheses if present
			part = stripOuterParens(part)
			cond, err := ParseCondition(part)
			if err != nil {
				return nil, err
			}
			conditions[i] = cond
		}
		return &AndCondition{Conditions: conditions}, nil
	}

	return nil, fmt.Errorf("invalid logical expression: %s", expr)
}

// stripOuterParens removes surrounding parentheses if they enclose the entire expression
func stripOuterParens(expr string) string {
	expr = strings.TrimSpace(expr)
	if len(expr) < 2 {
		return expr
	}
	if expr[0] != '(' || expr[len(expr)-1] != ')' {
		return expr
	}

	// Check if the opening paren is matched by the closing paren
	depth := 0
	for i := 0; i < len(expr)-1; i++ {
		if expr[i] == '(' {
			depth++
		} else if expr[i] == ')' {
			depth--
		}
		if depth == 0 {
			// Closing paren is not at the end, so outer parens don't enclose everything
			return expr
		}
	}

	// The first opening paren is matched by the last closing paren
	return expr[1 : len(expr)-1]
}

func splitOutsideParens(expr string, sep string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	i := 0
	for i < len(expr) {
		if expr[i] == '(' {
			depth++
			current.WriteByte(expr[i])
			i++
		} else if expr[i] == ')' {
			depth--
			current.WriteByte(expr[i])
			i++
		} else if depth == 0 && strings.HasPrefix(expr[i:], sep) {
			parts = append(parts, current.String())
			current.Reset()
			i += len(sep)
		} else {
			current.WriteByte(expr[i])
			i++
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

func unquoteValue(s string) string {
	// Remove surrounding quotes if present
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

func parseComparisonCondition(expr string) (Condition, error) {
	// Try each operator in order of length (longest first to avoid ambiguity)
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
		if idx := strings.Index(expr, " "+o.op+" "); idx != -1 {
			field := strings.TrimSpace(expr[:idx])
			valueStr := strings.TrimSpace(expr[idx+len(" "+o.op+" "):])
			// Unquote values for string comparisons
			unquoted := unquoteValue(valueStr)

			cc := &ComparisonCondition{
				Field:    field,
				Operator: o.opType,
				Value:    unquoted,
			}

			// Compile regex for match/notmatch operators
			if o.opType == "match" || o.opType == "notmatch" {
				re, err := regexp.Compile(unquoted)
				if err != nil {
					return nil, fmt.Errorf("invalid regex pattern: %w", err)
				}
				cc.regex = re
			}

			return cc, nil
		}
	}

	// Check for "in", "contains", "startswith", "endswith" operators
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
		if idx := strings.Index(expr, " "+o.op+" "); idx != -1 {
			field := strings.TrimSpace(expr[:idx])
			valueStr := strings.TrimSpace(expr[idx+len(" "+o.op+" "):])
			unquoted := unquoteValue(valueStr)
			return &ComparisonCondition{
				Field:    field,
				Operator: o.opType,
				Value:    unquoted,
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid condition expression: %s", expr)
}

// ComparisonCondition compares a field value to a target value
type ComparisonCondition struct {
	Field    string         // field path (e.g., "user.id")
	Operator string         // eq, ne, lt, lte, gt, gte, match, notmatch, contains, startswith, endswith, in
	Value    string         // string representation of target value
	regex    *regexp.Regexp // compiled regex for match/notmatch operators (nil for others)
}

func (c *ComparisonCondition) Evaluate(e event.Event) (bool, error) {
	fieldValue, exists := e.GetField(c.Field)
	if !exists {
		// Field doesn't exist: only true for "!=" or "!~"
		return c.Operator == "ne" || c.Operator == "notmatch", nil
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
		// Split comma-separated values
		values := strings.Split(c.Value, ",")
		for _, v := range values {
			if strings.TrimSpace(v) == fieldStr {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", c.Operator)
	}
}

func compareNumbers(a, b string, cmp func(float64, float64) bool) (bool, error) {
	numA, errA := strconv.ParseFloat(a, 64)
	numB, errB := strconv.ParseFloat(b, 64)
	if errA != nil || errB != nil {
		return false, fmt.Errorf("non-numeric operands for comparison: %s, %s", a, b)
	}
	return cmp(numA, numB), nil
}

// AndCondition evaluates to true if all conditions are true
type AndCondition struct {
	Conditions []Condition
}

func (a *AndCondition) Evaluate(e event.Event) (bool, error) {
	for _, c := range a.Conditions {
		match, err := c.Evaluate(e)
		if err != nil {
			return false, err
		}
		if !match {
			return false, nil
		}
	}
	return true, nil
}

// OrCondition evaluates to true if any condition is true
type OrCondition struct {
	Conditions []Condition
}

func (o *OrCondition) Evaluate(e event.Event) (bool, error) {
	for _, c := range o.Conditions {
		match, err := c.Evaluate(e)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

// NotCondition negates a condition
type NotCondition struct {
	Cond Condition
}

func (n *NotCondition) Evaluate(e event.Event) (bool, error) {
	match, err := n.Cond.Evaluate(e)
	if err != nil {
		return false, err
	}
	return !match, nil
}
