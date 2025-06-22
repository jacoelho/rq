package jsonpath

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	litNum litKind = iota + 1
	litStr
	litRegex
	litArray
)

var filterRe = regexp.MustCompile(`^@((?:\.[-\w]+)+)?\s*(==|!=|<=|>=|<|>|=~|!~|in|nin)?\s*(.*)$`)

// selector defines the interface for JSONPath selectors that can match path elements.
// val is the scalar value of pe, or nil if pe is a container.
type selector interface {
	match(pe pathElem, val any) bool
}

type litKind uint8

type segment struct {
	deep bool       // true for '..' descendant operator
	sels []selector // list of selectors for this segment (e.g. name, index, filter)
}

type (
	nameSel     string
	wildcardSel struct{}
	indexSel    int
	sliceSel    struct{ start, end, step int }
)

type filterSel struct {
	path   []string
	cmp    comparison
	exists bool // true for existence check like [?(@.isbn)]
}

type comparison struct {
	op    string
	num   float64
	str   string
	regex *regexp.Regexp
	arr   []any
	kind  litKind
}

func (n nameSel) match(pe pathElem, _ any) bool {
	return !pe.isArray && pe.name == string(n)
}

func (wildcardSel) match(_ pathElem, _ any) bool {
	return true
}

func (i indexSel) match(pe pathElem, _ any) bool {
	return pe.isArray && pe.index == int(i)
}

func (s sliceSel) match(pe pathElem, _ any) bool {
	if !pe.isArray {
		return false
	}

	step := s.step
	if step == 0 {
		step = 1
	}

	if step > 0 {
		if pe.index < s.start || pe.index >= s.end {
			return false
		}
		return (pe.index-s.start)%step == 0
	} else if step < 0 {
		// Descending slice, e.g. [5:1:-1]
		return pe.index >= s.start && pe.index < s.end && (pe.index-s.start)%s.step == 0
	}
	return false
}

func (f filterSel) match(_ pathElem, val any) bool {
	target := f.extractTarget(val)

	if f.exists {
		return target != nil
	}

	// Filters operate on the value of the current node (@).
	// If @ is a container, its 'val' on the vals stack is nil.
	if target == nil {
		return false
	}

	return f.evaluateComparison(target)
}

func (f filterSel) extractTarget(val any) any {
	m, ok := val.(map[string]any)
	if !ok {
		return val
	}

	current := any(m)
	for _, p := range f.path {
		child, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = child[p]
	}
	return current
}

func (f filterSel) evaluateComparison(target any) bool {
	switch f.cmp.kind {
	case litNum:
		return f.evaluateNumericComparison(target)
	case litStr:
		return f.evaluateStringComparison(target)
	case litRegex:
		return f.evaluateRegexComparison(target)
	case litArray:
		return f.evaluateArrayComparison(target)
	}
	return false
}

func (f filterSel) evaluateNumericComparison(target any) bool {
	num, ok := target.(json.Number)
	if !ok {
		return false
	}
	v, err := num.Float64()
	if err != nil {
		return false
	}

	switch f.cmp.op {
	case "==":
		return v == f.cmp.num
	case "!=":
		return v != f.cmp.num
	case "<":
		return v < f.cmp.num
	case "<=":
		return v <= f.cmp.num
	case ">":
		return v > f.cmp.num
	case ">=":
		return v >= f.cmp.num
	}
	return false
}

func (f filterSel) evaluateStringComparison(target any) bool {
	s, ok := target.(string)
	if !ok {
		return false
	}

	switch f.cmp.op {
	case "==":
		return s == f.cmp.str
	case "!=":
		return s != f.cmp.str
	}
	return false
}

func (f filterSel) evaluateRegexComparison(target any) bool {
	s, ok := target.(string)
	if !ok {
		return false
	}

	m := f.cmp.regex.MatchString(s)
	switch f.cmp.op {
	case "=~":
		return m
	case "!~":
		return !m
	}
	return false
}

