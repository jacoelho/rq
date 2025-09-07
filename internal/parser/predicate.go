package parser

import (
	"errors"
	"fmt"

	yaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

// normalizeNumericType converts all numeric types to a consistent representation.
// This eliminates the unpredictability of YAML parsing that can produce various
// numeric types (int, int64, uint64, float64) for the same value.
func normalizeNumericType(value any) any {
	switch v := value.(type) {
	// Normalize all integer types to int64
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v // Already normalized
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return int64(v) // Note: Potential overflow, but matches original behavior
	// Normalize all float types to float64
	case float32:
		return float64(v)
	case float64:
		return v // Already normalized
	// Keep other types as-is
	default:
		return value
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
			// Parse the value
			if err := yaml.NodeToValue(valNode.Value, &p.Value); err != nil {
				return fmt.Errorf("failed to parse value: %w", err)
			}
			// Normalize numeric types for consistency
			p.Value = normalizeNumericType(p.Value)
		default:
			// Handle direct operation format (e.g., "equals": "test")
			p.Operation = key.Value
			if err := yaml.NodeToValue(valNode.Value, &p.Value); err != nil {
				return fmt.Errorf("failed to parse value for %q: %w", key.Value, err)
			}
			// Normalize numeric types for consistency
			p.Value = normalizeNumericType(p.Value)
		}
	}

	if p.Operation == "" {
		return errors.New("predicate must specify an operation")
	}

	return nil
}
