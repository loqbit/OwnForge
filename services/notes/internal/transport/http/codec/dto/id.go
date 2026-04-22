package dto

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// ID is a JSON-safe int64 identifier.
//
// Snowflake IDs may exceed JavaScript's Number.MAX_SAFE_INTEGER (2^53 - 1),
// which can silently lose precision (for example, 2043093237857521664 -> 2043093237857521700).
// This type serializes int64 values as JSON strings to avoid data corruption.
//
// Marshal：   123 → "123"
// Unmarshal: "123" -> 123, or 123 -> 123 (both forms are supported)
type ID int64

// Int64 returns the raw int64 value.
func (id ID) Int64() int64 { return int64(id) }

func (id ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatInt(int64(id), 10))
}

func (id *ID) UnmarshalJSON(b []byte) error {
	// Try the string form first: "123"
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("dto.ID: invalid string %q: %w", s, err)
		}
		*id = ID(v)
		return nil
	}
	// Fall back to the raw numeric form: 123
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("dto.ID: cannot unmarshal %s: %w", string(b), err)
	}
	*id = ID(n)
	return nil
}

// IDSlice is a []int64 that serializes each element as a JSON string.
// Unmarshal accepts both ["1","2"] and [1,2].
type IDSlice []int64

func (s IDSlice) MarshalJSON() ([]byte, error) {
	strs := make([]string, len(s))
	for i, v := range s {
		strs[i] = strconv.FormatInt(v, 10)
	}
	return json.Marshal(strs)
}

func (s *IDSlice) UnmarshalJSON(b []byte) error {
	// Try []string first.
	var strs []string
	if err := json.Unmarshal(b, &strs); err == nil {
		result := make([]int64, len(strs))
		for i, str := range strs {
			v, err := strconv.ParseInt(str, 10, 64)
			if err != nil {
				return fmt.Errorf("dto.IDSlice[%d]: invalid string %q: %w", i, str, err)
			}
			result[i] = v
		}
		*s = result
		return nil
	}
	// Fall back to []int64.
	var nums []int64
	if err := json.Unmarshal(b, &nums); err != nil {
		return fmt.Errorf("dto.IDSlice: cannot unmarshal %s: %w", string(b), err)
	}
	*s = nums
	return nil
}
