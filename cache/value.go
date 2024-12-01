package cache

import "strconv"

type strValue string

func (s *strValue) CacheKey() string {
	return string(*s)
}

func StrValue(a string) Value {
	v := strValue(a)
	return &v
}

func ToStr(v Value) string {
	return string(*(v.(*strValue)))
}

type intValue int

func (i *intValue) CacheKey() string {
	return strconv.Itoa(int(*i))
}

func IntValue(i int) Value {
	v := intValue(i)
	return &v
}

func ToInt(v Value) int {
	return int(*(v.(*intValue)))
}

type int64Value int64

func (i *int64Value) CacheKey() string {
	return strconv.FormatInt(int64(*i), 10)
}

func Int64Value(i int64) Value {
	v := int64Value(i)
	return &v
}

func ToInt64(v Value) int64 {
	return int64(*(v.(*int64Value)))
}
