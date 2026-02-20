package model

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"
)

// KeyValue represents one key/value request entry.
type KeyValue struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

// KeyValues preserves insertion order for headers/query fields.
type KeyValues []KeyValue

// Get returns the last value for an exact key match.
func (entries KeyValues) Get(key string) (string, bool) {
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Key == key {
			return entries[i].Value, true
		}
	}
	return "", false
}

// GetFold returns the last value for a case-insensitive key match.
func (entries KeyValues) GetFold(key string) (string, bool) {
	for i := len(entries) - 1; i >= 0; i-- {
		if strings.EqualFold(entries[i].Key, key) {
			return entries[i].Value, true
		}
	}
	return "", false
}

// UnmarshalYAML supports both mapping and sequence forms:
// headers:
//
//	Content-Type: application/json
//
// or:
// headers:
//   - key: Content-Type
//     value: application/json
func (entries *KeyValues) UnmarshalYAML(node ast.Node) error {
	switch n := node.(type) {
	case *ast.MappingNode:
		out := make(KeyValues, 0, len(n.Values))
		for _, pair := range n.Values {
			keyNode, ok := pair.Key.(*ast.StringNode)
			if !ok {
				return fmt.Errorf("%w: key/value key must be string", ErrParser)
			}

			value, err := nodeToString(pair.Value)
			if err != nil {
				return fmt.Errorf("%w: invalid value for key %q: %v", ErrParser, keyNode.Value, err)
			}

			out = append(out, KeyValue{
				Key:   keyNode.Value,
				Value: value,
			})
		}
		*entries = out
		return nil
	case *ast.SequenceNode:
		out := make(KeyValues, 0, len(n.Values))
		for index, item := range n.Values {
			mapNode, ok := item.(*ast.MappingNode)
			if !ok {
				return fmt.Errorf("%w: key/value entry at index %d must be mapping", ErrParser, index)
			}

			var (
				key      string
				value    string
				hasKey   bool
				hasValue bool
			)

			for _, pair := range mapNode.Values {
				fieldNode, ok := pair.Key.(*ast.StringNode)
				if !ok {
					return fmt.Errorf("%w: key/value entry field key must be string", ErrParser)
				}

				switch fieldNode.Value {
				case "key":
					strNode, ok := pair.Value.(*ast.StringNode)
					if !ok {
						return fmt.Errorf("%w: key/value entry key must be string", ErrParser)
					}
					key = strNode.Value
					hasKey = true
				case "value":
					parsed, err := nodeToString(pair.Value)
					if err != nil {
						return fmt.Errorf("%w: invalid key/value entry value: %v", ErrParser, err)
					}
					value = parsed
					hasValue = true
				default:
					return fmt.Errorf("%w: key/value entry unknown field %q", ErrParser, fieldNode.Value)
				}
			}

			if !hasKey {
				return fmt.Errorf("%w: key/value entry at index %d missing key", ErrParser, index)
			}
			if !hasValue {
				return fmt.Errorf("%w: key/value entry at index %d missing value", ErrParser, index)
			}

			out = append(out, KeyValue{
				Key:   key,
				Value: value,
			})
		}
		*entries = out
		return nil
	default:
		return fmt.Errorf("%w: key/value field must be mapping or sequence", ErrParser)
	}
}

// MarshalYAML emits the ordered sequence representation.
func (entries KeyValues) MarshalYAML() (any, error) {
	type keyValueYAML struct {
		Key   string `yaml:"key"`
		Value string `yaml:"value"`
	}

	out := make([]keyValueYAML, 0, len(entries))
	for _, entry := range entries {
		out = append(out, keyValueYAML(entry))
	}

	return out, nil
}

func nodeToString(node ast.Node) (string, error) {
	switch n := node.(type) {
	case *ast.NullNode:
		return "", nil
	case *ast.StringNode:
		return n.Value, nil
	case *ast.IntegerNode:
		if n.Value == nil {
			return "", fmt.Errorf("integer node has nil value")
		}
		if v, ok := n.Value.(int64); ok {
			return strconv.FormatInt(v, 10), nil
		}
		if v, ok := n.Value.(uint64); ok {
			return strconv.FormatUint(v, 10), nil
		}
		return "", fmt.Errorf("unexpected integer node value type: %T", n.Value)
	case *ast.FloatNode:
		return strconv.FormatFloat(n.Value, 'f', -1, 64), nil
	case *ast.BoolNode:
		if n.Value {
			return "true", nil
		}
		return "false", nil
	default:
		return "", fmt.Errorf("value must be scalar, got %T", node)
	}
}
