package log

import (
	"github.com/gin-gonic/gin"
	"github.com/lxt1045/errors/zerolog"
	"github.com/lxt1045/utils/gid"
)

const (
	ginLogID  = "logid"
	ginLogger = "logger"
)

func GinLogID(c *gin.Context) int64 {
	vid, _ := c.Get(ginLogID)
	logid, _ := vid.(int64)
	return logid
}

func GinWithLogid(c *gin.Context, logid int64) *zerolog.Logger {
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
	logid, _ := vid.(int64)
	if logid == 0 {
		logid, ok := c.Value(logID{}).(int64)
		if ok {
			l := zerolog.Ctx(c)
			c.Set(ginLogID, logid)
			c.Set(ginLogger, &l)
			return l
		}
		logid = gid.New()
	}

	return GinWithLogid(c, logid)
}
