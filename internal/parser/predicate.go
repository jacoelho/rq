package parser

import (
	"errors"
	"fmt"

	"github.com/goccy/go-yaml/ast"
)

// nodeToValue extracts values from AST nodes.
// integer node value is normalized to int64
// float node value is always float64
func nodeToValue(node ast.Node) (any, error) {
	switch n := node.(type) {
	case *ast.IntegerNode:
		if n.Value == nil {
			return nil, errors.New("integer node has nil value")
		}
		if v, ok := n.Value.(int64); ok {
			return v, nil
		}
		if v, ok := n.Value.(uint64); ok {
			return int64(v), nil
		}
		return nil, fmt.Errorf("unexpected integer node value type: %T", n.Value)
	case *ast.FloatNode:
		return n.Value, nil
	case *ast.StringNode:
		return n.Value, nil
	case *ast.BoolNode:
		return n.Value, nil
	case *ast.NullNode:
		return nil, nil
	case *ast.SequenceNode:
		var result []any
		for i, item := range n.Values {
			val, err := nodeToValue(item)
			if err != nil {
				return nil, fmt.Errorf("invalid value at index %d: %w", i, err)
			}
			result = append(result, val)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported node type: %T", node)
	}
}

// Predicate represents a parsed predicate from YAML.
// The parser handles YAML parsing only; validation is delegated to the evaluator.
type Predicate struct {
	Operation string
	Value     any
}

// UnmarshalYAML decodes a predicate from YAML.
// It performs basic parsing only; validation should be done by the evaluator.
func (p *Predicate) UnmarshalYAML(node ast.Node) error {
	mapNode, ok := node.(*ast.MappingNode)
	if !ok {
		return errors.New("predicate must be a mapping")
	}
	if len(mapNode.Values) == 0 {
		return errors.New("predicate mapping is empty")
	}

	for _, valNode := range mapNode.Values {
		key, ok := valNode.Key.(*ast.StringNode)
		if !ok {
			return errors.New("predicate key must be a string")
		}

		switch key.Value {
		case "op":
			// Handle explicit "op" + "value" format
			opNode, ok := valNode.Value.(*ast.StringNode)
			if !ok {
				return errors.New("op value must be a string")
			}
			p.Operation = opNode.Value
		case "value":
			value, err := nodeToValue(valNode.Value)
			if err != nil {
				return fmt.Errorf("failed to parse value: %w", err)
			}
			p.Value = value
		default:
			// Handle direct operation format (e.g., "equals": "test")
			p.Operation = key.Value
			value, err := nodeToValue(valNode.Value)
			if err != nil {
				return fmt.Errorf("failed to parse value for %q: %w", key.Value, err)
			}
			p.Value = value
		}
	}

	if p.Operation == "" {
		return errors.New("predicate must specify an operation")
	}

	return nil
}