func (f filterSel) evaluateArrayComparison(target any) bool {
	for _, item := range f.cmp.arr {
		if compareValues(target, item) {
			switch f.cmp.op {
			case "in":
				return true
			case "nin":
				return false
			}
		}
	}

	switch f.cmp.op {
	case "in":
		return false // not found in array
	case "nin":
		return true // not found in array (which is what we want for nin)
	}
	return false
}

// compareValues compares two values for equality, handling different types
func compareValues(a, b any) bool {
	// Handle nil comparisons first
	if a == nil && b == nil {
		return true
	}

	switch valA := a.(type) {
	case json.Number:
		switch valB := b.(type) {
		case json.Number:
			return valA == valB
		case float64:
			// Compare json.Number with regular number
			if floatA, err := valA.Float64(); err == nil {
				return floatA == valB
			}
		}
	case string:
		if valB, ok := b.(string); ok {
			return valA == valB
		}
	case bool:
		if valB, ok := b.(bool); ok {
			return valA == valB
		}
	}

	// Direct comparison for other types
	return a == b
}

func compile(expr string) ([]segment, error) {
	if err := validateExpression(expr); err != nil {
		return nil, err
	}

	if expr == "$" {
		return []segment{}, nil
	}

	i := 1 // current parsing index in expr, after '$'
	var segs []segment

	for i < len(expr) {
		seg, newIndex, err := parseSegment(expr, i)
		if err != nil {
			return nil, err
		}
		segs = append(segs, seg)
		i = newIndex
	}

	if len(segs) == 0 && expr != "$" {
		return nil, fmt.Errorf("%w: expression parsed to no segments but was not '$'", ErrSyntax)
	}
	return segs, nil
}

func validateExpression(expr string) error {
	if expr == "" {
		return fmt.Errorf("%w: expression cannot be empty", ErrSyntax)
	}
	if expr[0] != '$' || (len(expr) > 1 && expr[1] != '.' && expr[1] != '[') {
		return fmt.Errorf("%w: expression must start with '$', '$.', or '$['", ErrSyntax)
	}
	return nil
}

func parseSegment(expr string, i int) (segment, int, error) {
	if i >= len(expr) {
		return segment{}, i, fmt.Errorf("%w: unexpected end of expression", ErrSyntax)
	}

	if expr[i] == '.' {
		return parseDotSegment(expr, i)
	}
	if expr[i] == '[' {
		return parseBracketSegment(expr, i)
	}

	return segment{}, i, fmt.Errorf("%w: unexpected token '%c' at position %d, expected '.' or '['", ErrSyntax, expr[i], i)
}

func parseDotSegment(expr string, i int) (segment, int, error) {
	seg := segment{}

	if i+1 < len(expr) && expr[i+1] == '.' { // descendant '..'
		seg.deep = true
		i += 2
	} else { // child '.'
		i++
	}

	if i >= len(expr) { // path cannot end with '.' or '..'
		return segment{}, i, fmt.Errorf("%w: path segment cannot end with '.' or '..'", ErrSyntax)
	}

	if expr[i] == '*' { // wildcard
		seg.sels = append(seg.sels, wildcardSel{})
		i++
	} else { // name selector
		name, newIndex, err := parseName(expr, i)
		if err != nil {
			return segment{}, i, err
		}
		seg.sels = append(seg.sels, nameSel(name))
		i = newIndex
	}

	return seg, i, nil
}

func parseName(expr string, i int) (string, int, error) {
	start := i
	for i < len(expr) && idRune(expr[i]) {
		i++
	}
	if start == i { // name cannot be empty
		return "", i, fmt.Errorf("%w: name selector cannot be empty after '.'", ErrSyntax)
	}
	return expr[start:i], i, nil
}

func parseBracketSegment(expr string, i int) (segment, int, error) {
	i++ // consume '['
	if i >= len(expr) {
		return segment{}, i, fmt.Errorf("%w: unterminated bracket selector, missing ']'", ErrSyntax)
	}

	// Filter expression [?(...)]
	if i+1 < len(expr) && expr[i] == '?' && expr[i+1] == '(' {
		return parseFilterSegment(expr, i)
	}

	// Union / slice / index / quoted names
	return parseUnionSegment(expr, i)
}

