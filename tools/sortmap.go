package tools

import (
	"bytes"
	"encoding/json"
	"sort"
)

type Map[V any] struct {
	M          map[string]V
	SortKeys   []string
	EscapeHTML bool
	Indent     bool
}

func NewMap[V any](m map[string]V, sortKeys []string, escapeHTML, indent bool) Map[V] {
	return Map[V]{
		M:          m,
		SortKeys:   sortKeys,
		EscapeHTML: escapeHTML,
		Indent:     indent,
	}
}

type KV struct {
	K string
	V interface{}
}

var _ json.Marshaler = Map[interface{}]{}

func (m Map[V]) MarshalJSON() ([]byte, error) {
	kvs := make([]KV, 0, len(m.M))

	mDel := make(map[string]struct{})
	for _, k := range m.SortKeys {
		v, ok := m.M[k]
		if ok {
			kvs = append(kvs, KV{
				K: k,
				V: v,
			})
			mDel[k] = struct{}{}
		}
	}
	idxSorted := len(kvs)
	for k, v := range m.M {
		if _, ok := mDel[k]; ok {
			continue
		}
		kvs = append(kvs, KV{
			K: k,
			V: v,
		})
	}

	needSort := kvs[idxSorted:]
	sort.Slice(needSort, func(i, j int) bool {
		return needSort[i].K < needSort[j].K
	})

	buf := bytes.Buffer{}
	buf.WriteByte('{')
	for i, kv := range kvs {
		if i != 0 {
			buf.WriteByte(',')
		}
		buf.WriteByte('"')
		buf.WriteString(kv.K)
		buf.WriteByte('"')
		buf.WriteByte(':')

		// bs, err := json.Marshal(kv.V)
		// if err != nil {
		// 	return nil, err
		// }
		// buf.Write(bs)

		encoder := json.NewEncoder(&buf)
		if !m.EscapeHTML {
			encoder.SetEscapeHTML(false) // 关键：关闭转义
		}
		if m.Indent {
			encoder.SetIndent("", "  ")
		}
		err := encoder.Encode(kv.V)
		if err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')

	bs := buf.Bytes()
	if m.EscapeHTML {
		bs = bytes.ReplaceAll(bs, []byte("/"), []byte(`\/`))
	}
	return bs, nil
}
