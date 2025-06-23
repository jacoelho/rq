package jsonpath

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"strconv"
	"strings"

	"github.com/jacoelho/rq/internal/stack"
)

// Result represents a single match from a JSONPath query.
// It contains both the canonical path and the matched value.
type Result struct {
	Path  string // canonical JSONPath
	Value any    // scalar value, decoded map[string]any / []any, or other types
}

type streamContext struct {
	pathStack      *stack.Stack[pathElem]
	valueStack     *stack.Stack[any]
	containerStack *stack.Stack[containerFrame]
	segs           []segment
	dec            *json.Decoder
	yield          func(Result, error) bool
}

type tokenProcessor struct {
	sc  *streamContext
	ctx context.Context
}

func (sc *streamContext) buildPath() string {
	var b strings.Builder
	b.WriteByte('$')
	if sc.pathStack.IsEmpty() {
		return b.String()
	}
	pathElements := sc.pathStack.ToSlice()
	for i, pe := range pathElements {
		if pe.isArray {
			b.WriteByte('[')
			b.WriteString(strconv.Itoa(pe.index))
			b.WriteByte(']')
		} else {
			if i > 0 || pe.name != "" || len(pathElements) == 1 && pe.name != "" {
				b.WriteByte('.')
			}
			b.WriteString(pe.name)
		}
	}
	return b.String()
}

func (sc *streamContext) matchNow() bool {
	return matchPath(sc.segs, sc.pathStack.ToSlice(), sc.valueStack.ToSlice())
}

func (sc *streamContext) valueDone() {
	if sc.containerStack.IsEmpty() {
		return
	}
	top := sc.containerStack.PeekRef()
	if top.kind == kindArr {
		top.idx++
	}
	if top.kind == kindObj {
		top.needKey = true
	}
}

// Stream compiles a JSONPath expression and returns a lazy iterator of results.
// The iterator yields Result values for each match found in the JSON stream.
//
// The provided context can be used to cancel the streaming operation.
// The reader should contain valid JSON data.
// The expr parameter should be a valid JSONPath expression starting with '$'.
func Stream(ctx context.Context, r io.Reader, expr string) (iter.Seq2[Result, error], error) {
	segs, err := compile(expr)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(r)
	dec.UseNumber() // Use json.Number for all numeric values

	return createSequence(ctx, dec, segs), nil
}

func createSequence(ctx context.Context, dec *json.Decoder, segs []segment) iter.Seq2[Result, error] {
	return func(yield func(Result, error) bool) {
		sc := &streamContext{
			pathStack:      stack.New[pathElem](),
			valueStack:     stack.New[any](),
			containerStack: stack.New[containerFrame](),
			segs:           segs,
			dec:            dec,
			yield:          createYieldFunc(ctx, yield),
		}

		processor := &tokenProcessor{sc: sc, ctx: ctx}
		processor.processTokens()
	}
}

func createYieldFunc(ctx context.Context, yield func(Result, error) bool) func(Result, error) bool {
	return func(result Result, err error) bool {
		if ctx.Err() != nil {
			return yield(Result{}, ctx.Err())
		}
		return yield(result, err)
	}
}

func (tp *tokenProcessor) processTokens() {
	for {
		if tp.ctx.Err() != nil {
			tp.sc.yield(Result{}, tp.ctx.Err())
			return
		}

		tok, err := tp.sc.dec.Token()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			tp.sc.yield(Result{}, err)
			return
		}

		if !tp.processToken(tok) {
			return
		}
	}
}

func (tp *tokenProcessor) processToken(tok any) bool {
	if tp.sc.containerStack.IsEmpty() {
		return tp.handleRootLevel(tok)
	}

	top := tp.sc.containerStack.PeekRef()
	switch top.kind {
	case kindObj:
		return tp.sc.handleObjectToken(tok, top)
	case kindArr:
		return tp.sc.handleArrayToken(tok, top)
	default:
		return false
	}
}

