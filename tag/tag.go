package tag

import (
	"reflect"
	"strings"
	"sync"
	"unsafe"

	"github.com/lxt1045/errors"
)

// TagInfo 拥有tag的struct的成员的解析结果
type TagInfo struct {
	// 常用的放前面，在缓存的概率大
	TagName string //

	BaseType reflect.Type //
	BaseKind reflect.Kind // 次成员可能是 **string,[]int 等这种复杂类型,这个 用来指示 "最里层" 的类型
	Offset   uintptr      //偏移量
}

var (
	tags     = make(map[string]map[string]*TagInfo)
	tagsLock sync.Mutex
)

func NewTagInfos(in interface{}, tag string) (tis map[string]*TagInfo, err error) {
	typIn := reflect.TypeOf(in)
	tis = func() (tis map[string]*TagInfo) {
		tagsLock.Lock()
		defer tagsLock.Unlock()
		tis = tags[typIn.String()]
		return
	}()
	if tis != nil {
		return
	}
	tis, err = newTagInfos(typIn, tag)
	if err != nil {
		return
	}
	tagsLock.Lock()
	defer tagsLock.Unlock()
	tags[typIn.String()] = tis
	return
}

func newTagInfos(typIn reflect.Type, tag string) (tis map[string]*TagInfo, err error) {
	if typIn.Kind() == reflect.Pointer {
		typIn = typIn.Elem()
	}
	if typIn.Kind() != reflect.Struct {
		err = errors.Errorf("NewStructTagInfo only accepts structs; got %v", typIn.Kind())
		return
	}

	tis = make(map[string]*TagInfo)

	// 解析 struct 成员类型
	for i := 0; i < typIn.NumField(); i++ {
		field := typIn.Field(i)
		son := &TagInfo{
			BaseType: field.Type,
			Offset:   field.Offset,
			BaseKind: field.Type.Kind(),
			TagName:  field.Name,
		}

		if !field.IsExported() {
			continue // 非导出成员不处理
		}
		tagv := strings.TrimSpace(field.Tag.Get(tag))
		tvs := strings.Split(tagv, ",")
		tagName := strings.TrimSpace(tvs[0]) // 此处加上 双引号 是为了方便使用 改进后的 hash map

		if tagName == "-" {
			continue
		}

		tis[son.TagName] = son
		if tagName != "" {
			son.TagName = tagName
			tis[son.TagName] = son
		}

	}

	return
}

func GetElem[E any, T any](p *T, offset uintptr) (pOut E) {
	p1 := unsafe.Pointer(p)
	p2 := unsafe.Pointer(uintptr(p1) + uintptr(offset))
	return *(*E)(p2)
}

func GetElemPointer[E any, T any](p *T, offset uintptr) (pOut *E) {
	p1 := unsafe.Pointer(p)
	p2 := unsafe.Pointer(uintptr(p1) + uintptr(offset))
	return (*E)(p2)
}
