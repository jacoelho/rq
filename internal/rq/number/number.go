package number

import (
	"encoding/json"
	"fmt"
)

// ToFloat64 converts supported numeric values to float64.
func ToFloat64(value any) (float64, bool) {
	switch current := value.(type) {
	case int:
		return float64(current), true
	case int8:
		return float64(current), true
	case int16:
		return float64(current), true
	case int32:
		return float64(current), true
	case int64:
		return float64(current), true
	case uint:
		return float64(current), true
	case uint8:
		return float64(current), true
	case uint16:
		return float64(current), true
	case uint32:
		return float64(current), true
	case uint64:
		return float64(current), true
	case float32:
		return float64(current), true
	case float64:
		return current, true
	case json.Number:
		parsed, err := current.Float64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

// ToStrictInt converts integer-typed values into int.
func ToStrictInt(value any) (int, error) {
	switch current := value.(type) {
	case int:
		return current, nil
	case int8:
		return int(current), nil
	case int16:
		return int(current), nil
	case int32:
		return int(current), nil
	case int64:
		return int(current), nil
	case uint:
		return int(current), nil
	case uint8:
		return int(current), nil
	case uint16:
		return int(current), nil
	case uint32:
		return int(current), nil
	case uint64:
		return int(current), nil
	default:
		return 0, fmt.Errorf("value %T is not an integer", value)
	}
}