func (tp *tokenProcessor) handleRootLevel(tok any) bool {
	if !tp.sc.handleRootToken(tok) {
		return false
	}

	if d, ok := tok.(json.Delim); ok {
		tp.pushRootContainer(d)
	}
	return true
}

func (tp *tokenProcessor) pushRootContainer(d json.Delim) {
	switch d {
	case '{':
		tp.sc.containerStack.Push(containerFrame{kind: kindObj, needKey: true})
	case '[':
		tp.sc.containerStack.Push(containerFrame{kind: kindArr})
	}
	tp.sc.pathStack.Push(pathElem{})
	tp.sc.valueStack.Push(nil)
}

// Validate checks if a JSONPath expression is syntactically valid.
// It returns an error if the expression is invalid, or nil if the expression is valid.
// The expr parameter should be a valid JSONPath expression starting with '$'.
func Validate(expr string) error {
	_, err := compile(expr)
	return err
}

func (sc *streamContext) handleRootToken(tok any) bool {
	isRootMatch := matchPath(sc.segs, nil, nil)

	switch d := tok.(type) {
	case json.Delim:
		return sc.handleRootDelimiter(d, isRootMatch)
	default:
		return sc.handleRootScalar(tok, isRootMatch)
	}
}

func (sc *streamContext) handleRootDelimiter(d json.Delim, isRootMatch bool) bool {
	if !isValidRootDelimiter(d) {
		sc.yield(Result{}, ErrMalformed)
		return false
	}

	if !isRootMatch {
		return true
	}

	actualValue, err := decodeSubtree(sc.dec, d)
	if err != nil {
		sc.yield(Result{}, err)
		return false
	}
	return sc.yield(Result{Path: "$", Value: actualValue}, nil)
}

func (sc *streamContext) handleRootScalar(tok any, isRootMatch bool) bool {
	if isRootMatch {
		return sc.yield(Result{Path: "$", Value: tok}, nil)
	}
	return false
}

func isValidRootDelimiter(d json.Delim) bool {
	return d == '{' || d == '['
}

func (sc *streamContext) handleObjectToken(tok any, top *containerFrame) bool {
	if top.needKey {
		return sc.handleObjectKey(tok, top)
	}
	return sc.handleObjectValue(tok, top)
}

func (sc *streamContext) handleObjectKey(tok any, top *containerFrame) bool {
	if d, ok := tok.(json.Delim); ok && d == '}' {
		return sc.closeContainer()
	}

	key, ok := tok.(string)
	if !ok {
		return false
	}

	top.key = key
	top.needKey = false
	return true
}

func (sc *streamContext) handleObjectValue(tok any, top *containerFrame) bool {
	sc.pathStack.Push(pathElem{name: top.key})
	isMatch := sc.matchNow()
	return sc.processJSONValue(tok, isMatch)
}

func (sc *streamContext) handleArrayToken(tok any, top *containerFrame) bool {
	if d, ok := tok.(json.Delim); ok && d == ']' {
		return sc.closeContainer()
	}

	sc.pathStack.Push(pathElem{isArray: true, index: top.idx})

	if sc.processFilterMatch(tok) {
		return true
	}

	isMatch := sc.matchNow()
	return sc.processJSONValue(tok, isMatch)
}

func (sc *streamContext) closeContainer() bool {
	sc.containerStack.Pop()
	sc.pathStack.Pop()
	sc.valueStack.Pop()
	sc.valueDone()
	return true
}

func (sc *streamContext) processFilterMatch(tok any) bool {
	sel, ok, filterSegIdx := isFilterMatch(sc.segs, sc.pathStack.ToSlice(), sc.valueStack.ToSlice())
	if !ok {
		return false
	}

	d, ok := tok.(json.Delim)
	if !ok {
		return false
	}

	return sc.handleFilterContainer(d, sel, filterSegIdx)
}

func (sc *streamContext) handleFilterContainer(d json.Delim, sel filterSel, filterSegIdx int) bool {
	fullObj, err := decodeSubtree(sc.dec, d)
	if err != nil {
		sc.yield(Result{}, err)
		return true
	}

	if peekElem, ok := sc.pathStack.Peek(); ok && sel.match(peekElem, fullObj) {
		sc.processFilterResult(fullObj, filterSegIdx)
	}

	sc.pathStack.Pop()
	sc.valueDone()
	return true
}

