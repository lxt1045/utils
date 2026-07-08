//go:build !windows && cgo

// Package geos 提供对 libgeos_c 中 GEOSLineMerge 的最小 cgo 封装，
// 作为 geohash.MergeCurves 的参照实现。
//
// 说明：simplefeatures v0.59.0 的公开 API(含其 geos 子包)并未导出
// LineMerge——它只封装了 UnaryUnion，而 UnaryUnion 仅对线做节点化
// (noding)，并不会把多条线首尾缝合成一条连续线串。真正等价于
// "把无序曲线拼接成一条线"的 GEOS 操作是 C 库里的 GEOSLineMerge
// (simplefeatures 内部走的也是这个 C 函数)。因此这里直接绑定
// GEOSLineMerge，作为最权威的对照。
//
// 该包只能在装有 libgeos 的类 Unix 环境(如 WSL)下编译运行，需要 cgo。
package geos

/*
#cgo pkg-config: geos
#cgo LDFLAGS: -lgeos_c
#include <stdlib.h>
#include <geos_c.h>

// GEOS 的错误/通知回调需要一个 C 函数指针；这里用空实现，错误通过
// 返回的空指针来判断即可，避免把 va_list 传回 Go。
static void geosNoticeHandler(const char *fmt, ...) {}
static void geosErrorHandler(const char *fmt, ...) {}

static GEOSContextHandle_t geosInit() {
    GEOSContextHandle_t h = GEOS_init_r();
    GEOSContext_setNoticeHandler_r(h, geosNoticeHandler);
    GEOSContext_setErrorHandler_r(h, geosErrorHandler);
    return h;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// LineMergeWKT 接收一段 MULTILINESTRING 的 WKT，调用 libgeos 的
// GEOSLineMerge 把其中的多条线尽可能缝合成连续线串，返回结果的 WKT
// (可能是 LINESTRING 或 MULTILINESTRING)。
//
// 这是对 GEOS C API 的直接调用，与 PostGIS 的 ST_LineMerge、
// simplefeatures 内部走的路径使用同一实现。
func LineMergeWKT(inputWKT string) (string, error) {
	handle := C.geosInit()
	if handle == nil {
		return "", errors.New("geos: GEOS_init_r failed")
	}
	defer C.finishGEOS_r(handle)

	reader := C.GEOSWKTReader_create_r(handle)
	if reader == nil {
		return "", errors.New("geos: GEOSWKTReader_create_r failed")
	}
	defer C.GEOSWKTReader_destroy_r(handle, reader)

	cin := C.CString(inputWKT)
	defer C.free(unsafe.Pointer(cin))

	g := C.GEOSWKTReader_read_r(handle, reader, cin)
	if g == nil {
		return "", errors.New("geos: parse input WKT failed")
	}
	defer C.GEOSGeom_destroy_r(handle, g)

	merged := C.GEOSLineMerge_r(handle, g)
	if merged == nil {
		return "", errors.New("geos: GEOSLineMerge_r returned null")
	}
	defer C.GEOSGeom_destroy_r(handle, merged)

	writer := C.GEOSWKTWriter_create_r(handle)
	if writer == nil {
		return "", errors.New("geos: GEOSWKTWriter_create_r failed")
	}
	defer C.GEOSWKTWriter_destroy_r(handle, writer)
	// 去掉多余的尾随零，便于阅读；不影响数值。
	C.GEOSWKTWriter_setTrim_r(handle, writer, 1)
	C.GEOSWKTWriter_setRoundingPrecision_r(handle, writer, 12)

	cout := C.GEOSWKTWriter_write_r(handle, writer, merged)
	if cout == nil {
		return "", errors.New("geos: write output WKT failed")
	}
	defer C.GEOSFree_r(handle, unsafe.Pointer(cout))

	return C.GoString(cout), nil
}
