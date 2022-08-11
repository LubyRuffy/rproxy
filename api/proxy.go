package api

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/LubyRuffy/rproxy/models"
	"github.com/elazarl/goproxy"
	"github.com/gin-gonic/gin"
	"h12.io/socks"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

var (
	defaultTimeOut = time.Second * 15
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
	db := models.GetDB()
	proxy := goproxy.NewProxyHttpServer()

	if c.Request.Method == http.MethodConnect {
		db = db.Where(&models.Proxy{Connect: true})
	}

	// 每次取三条测试
	var ps []models.Proxy
	if err := models.GetDB().Order("RANDOM()").Limit(3).Find(&ps).Error; err != nil {
		c.JSON(500, map[string]interface{}{
			"code":    500,
			"message": fmt.Sprintf("db failed: %v", err),
		})
		return
	}

	proxy.Tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		ch := make(chan net.Conn, 1)
		hasSign := false
		var signLock sync.Mutex
		for _, p := range ps {
			go func(p models.Proxy) {
				log.Printf("fetch %s from %s", c.Request.RequestURI, p.ProxyURL)

				var conn net.Conn
				var err error

				if c.Request.Method == http.MethodConnect {
					switch p.ProxyType {
					case "http", "https":
						conn, err = proxy.NewConnectDialToProxy(p.ProxyURL)(network, addr)
					case "socks5":
						conn, err = socks.Dial(p.ProxyURL)(network, addr)
					}
				} else {
					//http.ProxyURL()
					conn, err = net.Dial(network, addr)
				}

				if err != nil {
					p.FailedCount++
					p.LastFailedTime.Time = time.Now()
					p.LastError = err.Error()
					models.GetDB().Save(&p)
					return
				}

				if !hasSign {
					signLock.Lock()
					hasSign = true
					signLock.Unlock()
					ch <- conn
					p.SuccessCount++
					p.LastSuccessTime.Time = time.Now()
					models.GetDB().Save(&p)
				}
			}(p)
		}

		select {
		case <-ctx.Done():
			return nil, errors.New("context done")
		case conn := <-ch:
			return conn, nil
		case <-time.After(defaultTimeOut):
		}
		return nil, errors.New("no alive proxy")
	}

	proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		if resp != nil {
			log.Println(resp.StatusCode, resp.Request.URL, resp.Request.RequestURI)
		}
		return resp
	})
	proxy.ServeHTTP(c.Writer, c.Request)
}