func (sc *streamContext) processFilterResult(fullObj any, filterSegIdx int) {
	remainingSegs := sc.segs[filterSegIdx+1:]
	if len(remainingSegs) == 0 {
		pathStr := sc.buildPath()
		sc.yield(Result{Path: pathStr, Value: fullObj}, nil)
	} else {
		sc.processRemainingSegments(fullObj, remainingSegs, sc.buildPath(), sc.yield)
	}
}

func (sc *streamContext) hasRemainingSegments() bool {
	pathDepth := sc.pathStack.Size()
	return len(sc.segs) > pathDepth-1
}

func (sc *streamContext) processJSONValue(value any, isMatch bool) bool {
	switch dValue := value.(type) {
	case json.Delim:
		return sc.handleDelimiterValue(dValue, isMatch)
	default:
		return sc.handleScalarValue(dValue, isMatch)
	}
}

func (sc *streamContext) handleDelimiterValue(dValue json.Delim, isMatch bool) bool {
	if !isValidDelimiter(dValue) {
		sc.yield(Result{}, fmt.Errorf("%w: unexpected delimiter", ErrMalformed))
		return false
	}

	if isMatch {
		return sc.handleMatchingContainer(dValue)
	}

	return sc.handleNonMatchingContainer(dValue)
}

func (sc *streamContext) handleMatchingContainer(dValue json.Delim) bool {
	if sc.hasRemainingSegments() {
		return sc.streamThroughContainer(dValue)
	}

	return sc.decodeAndYieldContainer(dValue)
}

func (sc *streamContext) handleNonMatchingContainer(dValue json.Delim) bool {
	sc.valueStack.Push(nil)
	sc.pushContainer(dValue)
	return true
}

func (sc *streamContext) decodeAndYieldContainer(dValue json.Delim) bool {
	pathStr := sc.buildPath()
	actualValue, err := decodeSubtree(sc.dec, dValue)
	if err != nil {
		sc.yield(Result{}, err)
		return false
	}

	if !sc.yield(Result{Path: pathStr, Value: actualValue}, nil) {
		return false
	}

	sc.pathStack.Pop()
	sc.valueDone()
	return true
}

func (sc *streamContext) handleScalarValue(value any, isMatch bool) bool {
	sc.valueStack.Push(value)

	if isMatch {
		pathStr := sc.buildPath()
		if !sc.yield(Result{Path: pathStr, Value: value}, nil) {
			return false
		}
	}

	sc.pathStack.Pop()
	sc.valueStack.Pop()
	sc.valueDone()
	return true
}

func isValidDelimiter(d json.Delim) bool {
	return d == '{' || d == '['
}

func (sc *streamContext) pushContainer(dValue json.Delim) {
	switch dValue {
	case '{':
		sc.containerStack.Push(containerFrame{kind: kindObj, needKey: true})
	case '[':
		sc.containerStack.Push(containerFrame{kind: kindArr})
	}
}

func (sc *streamContext) streamThroughContainer(openingDelim json.Delim) bool {
	sc.valueStack.Push(nil)
	sc.pushContainer(openingDelim)
	return true
}

func decodeSubtree(dec *json.Decoder, openingDelim json.Delim) (any, error) {
	switch openingDelim {
	case '{':
		return decodeObjectSubtree(dec)
	case '[':
		return decodeArraySubtree(dec)
	default:
		return nil, errors.New("internal error in decodeSubtree with invalid openingDelim")
	}
}

func decodeObjectSubtree(dec *json.Decoder) (any, error) {
	obj := make(map[string]any)

	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}

		if d, ok := tok.(json.Delim); ok && d == '}' {
			return obj, nil
		}

		key, ok := tok.(string)
		if !ok {
			return nil, ErrMalformed
		}

		value, err := decodeObjectValue(dec)
		if err != nil {
			return nil, err
		}

		obj[key] = value
	}
}

