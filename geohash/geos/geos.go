//go:build !windows && cgo

// Package geos 提供对 libgeos_c 若干函数的最小 cgo 封装，作为
// geohash.MergeCurves 系列的参照实现：
//   - LineMergeWKT  : 绑定 GEOSLineMerge，把多条线首尾缝合成连续线串。
//   - UnaryUnionWKT : 绑定 GEOSUnaryUnion，对线做 noding + 去重叠 + dissolve，
//                     是容差 snap-rounding 版 MergeCurves 的 noding 参照。
//
// 说明：simplefeatures v0.59.0 的公开 API 并未导出这两个操作的直接入口，
// 故这里直接绑定 C 库函数(与 PostGIS ST_LineMerge / ST_UnaryUnion 同一实现)。
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

// UnaryUnionWKT 接收一段几何对象的 WKT(可以是 MULTILINESTRING 或其他类型)，
// 调用 libgeos 的 GEOSUnaryUnion 对其做 noding + dissolve + 去重叠，
// 返回结果的 WKT(对线输入，通常是 MULTILINESTRING 或 LINESTRING)。
//
// UnaryUnion 做真正的几何 noding：在交点处打断线段、去掉重叠段、合并
// 共享端点的线段。这是容差 snap-rounding 实现的参照标准。
//
// 注意：GEOSUnaryUnion 要求输入顶点**精确重合**才会合并；近似重合但不相等
// 的点会留下细缝或多余边。若要对比容差版本，调用方需先对输入 snap 到网格。
func UnaryUnionWKT(inputWKT string) (string, error) {
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

	unioned := C.GEOSUnaryUnion_r(handle, g)
	if unioned == nil {
		return "", errors.New("geos: GEOSUnaryUnion_r returned null")
	}
	defer C.GEOSGeom_destroy_r(handle, unioned)

	writer := C.GEOSWKTWriter_create_r(handle)
	if writer == nil {
		return "", errors.New("geos: GEOSWKTWriter_create_r failed")
	}
	defer C.GEOSWKTWriter_destroy_r(handle, writer)
	C.GEOSWKTWriter_setTrim_r(handle, writer, 1)
	C.GEOSWKTWriter_setRoundingPrecision_r(handle, writer, 12)

	cout := C.GEOSWKTWriter_write_r(handle, writer, unioned)
	if cout == nil {
		return "", errors.New("geos: write output WKT failed")
	}
	defer C.GEOSFree_r(handle, unsafe.Pointer(cout))

	return C.GoString(cout), nil
}
