package jsonpath

// Package jsonpath provides an O(depth) memory JSONPath engine for streaming JSON data.
// It emits a lazy iterator of Result values, each containing the canonical path
// and the value of every node that matches the JSONPath expression.
//
// Supported selectors (RFC 9535 terminology):
//   - Child `.` and descendant `..` segments
//   - Name, array index, wildcard `*`, slices `start:end:step`, unions `[a,b]`
//   - Scalar filters `[?(@ <op> <literal>)]` where:
//     <op>      →  ==  !=  <  <=  >  >=  =~  !~  in  nin
//     <literal> →  number  |  'string'  |  /regex/flags  |  [value1,value2,...]
//     (flags: i,m; array values can be strings, numbers, booleans, null)
//
// Unsupported features raise ErrNotSupported at compile time.
