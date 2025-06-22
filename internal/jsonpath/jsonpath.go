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
	Value any    // scalar value, or decoded map[string]any / []any for containers
}

type streamContext struct {
	pathStack      *stack.Stack[pathElem]
	valueStack     *stack.Stack[any]
	containerStack *stack.Stack[containerFrame]
	segs           []segment
	dec            *json.Decoder
	yield          func(Result, error) bool
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

	seq := iter.Seq2[Result, error](func(yield func(Result, error) bool) {
		sc := &streamContext{
			pathStack:      stack.New[pathElem](),
			valueStack:     stack.New[any](),
			containerStack: stack.New[containerFrame](),
			segs:           segs,
			dec:            dec,
			yield: func(result Result, err error) bool {
				if ctx.Err() != nil {
					return yield(Result{}, ctx.Err())
				}
				return yield(result, err)
			},
		}

		for {
			if ctx.Err() != nil {
				yield(Result{}, ctx.Err())
				return
			}

			tok, err := dec.Token()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				yield(Result{}, err)
				return
			}

			if sc.containerStack.IsEmpty() {
				if !sc.handleRootToken(tok) {
					return
				}
				if d, ok := tok.(json.Delim); ok {
					switch d {
					case '{':
						sc.containerStack.Push(containerFrame{kind: kindObj, needKey: true})
					case '[':
						sc.containerStack.Push(containerFrame{kind: kindArr})
					}
					sc.pathStack.Push(pathElem{})
					sc.valueStack.Push(nil)
				}
				continue
			}

			top := sc.containerStack.PeekRef()

			if top.kind == kindObj {
				if !sc.handleObjectToken(tok, top) {
					return
				}
				continue
			}

			if top.kind == kindArr {
				if !sc.handleArrayToken(tok, top) {
					return
				}
				continue
			}
		}
	})

	return seq, nil
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
		if d != '{' && d != '[' {
			sc.yield(Result{}, ErrMalformed)
			return false
		}
		if !isRootMatch {
			return true // Continue processing for non-matching root
		}
		actualValue, err := decodeSubtree(sc.dec, d)
		if err != nil {
			sc.yield(Result{}, err)
			return false
		}
		return sc.yield(Result{Path: "$", Value: actualValue}, nil)
	default:
		if isRootMatch {
			return sc.yield(Result{Path: "$", Value: tok}, nil)
		}
		return false // End processing for non-matching scalar root
	}
}

func (sc *streamContext) handleObjectToken(tok any, top *containerFrame) bool {
	if top.needKey {
		return sc.handleObjectKey(tok, top)
	}

	return sc.handleObjectValue(tok, top)
}

func (sc *streamContext) handleObjectKey(tok any, top *containerFrame) bool {
	if d, ok := tok.(json.Delim); ok && d == '}' {
		sc.containerStack.Pop()
		sc.pathStack.Pop()
		sc.valueStack.Pop()
		sc.valueDone()
		return true
	}

	key, ok := tok.(string)
	if !ok {
		return false // Invalid object key
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
		sc.containerStack.Pop()
		sc.pathStack.Pop()
		sc.valueStack.Pop()
		sc.valueDone()
		return true
	}

	sc.pathStack.Push(pathElem{isArray: true, index: top.idx})

	if sc.processFilterMatch(tok) {
		return true
	}

	isMatch := sc.matchNow()
	return sc.processArrayElement(tok, isMatch)
}

func (sc *streamContext) processFilterMatch(tok any) bool {
	if sel, ok, filterSegIdx := isFilterMatch(sc.segs, sc.pathStack.ToSlice(), sc.valueStack.ToSlice()); ok {
		if d, ok := tok.(json.Delim); ok {
			obj, err := decodeSubtree(sc.dec, d)
			if err != nil {
				sc.yield(Result{}, err)
				return true // indicates we handled this case
			}

			if peekElem, ok := sc.pathStack.Peek(); ok && sel.match(peekElem, obj) {
				remainingSegs := sc.segs[filterSegIdx+1:]
				if len(remainingSegs) == 0 {
					pathStr := sc.buildPath()
					sc.yield(Result{Path: pathStr, Value: obj}, nil)
				} else {
					sc.processRemainingSegments(obj, remainingSegs, sc.buildPath(), sc.yield)
				}
			}

			sc.pathStack.Pop()
			sc.valueDone()
			return true // indicates we handled this case
		}
	}
	return false // no filter match, continue with standard processing
}

func (sc *streamContext) processArrayElement(dValue any, isMatch bool) bool {
	return sc.processJSONValue(dValue, isMatch)
}

// processJSONValue handles both container and scalar JSON values with shared logic
func (sc *streamContext) processJSONValue(value any, isMatch bool) bool {
	switch dValue := value.(type) {
	case json.Delim:
		if dValue != '{' && dValue != '[' {
			sc.yield(Result{}, fmt.Errorf("%w: unexpected delimiter", ErrMalformed))
			return false
		}

		if isMatch {
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
		} else {
			sc.valueStack.Push(nil)
			if dValue == '{' {
				sc.containerStack.Push(containerFrame{kind: kindObj, needKey: true})
			} else {
				sc.containerStack.Push(containerFrame{kind: kindArr})
			}
		}
	default:
		sc.valueStack.Push(dValue)
		if isMatch {
			pathStr := sc.buildPath()
			if !sc.yield(Result{Path: pathStr, Value: dValue}, nil) {
				return false
			}
		}
		sc.pathStack.Pop()
		sc.valueStack.Pop()
		sc.valueDone()
	}

	return true
}

