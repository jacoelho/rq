package jsonpath

// Constants for container types used during JSON parsing.
const (
	kindObj containerKind = iota
	kindArr
)

// containerKind represents the type of JSON container being processed.
type containerKind uint8

// containerFrame maintains state information for JSON containers during streaming.
// It tracks the current position and state within objects and arrays.
type containerFrame struct {
	kind    containerKind
	idx     int    // current index for arrays
	needKey bool   // true if object expects a key next
	key     string // last key read for an object
}

// pathElem represents a single element in a JSONPath.
// It can represent either an object property or an array index.
type pathElem struct {
	isArray bool
	name    string // property name for objects
	index   int    // index for arrays
}
