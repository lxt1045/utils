package log

import (
	"uuid"

	"github.com/gin-gonic/gin"
	"github.com/lxt1045/errors/zerolog"
)

const (
	ginLogID  = "logid"
	ginLogger = "logger"

	// HeaderLogID 是 GinMiddleware 写回给客户端的响应头名，用于把本次请求的
	// log_id 透传给调用方（导出以便 handler/客户端引用同一名字，避免写死字符串）。
	HeaderLogID = "X-Log-Id"
)

func GinLogID(c *gin.Context) uuid.UUID {
	vid, _ := c.Get(ginLogID)
	logid, _ := toUUID(vid)
	return logid
}

func GinWithLogid(c *gin.Context, logid uuid.UUID) *zerolog.Logger {
	c.Set(ginLogID, logid)

	l := zerolog.New(GetOutput())
	l = l.Hook(logidHook{logid: logid})

	c.Set(ginLogger, &l)
	return &l
}

func GinCtx(c *gin.Context) *zerolog.Logger {
	v, _ := c.Get(ginLogger)
	logger, ok := v.(*zerolog.Logger)
	if ok {
		return logger
	}

	vid, _ := c.Get(ginLogID)
	logid, ok := toUUID(vid)
	if !ok {
		logid, ok = toUUID(c.Value(logID{}))
		if ok {
			l := zerolog.Ctx(c)
			c.Set(ginLogID, logid)
			c.Set(ginLogger, &l)
			return l
		}
		logid = uuid.NewV7()
	}

	return GinWithLogid(c, logid)
}

func toUUID(v any) (id uuid.UUID, ok bool) {
	id, _ = v.(uuid.UUID)
	ok = id != uuid.Nil()
	return
}