func parseFilterSegment(expr string, i int) (segment, int, error) {
	tempEnd := findMatchingBracket(expr, i-1)
	if tempEnd == -1 {
		return segment{}, i, fmt.Errorf("%w: unterminated filter expression, missing ']' for '[?(...)'", ErrSyntax)
	}

	fullContent := expr[i:tempEnd]
	i = tempEnd + 1

	if !strings.HasPrefix(fullContent, "?(") || !strings.HasSuffix(fullContent, ")") {
		return segment{}, i, fmt.Errorf("%w: malformed filter structure, expected '[?(<expression>)]' but got '[%s]'", ErrSyntax, fullContent)
	}
	if len(fullContent) < 4 { // Smallest valid is "?()"
		return segment{}, i, fmt.Errorf("%w: filter expression body is too short in '[%s]'", ErrSyntax, fullContent)
	}

	inside := fullContent[2 : len(fullContent)-1] // Extract content between "?(" and ")"
	fs, err := parseFilter(strings.TrimSpace(inside))
	if err != nil {
		return segment{}, i, fmt.Errorf("parsing filter body '%s': %w", inside, err)
	}

	seg := segment{}
	seg.sels = append(seg.sels, fs)
	return seg, i, nil
}

func parseUnionSegment(expr string, i int) (segment, int, error) {
	startContentForBracket := i
	endContentInBracket := strings.IndexByte(expr[i:], ']')
	if endContentInBracket == -1 {
		return segment{}, i, fmt.Errorf("%w: unterminated bracket selector, missing ']' for content starting at '%s'", ErrSyntax, expr[startContentForBracket:])
	}

	contentInBracket := expr[startContentForBracket : startContentForBracket+endContentInBracket]
	i = startContentForBracket + endContentInBracket + 1

	if strings.TrimSpace(contentInBracket) == "" {
		return segment{}, i, fmt.Errorf("%w: empty bracket selector '[]'", ErrSyntax)
	}

	parts := strings.Split(contentInBracket, ",")
	if len(parts) == 0 && contentInBracket != "" {
		parts = []string{contentInBracket}
	}

	seg := segment{}
	for _, part := range parts {
		selector, err := parseUnionPart(part)
		if err != nil {
			return segment{}, i, err
		}
		seg.sels = append(seg.sels, selector)
	}

	if len(seg.sels) == 0 {
		return segment{}, i, fmt.Errorf("%w: no valid selectors found in bracket content: '[%s]'", ErrSyntax, contentInBracket)
	}

	return seg, i, nil
}

func parseUnionPart(part string) (selector, error) {
	p := strings.TrimSpace(part)
	if p == "" {
		return nil, fmt.Errorf("%w: empty part in union selector", ErrSyntax)
	}

	if p == "*" { // wildcard
		return wildcardSel{}, nil
	}

	if isQuotedName(p) {
		return nameSel(p[1 : len(p)-1]), nil
	}

	if strings.Contains(p, ":") {
		return parseSlice(p)
	}

	if idx, err := strconv.Atoi(p); err == nil {
		if idx < 0 {
			return nil, fmt.Errorf("%w: negative array index (%d) is not supported in streaming mode", ErrNotSupported, idx)
		}
		return indexSel(idx), nil
	}

	return nil, fmt.Errorf("%w: invalid content '%s' in bracket selector", ErrSyntax, p)
}

