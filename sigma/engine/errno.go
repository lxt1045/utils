package engine

import (
	"github.com/lxt1045/errors"
)

/*
错误码分配： ( 32bit 最大值: 42949 67296)
  |  1 ~ 30000  | 0 ~ 99999 |
   模块编号       错误编号
*/

const moduleCode = 200000

var (
	UtilsUnexpected = errors.NewCode(-1, moduleCode+1, "")
)
