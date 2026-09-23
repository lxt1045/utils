package tools

import (
	"reflect"
	"slices"
)

func FlipKV[K comparable, V comparable](in map[K]V) (out map[V]K) {
	out = make(map[V]K, len(in))
	for k, v := range in {
		out[v] = k
	}
	return
}

func FlipSlice[K comparable, V comparable](in []V, skip ...int) (out map[V]K) {
	out = make(map[V]K, len(in))
	for k, v := range in {
		if slices.Contains(skip, k) {
			continue
		}
		x := new(K)
		vk := reflect.ValueOf(x)
		vk.Elem().SetInt(int64(k))
		out[v] = *x
	}
	return
}
