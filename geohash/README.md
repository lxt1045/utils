# geohash 面积计算与曲线合并

本文档记录在 `geohash` 包中新增面积计算函数与曲线合并函数的思考过程与实现结果。

## 需求

实现两个计算 WGS84 闭合曲线所围面积的函数：

1. 输入 `[]uint64` —— 由 WGS84 坐标经 geohash 整型编码而来的顶点列表；
2. 输入 `[]Coords` —— WGS84 浮点经纬度顶点列表。

两者都表示一条闭合曲线（多边形），返回面积（平方米）。

## 思考过程

### 1. 面积公式的选择

球面 / 椭球上的多边形面积有几种量级不同的算法：

| 方法 | 模型 | 精度 | 复杂度 |
| --- | --- | --- | --- |
| 平面投影后用鞋带公式 | 平面 | 低（高纬/大区域畸变大） | 低 |
| 球面角盈（spherical excess） | 正球 | 中（相对误差约 0.1%~0.5%，源于椭球扁率） | 低 |
| 椭球测地面积（Karney/geodesic） | 椭球 | 高（毫米级） | 高 |

对 geohash 的典型使用场景（城市、地块、围栏级别的区域），**球面角盈公式**在精度与实现成本之间最平衡，这也是 `github.com/paulmach/orb` 的 `geo.Area` 采用的方法。

采用的求和式（对多边形每条边累加）：

```
2A = R² · Σ (λ_{i+1} − λ_i) · (2 + sin φ_i + sin φ_{i+1})
```

其中 `λ` 为经度弧度、`φ` 为纬度弧度、`R` 为球半径。

### 2. 球半径的选择

不是用赤道半径 `a=6378137`，而是用 **等面积球半径（authalic radius）** `R ≈ 6371007.18 m`。它是一个表面积与 WGS84 椭球相等的正球体半径，用它做球面近似能让面积误差最小。

### 3. 是否引入第三方库

需求提到可参考或直接引用 `paulmach/orb`、`tidwall/geodesic` 等库。权衡后**选择自行实现**，原因：

- 当前 `geohash` 包（及其所在模块的这一子目录）**零业务依赖**，公式本身只有几行，引入整个几何库属于过度依赖；
- 包内已有 `Coords` 类型和 `EncodeInt/DecodeInt`，自实现能直接复用，接口更贴合；
- `orb` 用的就是同一个球面角盈公式，自实现结果与其一致，不损失精度。

若确实需要**毫米级大地测量精度**，应改用椭球测地算法（Karney 方法），可引入 `github.com/tidwall/geodesic` 的 `PolygonArea`。README 末尾给出对照说明。

### 4. 接口设计要点

- **闭合处理**：用 `coords[(i+1)%n]` 取模连接末点与首点，因此输入首尾点显式相接或不相接都能得到相同结果；
- **顶点顺序无关**：顺时针 / 逆时针只影响符号，最终取 `math.Abs`，返回值恒为非负；
- **退化输入**：顶点数 < 3 直接返回 0；
- **geohash 版复用浮点版**：`AreaGeoInt` 先 `DecodeInt` 还原经纬度，再调用 `AreaCoords`，避免重复实现。

### 5. geohash 版的精度说明

geohash 是有损网格编码，`DecodeInt` 还原的是所在网格的一个角点，顶点会被量化到网格分辨率上。本包用 32bit/轴 编码，分辨率极高，量化误差可忽略；但精度敏感场景应保留原始浮点坐标直接调用 `AreaCoords`。

## 结果

新增文件 `area.go`，导出两个函数：

```go
// 浮点坐标版
func AreaCoords(coords []Coords) float64

// geohash 整型版（内部 DecodeInt 后复用 AreaCoords）
func AreaGeoInt(geos []uint64) float64
```

### 用法

```go
// 1) 浮点坐标
ring := []geohash.Coords{
    {Lat: 39.90, Lng: 116.30},
    {Lat: 39.90, Lng: 116.50},
    {Lat: 40.00, Lng: 116.50},
    {Lat: 40.00, Lng: 116.30},
}
areaM2 := geohash.AreaCoords(ring)

// 2) geohash 整型
geos := make([]uint64, len(ring))
for i, c := range ring {
    geos[i] = geohash.EncodeInt(c.Lat, c.Lng)
}
areaM2 = geohash.AreaGeoInt(geos)
```

### 测试

`area_test.go` 覆盖：

