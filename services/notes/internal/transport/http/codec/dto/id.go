package dto

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// ID 是对 JSON 安全的 int64 标识符。
//
// Snowflake ID 可能超过 JavaScript 的 Number.MAX_SAFE_INTEGER（2^53 - 1），
// 从而导致静默精度丢失（例如 2043093237857521664 → 2043093237857521700）。
// 这个类型会将 int64 序列化为 JSON 字符串，以避免数据损坏。
//
// Marshal：   123 → "123"
// Unmarshal："123" → 123，或 123 → 123（两种格式都支持）
type ID int64

// Int64 返回原始的 int64 值。
func (id ID) Int64() int64 { return int64(id) }

func (id ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatInt(int64(id), 10))
}

func (id *ID) UnmarshalJSON(b []byte) error {
	// 优先尝试字符串形式："123"
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return fmt.Errorf("dto.ID: invalid string %q: %w", s, err)
		}
		*id = ID(v)
		return nil
	}
	// 回退到原始数字形式：123
	var n int64
	if err := json.Unmarshal(b, &n); err != nil {
		return fmt.Errorf("dto.ID: cannot unmarshal %s: %w", string(b), err)
	}
	*id = ID(n)
	return nil
}

// IDSlice 是一个 []int64，会将每个元素序列化为 JSON 字符串。
// Unmarshal 同时接受 ["1","2"] 和 [1,2]。
type IDSlice []int64

func (s IDSlice) MarshalJSON() ([]byte, error) {
	strs := make([]string, len(s))
	for i, v := range s {
		strs[i] = strconv.FormatInt(v, 10)
	}
	return json.Marshal(strs)
}

func (s *IDSlice) UnmarshalJSON(b []byte) error {
	// 优先尝试 []string
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
	// 回退到 []int64
	var nums []int64
	if err := json.Unmarshal(b, &nums); err != nil {
		return fmt.Errorf("dto.IDSlice: cannot unmarshal %s: %w", string(b), err)
	}
	*s = nums
	return nil
}
