package engine

import (
	"github.com/lxt1045/errors"
)

/*
错误码分配：
| 31~24(8bit:256) | 23~16(8bit:256) | 15~0(16bit:65536) |
     服务编号         模块编号              错误编号
*/

// 所有错误统一写在这里，统一分配错误码

// utils utils包错误
var (
	UtilsUnexpected = errors.NewCode(0, 0x01010001, "unexpected error")
)