func isQuotedName(s string) bool {
	return (len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'') ||
		(len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"')
}

func parseSlice(p string) (selector, error) {
	sliceBounds := strings.Split(p, ":")
	if len(sliceBounds) > 3 {
		return nil, fmt.Errorf("%w: too many colons in slice '%s'", ErrSyntax, p)
	}

	s := sliceSel{
		start: 0,
		end:   1 << 30, // effectively "no upper bound"
		step:  1,
	}

	if err := parseSliceBound(&s.start, sliceBounds[0], "start", p); err != nil {
		return nil, err
	}

	if len(sliceBounds) > 1 {
		if err := parseSliceBound(&s.end, sliceBounds[1], "end", p); err != nil {
			return nil, err
		}
	}

	if len(sliceBounds) == 3 {
		if err := parseSliceBound(&s.step, sliceBounds[2], "step", p); err != nil {
			return nil, err
		}
		if s.step == 0 {
			return nil, fmt.Errorf("%w: slice step cannot be zero in '%s'", ErrSyntax, p)
		}
	}

	if s.start < 0 || (len(sliceBounds) > 1 && sliceBounds[1] != "" && s.end < 0) {
		return nil, fmt.Errorf("%w: negative slice indices ('%s') are not supported in streaming mode", ErrNotSupported, p)
	}

	return s, nil
}

func parseSliceBound(target *int, valueStr, boundType, fullSlice string) error {
	trimmed := strings.TrimSpace(valueStr)
	if trimmed == "" {
		return nil
	}

	v, err := strconv.Atoi(trimmed)
	if err != nil {
		return fmt.Errorf("%w: slice %s '%s' in '%s' is not a number", ErrSyntax, boundType, trimmed, fullSlice)
	}

	*target = v
	return nil
}

// findMatchingBracket finds the matching closing bracket for an opening bracket at position start
func findMatchingBracket(expr string, start int) int {
	if start >= len(expr) || expr[start] != '[' {
		return -1
	}

	bracketDepth := 0
	inSingleQuote := false
	inDoubleQuote := false

	for i := start; i < len(expr); i++ {
		c := expr[i]

		// Handle escape sequences in quoted strings
		if i > 0 && expr[i-1] == '\\' {
			continue
		}

		// Handle quoted strings
		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}

		// Skip bracket tracking inside quoted strings
		if inSingleQuote || inDoubleQuote {
			continue
		}

		// Track bracket depth
		switch c {
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth == 0 {
				return i
			}
		}
	}

	return -1
}

// idRune checks if a byte is valid for unquoted names after '.'.
func idRune(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_' || b == '-'
}

// parseFilter compiles a single atomic comparison filter expression.
func parseFilter(s string) (filterSel, error) {
	s = strings.TrimSpace(s)
	m := filterRe.FindStringSubmatch(s)
	if m == nil {
		return filterSel{}, fmt.Errorf("%w: filter expression '%s' must be like '@.path <op> <literal>' or '@.path'", ErrNotSupported, s)
	}

	path, op, literal := m[1], m[2], m[3]
	if path == "" {
		return filterSel{}, fmt.Errorf("%w: filter expression '%s' must have a path starting with @", ErrSyntax, s)
	}

	fs := filterSel{path: strings.Split(path[1:], ".")}

	if op == "" {
		fs.exists = true
		return fs, nil
	}

	cmp, err := parseComparison(op, literal)
	if err != nil {
		return filterSel{}, err
	}

	fs.cmp = cmp
	return fs, nil
}

func parseComparison(op, literal string) (comparison, error) {
	if op == "in" || op == "nin" {
		return parseArrayComparison(op, literal)
	}

	if f, err := strconv.ParseFloat(literal, 64); err == nil {
		return parseNumericComparison(op, f, literal)
	}

	if cmp, ok := parseStringComparison(op, literal); ok {
		return cmp, nil
	}

	if cmp, err := parseRegexComparison(op, literal); err == nil {
		return cmp, nil
	}

	return comparison{}, fmt.Errorf("%w: unsupported literal format '%s'", ErrNotSupported, literal)
}

func parseNumericComparison(op string, value float64, literal string) (comparison, error) {
	switch op {
	case "==", "!=", "<", "<=", ">", ">=":
		return comparison{op: op, num: value, kind: litNum}, nil
	default:
		return comparison{}, fmt.Errorf("%w: operator '%s' not valid for numeric literal '%s'", ErrNotSupported, op, literal)
	}
}

func parseStringComparison(op, literal string) (comparison, bool) {
	isSingleQuoted := len(literal) >= 2 && literal[0] == '\'' && literal[len(literal)-1] == '\''
	isDoubleQuoted := len(literal) >= 2 && literal[0] == '"' && literal[len(literal)-1] == '"'

	if !isSingleQuoted && !isDoubleQuoted {
		return comparison{}, false
	}

	switch op {
	case "==", "!=":
		return comparison{op: op, str: literal[1 : len(literal)-1], kind: litStr}, true
	default:
		return comparison{}, false
	}
}

