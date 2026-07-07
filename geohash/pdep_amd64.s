//go:build  amd64 || amd64p32 
// +build amd64 amd64p32 


#include "go_asm.h"
#include "textflag.h"
#include "funcdata.h"

// func pdep64(src, mask uint64) uint64
TEXT ·pdep64(SB), NOSPLIT, $0-24
    MOVQ    src+0(FP), AX   // 将第一个参数 (src) 加载到 AX
    MOVQ    mask+8(FP), BX  // 将第二个参数 (mask) 加载到 BX
    PDEPQ   BX, AX, AX      // 执行 PDEP: 将 AX 中的位按 BX 掩码展开，结果存入 AX[reference:4]
    MOVQ    AX, ret+16(FP)  // 将结果 (AX) 存回返回值
    RET
