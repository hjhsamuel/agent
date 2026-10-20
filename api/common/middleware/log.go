package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func RequestLogger(ctx *gin.Context) {
	startTime := time.Now()
	fields := logrus.Fields{"Method": ctx.Request.Method, "Client": ctx.ClientIP()}

	ctx.Next()

	fields["StatusCode"] = ctx.Writer.Status()
	fields["Cost"] = time.Since(startTime)

	logrus.WithFields(fields).Info(ctx.Request.URL.Path)
}