func decodeSubtree(dec *json.Decoder, openingDelim json.Delim) (any, error) {
	if openingDelim == '{' {
		return decodeObjectSubtree(dec)
	}
	if openingDelim == '[' {
		return decodeArraySubtree(dec)
	}
	return nil, errors.New("internal error in decodeSubtree with invalid openingDelim")
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

		valueToken, err := dec.Token()
		if err != nil {
			return nil, err
		}

		if vd, ok := valueToken.(json.Delim); ok && (vd == '{' || vd == '[') {
			nestedValue, err := decodeSubtree(dec, vd)
			if err != nil {
				return nil, err
			}
			obj[key] = nestedValue
		} else {
			obj[key] = valueToken
		}
	}
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
	if len(remainingSegs) == 0 {
		return
	}

	sc.processSegments(obj, remainingSegs, basePath, yield)
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

// processDeepSegment handles descendant operator '..' by recursively searching all levels
func (sc *streamContext) processDeepSegment(obj any, seg segment, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	for _, sel := range seg.sels {
		if sc.matchesSelectorForValue(sel, obj) {
			sc.processSegments(obj, remainingSegs, currentPath, yield)
		}
	}

	switch v := obj.(type) {
	case map[string]any:
		for key, value := range v {
			childPath := currentPath + "." + key
			sc.processDeepSegment(value, seg, remainingSegs, childPath, yield)
		}
	case []any:
		for i, value := range v {
			childPath := currentPath + "[" + strconv.Itoa(i) + "]"
			sc.processDeepSegment(value, seg, remainingSegs, childPath, yield)
		}
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
		switch s := sel.(type) {
		case nameSel:
			if value, exists := obj[string(s)]; exists {
				childPath := sc.buildPropertyPath(currentPath, string(s))
				sc.processSegments(value, remainingSegs, childPath, yield)
			}

		case wildcardSel:
			for key, value := range obj {
				childPath := sc.buildPropertyPath(currentPath, key)
				sc.processSegments(value, remainingSegs, childPath, yield)
			}

		case filterSel:
			// Filter selectors don't apply to objects in post-filter processing
			continue

		default:
			// Other selectors (indexSel, sliceSel) don't apply to objects
			continue
		}
	}
}

func (sc *streamContext) processArraySegment(arr []any, seg segment, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	for _, sel := range seg.sels {
		switch s := sel.(type) {
		case indexSel:
			idx := int(s)
			if idx >= 0 && idx < len(arr) {
				childPath := currentPath + "[" + strconv.Itoa(idx) + "]"
				sc.processSegments(arr[idx], remainingSegs, childPath, yield)
			}

		case sliceSel:
			sc.processSliceSelector(arr, s, remainingSegs, currentPath, yield)

		case wildcardSel:
			for i, value := range arr {
				childPath := currentPath + "[" + strconv.Itoa(i) + "]"
				sc.processSegments(value, remainingSegs, childPath, yield)
			}

		case nameSel:
			// Name selectors don't apply to arrays
			continue

		case filterSel:
			for i, value := range arr {
				pe := pathElem{isArray: true, index: i}
				if s.match(pe, value) {
					childPath := currentPath + "[" + strconv.Itoa(i) + "]"
					sc.processSegments(value, remainingSegs, childPath, yield)
				}
			}

		default:
			continue
		}
	}
}

func (sc *streamContext) processSliceSelector(arr []any, slice sliceSel, remainingSegs []segment, currentPath string, yield func(Result, error) bool) {
	step := slice.step
	if step == 0 {
		step = 1
	}

	if step > 0 {
		start := slice.start
		end := min(slice.end, len(arr))

		for i := start; i < end; i += step {
			if i >= 0 && i < len(arr) {
				childPath := currentPath + "[" + strconv.Itoa(i) + "]"
				sc.processSegments(arr[i], remainingSegs, childPath, yield)
			}
		}
	} else {
		start := slice.start
		end := slice.end
		if start >= len(arr) {
			start = len(arr) - 1
		}

		for i := start; i > end; i += step {
			if i >= 0 && i < len(arr) {
				childPath := currentPath + "[" + strconv.Itoa(i) + "]"
				sc.processSegments(arr[i], remainingSegs, childPath, yield)
			}
		}
	}
}

// matchesSelectorForValue checks if a value matches a selector (used for deep scans)
func (sc *streamContext) matchesSelectorForValue(sel selector, value any) bool {
	switch s := sel.(type) {
	case wildcardSel:
		return true

	case nameSel:
		// Name selectors only match object properties, not standalone values
		return false

	case indexSel, sliceSel:
		// Array selectors only match array elements, not standalone values
		return false

	case filterSel:
		// For deep scans, we can apply filters directly to values
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
