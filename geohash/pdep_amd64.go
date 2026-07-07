//go:build amd64 || amd64p32
// +build amd64 amd64p32

package geohash

// 声明在汇编文件中实现的函数
func pdep64(src, mask uint64) uint64
