package tools

import "reflect"

func DeepCopyReflect(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}

	dst := make(map[string]interface{}, len(src))

	for k, v := range src {
		dst[k] = deepCopyValue(v)
	}

	return dst
}

func deepCopyValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	// 获取值的反射对象
	val := reflect.ValueOf(v)

	switch val.Kind() {
	case reflect.Map:
		// 处理嵌套的 map
		m := make(map[string]interface{})
		for _, key := range val.MapKeys() {
			m[key.String()] = deepCopyValue(val.MapIndex(key).Interface())
		}
		return m

	case reflect.Slice, reflect.Array:
		// 处理切片/数组
		length := val.Len()
		slice := make([]interface{}, length)
		for i := 0; i < length; i++ {
			slice[i] = deepCopyValue(val.Index(i).Interface())
		}
		return slice

	case reflect.Ptr:
		// 处理指针
		if val.IsNil() {
			return nil
		}
		elem := val.Elem()
		// 如果是基本类型，直接返回值的拷贝
		if elem.CanInterface() {
			return deepCopyValue(elem.Interface())
		}
		return nil

	case reflect.Struct:
		// 处理结构体（转换为 map）
		m := make(map[string]interface{})
		t := val.Type()
		for i := 0; i < val.NumField(); i++ {
			field := t.Field(i)
			// 只导出可导出的字段
			if field.PkgPath == "" {
				m[field.Name] = deepCopyValue(val.Field(i).Interface())
			}
		}
		return m

	default:
		// 基本类型（int, float, string, bool 等）直接返回
		return v
	}
}
