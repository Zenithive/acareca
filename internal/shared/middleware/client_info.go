package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
)

type contextKey uint8

const (
	contextKeyUserAgent contextKey = iota
	contextKeyIP
)

func ClientInfo() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, contextKeyUserAgent, c.GetHeader("User-Agent"))
		ctx = context.WithValue(ctx, contextKeyIP, c.ClientIP())
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func UserAgentFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(contextKeyUserAgent).(string)
	return v
}

func IPFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(contextKeyIP).(string)
	return v
}
