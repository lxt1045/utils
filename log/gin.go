package log

import (
	"github.com/gin-gonic/gin"
	"github.com/lxt1045/errors/zerolog"
	"github.com/lxt1045/utils/gid"
)

func GetLogID(ctx *gin.Context) int64 {
	logid, _ := getLogID(ctx)
	return logid
}

func getLogID(ctx *gin.Context) (logid int64, ok bool) {
	vid, _ := ctx.Get(ginLogID)
	logid, ok = vid.(int64)
	return
}
func GinGet(ctx *gin.Context) *zerolog.Logger {
	v, _ := ctx.Get(ginLogger)
	logger, ok := v.(*zerolog.Logger)
	if ok {
		return logger
	}

	vid, _ := ctx.Get(ginLogID)
	logid, _ := vid.(int64)
	if logid == 0 {
		logid = gid.GetGID()
	}

	return GinWithLogid(ctx, logid)
}

func GinCtx(ctx *gin.Context) *zerolog.Logger {
	return GinGet(ctx)
}
func GinWithLogid(ctx *gin.Context, logid int64) *zerolog.Logger {
	ctx.Set(ginLogID, logid)

	l := zerolog.New(output)
	l = l.Hook(logidHook{logid: logid})
	// l = l.Hook(th) // l = l.With().Timestamp().Logger()

	ctx.Set(ginLogger, &l)
	return &l
}
