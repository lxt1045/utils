# geos —— MergeCurves 对照 GEOSLineMerge

本目录用真实的 GEOS(C 库)`GEOSLineMerge` 作为参照，验证上层
`geohash.MergeCurves`(把多条无序曲线缝合成闭合环)的正确性与误差量级。

## 为什么直接绑定 GEOSLineMerge

`simplefeatures` 是 GEOS 的 CGO 封装，但 v0.59.0 的公开 API 并未导出
`LineMerge`——它的 `geos` 子包只封装了 `UnaryUnion`，而 `UnaryUnion` 仅对线做
节点化(noding)，**不会**把多条线首尾缝合成一条连续线串(实测 4 条输入线
仍输出 4 条线)。真正等价于“把无序曲线拼成一条线”的 GEOS 操作是 C 库里的
`GEOSLineMerge`(PostGIS 的 `ST_LineMerge`、simplefeatures 内部走的都是它)。

因此 `geos.go` 用最小 cgo 直接绑定 `GEOSLineMerge`，作为最权威的对照。
几何的构造/解析(WKT 互转)仍使用 simplefeatures 的 `geom` 包。

## 依赖与平台

- 需要 libgeos(含 `libgeos_c` 与 `geos_c.h`)、`pkg-config`、cgo。
- **Windows 无法编译运行**(无 libgeos)。用 WSL(已装 GEOS 3.12.x)。
- 源文件带 `//go:build !windows && cgo` 约束，在 Windows 下会被安全跳过。

## 运行

```sh
# 在 WSL 中(已安装 golang 与 libgeos-dev)
cd geohash/geos
CGO_ENABLED=1 go test -v ./...
```

## 结论(实测)

- **端点精确重合**时，`MergeCurves` 与 `GEOSLineMerge` 还原出的顶点完全一致：
  最大顶点误差 < 5e-8 m(浮点噪声级)，重建环面积相对误差 < 1e-11。
- **端点有微小抖动**(真实数据常见，~0.5m)时，`GEOSLineMerge` 因要求端点
  精确重合而无法缝合(仍返回 MultiLineString)；`MergeCurves` 凭 tolerance
  仍能正确缝合成环。这正是 `MergeCurves` 相对裸 `GEOSLineMerge` 的实用价值。

## 参考

```
https://github.com/libgeos/geos                         # GEOS 仓库
https://github.com/libgeos/libgeos/blob/main/INSTALL.md # GEOS 编译
https://github.com/peterstace/simplefeatures            # GEOS 的 CGO 封装
```
