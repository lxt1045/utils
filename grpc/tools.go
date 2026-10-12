package grpc

import (
	"context"
	"encoding/json"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"uuid"

	"github.com/lxt1045/errors"
	"github.com/lxt1045/errors/zerolog"
	"github.com/lxt1045/utils/log"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func i64ToStr(i int64) string {
	return strconv.FormatInt(i, 10)
}

const (
	grpcLogIDKey = "log_id"
	svcStartKey  = "s_start"
	svcEndKey    = "s_end"
	cliStartKey  = "c_start"
	cliNameKey   = "c_name"

	fromLog  = 1
	fromGRPC = 2

	warnTimeout int64 = 3000 // 单位ms
)

func getLogid(ctx context.Context) (logid uuid.UUID, from uint8) {
	logid = log.Logid(ctx)
	if logid != uuid.Nil() {
		from = fromLog
		return
	}
	logids := metadata.ValueFromIncomingContext(ctx, grpcLogIDKey)
	if len(logids) > 0 {
		logid, _ = uuid.Parse(logids[0])
		if logid != uuid.Nil() {
			from = fromGRPC
			return
		}
	}
	logid = uuid.NewV7()
	return
}

func GRPCContext(ctx context.Context) context.Context {
	logid, from := getLogid(ctx)
	if from != fromLog {
		ctx, _ = log.WithLogid(ctx, logid)
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(grpcLogIDKey, logid.String()))
	return ctx
}

func getCaller(l *zerolog.Event, full, method string) string {
	if l == nil /* || full == "" */ {
		return ""
	}
	if full[0] == '/' {
		full = full[1:]
	}
	i := strings.IndexByte(full, '.')
	if i < 0 {
		return ""
	}
	pre := full[:i]
	if method == "" {
		j := strings.IndexByte(full[i+1:], '/')
		if j < 0 {
			return ""
		}
		j += i + 1
		method = full[j+1:]
	}
	cs := errors.CallersSkip(3)
	for i, c := range cs {
		if strings.HasSuffix(c.Func, method) && strings.HasPrefix(c.Func, pre) {
			if i+1 < len(cs) {
				return cs[i+1].FileLine
			}
			return ""
		}
	}
	// 回退：动态调用（如 Lua 引擎经 dynamicpb 发起的调用）栈上没有生成代码
	// 客户端方法帧，上面按包名/方法名的启发式匹配必然失败；此时取第一个
	// 非框架/基础设施的业务帧作为 caller。
	// 注意 errors 库的 caller.Func 是短名（只保留末段路径，如 grpc.Invoke），
	// 只能用短名前缀匹配。
	for _, c := range cs {
		if c.Func == "" || c.FileLine == "" {
			continue
		}
		if isInfraFrame(c.Func) {
			continue
		}
		return c.FileLine
	}
	return ""
}

// isInfraFrame 判定短函数名是否属于框架/基础设施包（grpc 框架、本包、
// 日志与错误库、运行时）。caller 回退时不应指向这些帧。
func isInfraFrame(fn string) bool {
	for _, pre := range []string{"grpc.", "errors.", "zerolog.", "log.", "runtime."} {
		if strings.HasPrefix(fn, pre) {
			return true
		}
	}
	return false
}

// protoLogValue 把 proto 消息转成可 JSON 序列化的日志值。
// 生成的 pb 类型走 encoding/json 能输出字段，但引擎 dynamicpb 动态消息的
// 内部字段全是未导出的，encoding/json 只会得到 {}，所以统一用 protojson。
// 非 proto 类型原样返回。
func protoLogValue(v any) any {
	if m, ok := v.(proto.Message); ok {
		if bs, err := protojson.Marshal(m); err == nil {
			return json.RawMessage(bs)
		}
	}
	return v
}

func getMethod(l *zerolog.Event, srv any, full string) string {
	if l == nil || full == "" {
		return ""
	}
	i := strings.LastIndex(full, "/")
	if i < 0 {
		return ""
	}
	f, ok := reflect.TypeOf(srv).MethodByName(full[i+1:])
	if !ok {
		return ""
	}
	// 1. 获取方法的程序计数器 (PC)
	pc := f.Func.Pointer()

	return getPointerName(pc)
}

var (
	mFuncName = func() (p atomic.Pointer[map[uintptr]string]) {
		p.Store(&map[uintptr]string{})
		return
	}()
)

func GetFuncName(i interface{}) (name string) {
	p := reflect.ValueOf(i).Pointer()
	return getPointerName(p)
}
func getPointerName(p uintptr) (name string) {
	m := *mFuncName.Load()
	if name = m[p]; name != "" {
		return name
	}

	// name = runtime.FuncForPC(p).Name() // 通过 reflect.ValueOf 获取函数的 PC（程序计数器）
	f := runtime.FuncForPC(p)
	if f == nil {
		return ""
	}
	name, line := f.FileLine(p) // 通过 reflect.ValueOf 获取函数的 PC（程序计数器）
	if idx := strings.LastIndex(name, "/"); idx > 0 {
		if idx := strings.LastIndex(name[:idx], "/"); idx > 0 {
			name = name[idx+1:]
		}
	}
	name = name + ":" + strconv.Itoa(line)

	mm := make(map[uintptr]string, len(m)+1)
	for k, v := range m {
		mm[k] = v
	}
	mm[p] = name
	mFuncName.Store(&mm)
	return
}
