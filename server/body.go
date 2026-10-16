package main

import (
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
)

func httpMaxBody(c *gin.Context, n int64) io.ReadCloser {
	return http.MaxBytesReader(c.Writer, c.Request.Body, n)
}