func decodeObjectValue(dec *json.Decoder) (any, error) {
	valueToken, err := dec.Token()
	if err != nil {
		return nil, err
	}

	if vd, ok := valueToken.(json.Delim); ok && isValidDelimiter(vd) {
		return decodeSubtree(dec, vd)
	}

	return valueToken, nil
}

func decodeArraySubtree(dec *json.Decoder) (any, error) {
	arr := make([]any, 0)

	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}

		if d, ok := tok.(json.Delim); ok {
			if d == ']' {
				return arr, nil
			}
			nestedValue, err := decodeSubtree(dec, d)
			if err != nil {
				return nil, err
			}
			arr = append(arr, nestedValue)
		} else {
			arr = append(arr, tok)
		}
	}
}

func (sc *streamContext) processRemainingSegments(obj any, remainingSegs []segment, basePath string, yield func(Result, error) bool) {
	if len(remainingSegs) > 0 {
		sc.processSegments(obj, remainingSegs, basePath, yield)
	}
}

func (sc *streamContext) processSegments(obj any, segs []segment, currentPath string, yield func(Result, error) bool) {
	if len(segs) == 0 {
		yield(Result{Path: currentPath, Value: obj}, nil)
		return
	}

	seg := segs[0]
	remainingSegs := segs[1:]

	if seg.deep {
		sc.processDeepSegment(obj, seg, remainingSegs, currentPath, yield)
	} else {
		sc.processChildSegment(obj, seg, remainingSegs, currentPath, yield)
	}
}

func (sc *streamContext) processDeepSegment(obj any, seg segment, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	sc.matchSelectorsAtCurrentLevel(obj, seg, remainingSegs, currentPath, yield)
	sc.searchChildrenRecursively(obj, seg, remainingSegs, currentPath, yield)
}

func (sc *streamContext) matchSelectorsAtCurrentLevel(obj any, seg segment, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	for _, sel := range seg.sels {
		if sc.matchesSelectorForValue(sel, obj) {
			sc.processSegments(obj, remainingSegs, currentPath, yield)
		}
	}
}

func (sc *streamContext) searchChildrenRecursively(obj any, seg segment, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	switch v := obj.(type) {
	case map[string]any:
		sc.searchObjectChildren(v, seg, remainingSegs, currentPath, yield)
	case []any:
		sc.searchArrayChildren(v, seg, remainingSegs, currentPath, yield)
	}
}

func (sc *streamContext) searchObjectChildren(obj map[string]any, seg segment, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	for key, value := range obj {
		childPath := sc.buildPropertyPath(currentPath, key)
		sc.processDeepSegment(value, seg, remainingSegs, childPath, yield)
	}
}

func (sc *streamContext) searchArrayChildren(arr []any, seg segment, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	for i, value := range arr {
		childPath := currentPath + "[" + strconv.Itoa(i) + "]"
		sc.processDeepSegment(value, seg, remainingSegs, childPath, yield)
	}
}

func (sc *streamContext) processChildSegment(obj any, seg segment, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	switch v := obj.(type) {
	case map[string]any:
		sc.processObjectSegment(v, seg, remainingSegs, currentPath, yield)
	case []any:
		sc.processArraySegment(v, seg, remainingSegs, currentPath, yield)
	}
}

func (sc *streamContext) processObjectSegment(obj map[string]any, seg segment, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	for _, sel := range seg.sels {
		sc.processObjectSelector(obj, sel, remainingSegs, currentPath, yield)
	}
}

func (sc *streamContext) processObjectSelector(obj map[string]any, sel selector, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	switch s := sel.(type) {
	case nameSel:
		sc.processNameSelector(obj, s, remainingSegs, currentPath, yield)
	case wildcardSel:
		sc.processObjectWildcard(obj, remainingSegs, currentPath, yield)
	case filterSel:
		// Filter selectors don't apply to objects in post-filter processing
	default:
		// Other selectors (indexSel, sliceSel) don't apply to objects
	}
}

