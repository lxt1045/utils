package log

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
	"github.com/lxt1045/errors"
	"github.com/lxt1045/errors/zerolog"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	bodyBuf *strings.Builder
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	//memory copy here!
	w.bodyBuf.Write(b)
	return w.ResponseWriter.Write(b)
}

type bodyWriter struct {
	gin.ResponseWriter
	bs     []byte
	maxLen int
}

func (w bodyWriter) Write(b []byte) (n int, err error) {
	//memory copy here!
	n, err = w.ResponseWriter.Write(b)

	m := n
	if len(w.bs) >= w.maxLen {
		return
	}
	if len(w.bs)+m > w.maxLen {
		m = w.maxLen - len(w.bs)
	}
	w.bs = append(w.bs, b[:m]...)
	return
}

type bodyReader struct {
	r      io.ReadCloser
	bs     []byte
	maxLen int
}

func (r *bodyReader) Read(b []byte) (n int, err error) {
	n, err = r.r.Read(b)
	m := n
	if len(r.bs) >= r.maxLen {
		return
	}
	if len(r.bs)+m > r.maxLen {
		m = r.maxLen - len(r.bs)
	}
	r.bs = append(r.bs, b[:m]...)
	return
}

func Log(reqMaxLen, respMaxLen int) gin.HandlerFunc {
	if respMaxLen <= 0 {
		respMaxLen = 1024
	}
	if reqMaxLen <= 0 {
		reqMaxLen = 256
	}
	return func(c *gin.Context) {
		start := time.Now()
		logger := GinCtx(c)
		reqReader := &bodyReader{
			maxLen: reqMaxLen,
		}
		//if we need to log res body
		respWriter := bodyWriter{
			maxLen:         respMaxLen,
			ResponseWriter: c.Writer,
		}
		c.Writer = respWriter

		if c.Request.Body != nil && (c.Request.Method == http.MethodPut ||
			c.Request.Method == http.MethodPatch || c.Request.Method == http.MethodDelete) {
			reqReader.r = c.Request.Body
			c.Request.Body = io.NopCloser(reqReader)
		}
		defer func() {
			e := recover()
			statusCode := c.Writer.Status()
			loss := int64(time.Since(start))
			loss = loss / int64(time.Millisecond)

			var l *zerolog.Event
			if e != nil {
				l = logger.Error().Caller().Interface("recover", e).
					Array("stack", errors.ZerologStackWithSkips(1, errors.SkipFuncPrefix("gin.CustomRecoveryWithWriter")))
			} else if loss >= 500 && strings.HasPrefix(c.Request.URL.Scheme, "ws") {
				l = logger.Warn()
			} else if statusCode >= 400 {
				l = logger.Warn()
			} else {
				l = logger.Info()
			}

			if e != nil && len(respWriter.bs) == 0 {
				c.JSON(http.StatusOK, &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{
					Code:    -1,
					Message: "服务从panic中恢复",
				})
				c.Abort()
			}
			l.Caller(1).
				Int64("duration/ms", loss).
				Str("method", c.Request.Method).
				Str("url", c.Request.RequestURI).
				Str("req", unsafe.String(unsafe.SliceData(reqReader.bs), len(reqReader.bs))).
				Int("code", statusCode).
				Str("resp", unsafe.String(unsafe.SliceData(respWriter.bs), len(respWriter.bs))).
				Msg("end")
		}()
		c.Next()
	}
}

func Log1(reqMaxLan, respMaxLen int) gin.HandlerFunc {
	if respMaxLen <= 0 {
		respMaxLen = 1024
	}
	if reqMaxLan <= 0 {
		reqMaxLan = 256
	}
	return func(c *gin.Context) {
		start := time.Now()
		logger := GinCtx(c)
		var req []byte
		if c.Request.Body != nil && (c.Request.Method == http.MethodPut ||
			c.Request.Method == http.MethodPatch || c.Request.Method == http.MethodDelete) {
			data, err := io.ReadAll(c.Request.Body)
			if err != nil {
				err = errors.Errorf("读请求数据出错:%s", err.Error())
				logger.Error().Caller().Err(err).Msg("read request body failed")

				c.JSON(http.StatusOK, &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{
					Code:    -1,
					Message: err.Error(),
				})
				c.Abort()
				return
			}
			req = data
			c.Request.Body = io.NopCloser(bytes.NewBuffer(data))
		}
		//if we need to log res body
		blw := bodyLogWriter{
			bodyBuf:        &strings.Builder{},
			ResponseWriter: c.Writer,
		}
		c.Writer = blw
		defer func() {
			e := recover()
			statusCode := c.Writer.Status()
			loss := int64(time.Since(start))
			loss = loss / int64(time.Millisecond)

			l := logger.Info()
			if loss >= 500 && strings.HasPrefix(c.Request.URL.Scheme, "ws") {
				l = logger.Warn()
			} else if statusCode >= 400 {
				l = logger.Warn()
			}
			if e != nil {
				// stack := errors.CallersSkip(1)
				l = logger.Error().Caller().Interface("recover", e).
					Array("stack", errors.ZerologStackWithSkips(1, errors.SkipFuncPrefix("gin.CustomRecoveryWithWriter")))
			}

			strBody := blw.bodyBuf.String()
			if len(strBody) > respMaxLen {
				strBody = strBody[:(respMaxLen - 1)]
			}
			lenReq := len(req)
			if lenReq > reqMaxLan {
				lenReq = reqMaxLan
			}

			if e != nil && strBody == "" {
				c.JSON(http.StatusOK, &struct {
					Code    int    `json:"code"`
					Message string `json:"message"`
				}{
					Code:    -1,
					Message: "服务内部错误",
				})
				c.Abort()
			}
			l.Caller(1).
				Int64("duration/ms", loss).
				Str("method", c.Request.Method).
				Str("url", c.Request.RequestURI).
				Str("req", unsafe.String(unsafe.SliceData(req), lenReq)).
				Int("code", statusCode).
				Str("reply", strBody).
				Msg("end")
		}()
		c.Next()
	}
}