func parseRegexComparison(op, literal string) (comparison, error) {
	if len(literal) < 2 || literal[0] != '/' {
		return comparison{}, fmt.Errorf("not a regex literal")
	}

	lastSlash := strings.LastIndexByte(literal[1:], '/')
	if lastSlash == -1 {
		return comparison{}, fmt.Errorf("unterminated regex literal")
	}

	lastSlash++ // Adjust for the offset
	pat := literal[1:lastSlash]
	flags := literal[lastSlash+1:]

	if op != "=~" && op != "!~" {
		return comparison{}, fmt.Errorf("%w: operator '%s' not valid for regex literal %s", ErrNotSupported, op, literal)
	}

	goFlags, err := processRegexFlags(flags, literal)
	if err != nil {
		return comparison{}, err
	}

	fullPattern := pat
	if goFlags != "" {
		fullPattern = "(?" + goFlags + ")" + pat
	}

	re, err := regexp.Compile(fullPattern)
	if err != nil {
		return comparison{}, fmt.Errorf("compiling regex literal %s: %w", literal, err)
	}

	return comparison{op: op, regex: re, kind: litRegex}, nil
}

func parseArrayComparison(op, literal string) (comparison, error) {
	if op != "in" && op != "nin" {
		return comparison{}, fmt.Errorf("%w: operator '%s' not valid for array literal %s", ErrNotSupported, op, literal)
	}

	literal = strings.TrimSpace(literal)
	if !strings.HasPrefix(literal, "[") || !strings.HasSuffix(literal, "]") {
		return comparison{}, fmt.Errorf("%w: array literal '%s' must be enclosed in square brackets", ErrSyntax, literal)
	}

	content := strings.TrimSpace(literal[1 : len(literal)-1])
	if content == "" {
		return comparison{op: op, arr: []any{}, kind: litArray}, nil
	}

	var arr []any
	parts := splitArrayElements(content)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		value, err := parseArrayElement(part)
		if err != nil {
			return comparison{}, fmt.Errorf("parsing array element '%s': %w", part, err)
		}
		arr = append(arr, value)
	}

	return comparison{op: op, arr: arr, kind: litArray}, nil
}

// splitArrayElements splits array content by commas, respecting quoted strings
func splitArrayElements(content string) []string {
	if content == "" {
		return nil
	}

	var parts []string
	var current strings.Builder
	inQuotes := false
	quoteChar := byte(0)

	for i, c := range []byte(content) {
		switch {
		case !inQuotes && (c == '\'' || c == '"'):
			inQuotes = true
			quoteChar = c
			current.WriteByte(c)
		case inQuotes && c == quoteChar:
			// Simple escape handling: only check immediate backslash
			escaped := i > 0 && content[i-1] == '\\'
			if !escaped {
				inQuotes = false
			}
			current.WriteByte(c)
		case !inQuotes && c == ',':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

func parseArrayElement(element string) (any, error) {
	element = strings.TrimSpace(element)

	if _, err := strconv.ParseFloat(element, 64); err == nil {
		return json.Number(element), nil
	}

	if element == "true" {
		return true, nil
	}
	if element == "false" {
		return false, nil
	}

	if element == "null" {
		return nil, nil
	}

	if len(element) >= 2 {
		if (element[0] == '\'' && element[len(element)-1] == '\'') ||
			(element[0] == '"' && element[len(element)-1] == '"') {
			return element[1 : len(element)-1], nil
		}
	}

	return nil, fmt.Errorf("unsupported array element format: %s", element)
}

func processRegexFlags(flags, literal string) (string, error) {
	var goFlags string

	// Process supported regex flags
	for _, flag := range []string{"s", "i", "m"} {
		if strings.Contains(flags, flag) {
			goFlags += flag
		}
	}

	for _, fchar := range flags {
		if fchar != 's' && fchar != 'i' && fchar != 'm' {
			return "", fmt.Errorf("%w: unsupported regex flag '%c' in %s", ErrNotSupported, fchar, literal)
		}
	}

	return goFlags, nil
}