func (sc *streamContext) processNameSelector(obj map[string]any, sel nameSel, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	if value, exists := obj[string(sel)]; exists {
		childPath := sc.buildPropertyPath(currentPath, string(sel))
		sc.processSegments(value, remainingSegs, childPath, yield)
	}
}

func (sc *streamContext) processObjectWildcard(obj map[string]any, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	for key, value := range obj {
		childPath := sc.buildPropertyPath(currentPath, key)
		sc.processSegments(value, remainingSegs, childPath, yield)
	}
}

func (sc *streamContext) processArraySegment(arr []any, seg segment, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	for _, sel := range seg.sels {
		sc.processArraySelector(arr, sel, remainingSegs, currentPath, yield)
	}
}

func (sc *streamContext) processArraySelector(arr []any, sel selector, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	switch s := sel.(type) {
	case indexSel:
		sc.processIndexSelector(arr, s, remainingSegs, currentPath, yield)
	case sliceSel:
		sc.processSliceSelector(arr, s, remainingSegs, currentPath, yield)
	case wildcardSel:
		sc.processArrayWildcard(arr, remainingSegs, currentPath, yield)
	case filterSel:
		sc.processArrayFilter(arr, s, remainingSegs, currentPath, yield)
	case nameSel:
		// Name selectors don't apply to arrays
	default:
		// Unknown selector type
	}
}

func (sc *streamContext) processIndexSelector(arr []any, sel indexSel, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	idx := int(sel)
	if idx >= 0 && idx < len(arr) {
		childPath := currentPath + "[" + strconv.Itoa(idx) + "]"
		sc.processSegments(arr[idx], remainingSegs, childPath, yield)
	}
}

func (sc *streamContext) processArrayWildcard(arr []any, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	for i, value := range arr {
		childPath := currentPath + "[" + strconv.Itoa(i) + "]"
		sc.processSegments(value, remainingSegs, childPath, yield)
	}
}

func (sc *streamContext) processArrayFilter(arr []any, sel filterSel, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	for i, value := range arr {
		pe := pathElem{isArray: true, index: i}
		if sel.match(pe, value) {
			childPath := currentPath + "[" + strconv.Itoa(i) + "]"
			sc.processSegments(value, remainingSegs, childPath, yield)
		}
	}
}

func (sc *streamContext) processSliceSelector(arr []any, slice sliceSel, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	step := slice.step
	if step == 0 {
		step = 1
	}

	if step > 0 {
		sc.processForwardSlice(arr, slice, step, remainingSegs, currentPath, yield)
	} else {
		sc.processBackwardSlice(arr, slice, step, remainingSegs, currentPath, yield)
	}
}

func (sc *streamContext) processForwardSlice(arr []any, slice sliceSel, step int, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	start := slice.start
	end := min(slice.end, len(arr))

	for i := start; i < end; i += step {
		if i >= 0 && i < len(arr) {
			childPath := currentPath + "[" + strconv.Itoa(i) + "]"
			sc.processSegments(arr[i], remainingSegs, childPath, yield)
		}
	}
}

func (sc *streamContext) processBackwardSlice(arr []any, slice sliceSel, step int, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	start := slice.start
	if start >= len(arr) {
		start = len(arr) - 1
	}
	end := slice.end

	for i := start; i > end; i += step {
		if i >= 0 && i < len(arr) {
			childPath := currentPath + "[" + strconv.Itoa(i) + "]"
			sc.processSegments(arr[i], remainingSegs, childPath, yield)
		}
	}
}

func (sc *streamContext) matchesSelectorForValue(sel selector, value any) bool {
	switch s := sel.(type) {
	case wildcardSel:
		return true
	case nameSel, indexSel, sliceSel:
		return false
	case filterSel:
		pe := pathElem{}
		return s.match(pe, value)
	default:
		return false
	}
}

func (sc *streamContext) buildPropertyPath(basePath, propertyName string) string {
	if basePath == "$" {
		return "$." + propertyName
	}
	return basePath + "." + propertyName
}
