package ck

import (
	"reflect"
	"strings"
)

// FlattenMap 将嵌套的map拍平成单层map
// 支持嵌套的map[string]interface{}和slice
func FlattenMap(nestedMap map[string]interface{}, separator string) map[string]interface{} {
	if separator == "" {
		separator = "."
	}

	flatMap := make(map[string]interface{})
	flatten(nestedMap, "", flatMap, separator)
	return flatMap
}

// flatten 递归处理嵌套结构
func flatten(data interface{}, prefix string, result map[string]interface{}, separator string) {
	if data == nil {
		return
	}

	// 使用反射处理各种类型
	v := reflect.ValueOf(data)

	switch v.Kind() {
	case reflect.Map:
		// 处理map类型
		for _, key := range v.MapKeys() {
			if key.Kind() == reflect.String {
				mapValue := v.MapIndex(key).Interface()
				newKey := key.String()
				if prefix != "" {
					newKey = prefix + separator + newKey
				}
				flatten(mapValue, newKey, result, separator)
			}
		}
	// case reflect.Slice, reflect.Array:
	// 	// 处理切片和数组类型
	// 	for i := 0; i < v.Len(); i++ {
	// 		sliceValue := v.Index(i).Interface()
	// 		newKey := prefix + separator + strconv.Itoa(i)
	// 		flatten(sliceValue, newKey, result, separator)
	// 	}
	default:
		// 基本类型直接赋值
		result[prefix] = data
	}
}

// UnflattenMap 将拍平的map还原为嵌套map
func UnflattenMap(flatMap map[string]interface{}, separator string) map[string]interface{} {
	if separator == "" {
		separator = "."
	}

	result := make(map[string]interface{})

	for key, value := range flatMap {
		setNestedValue(result, strings.Split(key, separator), value)
	}

	return result
}

// setNestedValue 设置嵌套值
func setNestedValue(target map[string]interface{}, keys []string, value interface{}) {
	if len(keys) == 0 {
		return
	}

	if len(keys) == 1 {
		// 最后一层，直接赋值
		target[keys[0]] = value
		return
	}

	// 检查下一层是否已存在
	next, exists := target[keys[0]]
	if !exists {
		// 创建新的map
		next = make(map[string]interface{})
		target[keys[0]] = next
	}

	// 确保下一层是map类型
	if nextMap, ok := next.(map[string]interface{}); ok {
		setNestedValue(nextMap, keys[1:], value)
	} else {
		// 如果不是map类型，替换为map
		newMap := make(map[string]interface{})
		setNestedValue(newMap, keys[1:], value)
		target[keys[0]] = newMap
	}
}
