package api

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"github.com/LubyRuffy/rproxy/models"
	"github.com/elazarl/goproxy"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
)

type httpWriter struct {
	h    http.Header
	buf  bufio.Writer
	code int
}

func (hw httpWriter) Header() http.Header {
	return hw.h
}

func (hw httpWriter) Write(p []byte) (int, error) {
	return hw.buf.Write(p)
}

func (hw httpWriter) WriteHeader(statusCode int) {
	hw.code = statusCode
}

func proxyHandler(c *gin.Context) {
	// 每次取三条测试
	var ps []models.Proxy
	if err := models.GetDB().Order("RANDOM()").Limit(3).Find(&ps).Error; err != nil {
		c.JSON(500, map[string]interface{}{
			"code":    500,
			"message": fmt.Sprintf("db failed: %v", err),
		})
		return
	}

	for _, p := range ps {
		proxyURL, err := url.Parse(p.ProxyURL)
		if err != nil {
			c.JSON(500, map[string]interface{}{
				"code":    500,
				"message": fmt.Sprintf("db failed: %v", err),
			})
			return
		}

		go func(proxyURL *url.URL) {
			proxy := goproxy.NewProxyHttpServer()
			proxy.Tr = &http.Transport{
				Proxy:           http.ProxyURL(proxyURL),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
				return resp
			})
			proxy.ServeHTTP(c.Writer, c.Request)
		}(proxyURL)
	}

}