- 赤道附近 1°×1° 方块，与理论值 `111195²` 对比，相对误差 < 1%；
- 顺 / 逆时针顶点顺序结果一致且非负；
- 首尾显式相接与不相接结果一致；
- 退化输入（nil、少于 3 点）返回 0；
- geohash 版与浮点版结果相对误差 < 1e-3。

运行（本目录 `GOOS` 可能被设置为 `linux`，本机测试需指定当前平台）：

```bash
GOOS=windows go test -run TestArea -v ./
```

全部通过。

## 精度对照：需要更高精度时

球面角盈公式相对椭球真值的误差主要来自扁率，量级约 0.1%~0.5%。若需要更高精度：

```go
import "github.com/tidwall/geodesic"

var area, perimeter float64
p := geodesic.WGS84.PolygonArea(points, &area, &perimeter)
_ = p
```

`geodesic.PolygonArea` 基于 Karney 的椭球测地算法，精度可达毫米级，代价是更高的计算开销与一个额外依赖。

---

# 曲线拼接为闭合面

第二部分记录 `merge.go` 中"将多条无序曲线拼接成闭合环"的思考过程与实现结果。

## 需求

一个区域的边界被拆成了若干条曲线（polyline）分别存储：

- 曲线之间**没有先后顺序**，需要根据端点位置自动衔接；
- 每条曲线的**方向不确定**（正向 / 反向都可能），需要按需反转；
- 相邻曲线在衔接处**共享端点**（甚至因精度问题略有偏差），需要**按可接受误差去重**。

要求实现两个函数：

1. `MergeCurvesGeoInt(curves [][]uint64, tolerance float64) []uint64` —— geohash 版本；
2. `MergeCurves(curves [][]Coords, tolerance float64) []Coords` —— 浮点坐标版本。

## 思考过程

### 1. 问题本质

这是经典的 **line merging / polygon reconstruction** 问题，等价于 JTS/GEOS 的 `LineMerger`：
给定一堆无序、无向的线段，按共享端点把它们连成尽量长的链，闭合后即为多边形边界。

### 2. 衔接判定：为什么用 `tolerance`（米）

端点"重合"不能用浮点相等判断——采样精度、geohash 量化都会让本应重合的点产生微小偏差。
因此用**可接受误差 `tolerance`（米）**作为阈值：两端点距离 `<= tolerance` 即视为同一点。
距离复用包内 `Dist`（等距圆柱近似），对这种小尺度比较足够精确。

`tolerance` 同时服务两处去重：

- 单条曲线**内部**相邻冗余点的折叠（`dedupConsecutive`）；
- 曲线**之间**衔接点的重合判定与去重。

### 3. 拼接算法：贪心链式连接

不引入第三方几何库（本包零业务依赖），自实现贪心拼接：

1. 每条曲线先做内部去重；
2. 取第一条作为初始链，维护链的首端 `head` 与尾端 `tail`；
3. 反复扫描剩余曲线，找到某端点落在 `head`/`tail` 的 `tolerance` 内的曲线：
   - 四种衔接方向（tail→c0、tail→cn、cn→head、c0→head），必要时反转曲线；
   - 衔接时**丢弃重合的那个端点**完成去重；
4. 无曲线可衔接时停止；
5. 若首尾两端也在 `tolerance` 内，丢弃重复尾点，得到闭合环。

时间复杂度 O(n²)（n 为曲线数），对边界曲线这种规模足够。

### 4. geohash 版本的处理

`MergeCurvesGeoInt` 先 `DecodeInt` 还原坐标，调用 `MergeCurves` 拼接，再 `EncodeInt` 编码回 `[]uint64`。
需注意：geohash 量化会把端点吸附到网格角点，**`tolerance` 应不小于网格尺寸**，否则本应重合的端点可能因量化被判为不相接。

### 5. 边界与取舍

- 若输入无法连成单一环（存在断裂或多个独立环），只返回从初始曲线出发能连通的那条链——保持行为简单可预测；
- 返回的闭合环不含重复首尾点，可直接传入 `AreaCoords` 求面积，与第一部分自然衔接。

## 实现结果

`merge.go` 导出：

- `MergeCurves([]Coords 的列表, tolerance) []Coords`
- `MergeCurvesGeoInt([]uint64 的列表, tolerance) []uint64`

`merge_test.go` 覆盖：

- 把打乱顺序、部分反向的四条边拼回正方形，并验证面积与直接构造一致；
- `tolerance` 对微偏差端点的衔接与去重；
- geohash 版拼接后往返编解码正确；
- 空输入返回 nil。

```bash
GOOS=windows go test -run 'TestMerge|TestArea' -v ./
```

全部通过。
