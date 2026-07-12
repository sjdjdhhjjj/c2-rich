package main

import (
	"fmt"
	"strconv"
)

// task_data 在 JSON 中是 map[string]interface{}，数字会被解析为 float64。
// 这些辅助函数模拟 Python task_data.get(key, default) 的宽松行为。

func tdGetString(d map[string]interface{}, key, def string) string {
	if d == nil {
		return def
	}
	if v, ok := d[key]; ok && v != nil {
		switch t := v.(type) {
		case string:
			return t
		case float64:
			if t == float64(int64(t)) {
				return strconv.FormatInt(int64(t), 10)
			}
			return strconv.FormatFloat(t, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(t)
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	return def
}

func tdGetInt(d map[string]interface{}, key string, def int) int {
	if d == nil {
		return def
	}
	if v, ok := d[key]; ok && v != nil {
		switch t := v.(type) {
		case float64:
			return int(t)
		case string:
			if n, err := strconv.Atoi(t); err == nil {
				return n
			}
		case bool:
			if t {
				return 1
			}
			return 0
		}
	}
	return def
}

func tdGetBool(d map[string]interface{}, key string, def bool) bool {
	if d == nil {
		return def
	}
	if v, ok := d[key]; ok && v != nil {
		switch t := v.(type) {
		case bool:
			return t
		case string:
			return t == "true" || t == "1" || t == "yes"
		case float64:
			return t != 0
		}
	}
	return def
}

// tdGetMap 获取子 map（用于嵌套 task_data）
func tdGetMap(d map[string]interface{}, key string) map[string]interface{} {
	if d == nil {
		return nil
	}
	if v, ok := d[key]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// toString 把任意值转为字符串（用于错误信息）
func toString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case error:
		return t.Error()
	default:
		return fmt.Sprintf("%v", v)
	}
}
