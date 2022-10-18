package api

import (
	"context"
	"fmt"
	"github.com/LubyRuffy/rproxy/models"
	"github.com/elazarl/goproxy"
	"github.com/gin-gonic/gin"
	"h12.io/socks"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

var (
	defaultTimeOut = time.Second * 15
)

func proxyServeHTTP(c *gin.Context) {
	// 外部做好认证

	db := models.GetDB().Model(&models.Proxy{}).
		Joins("join user_proxies on user_proxies.proxy_id=proxies.id").
		Where("user_proxies.proxy_id>0 and user_proxies.user_id=?", userId(c))
	proxy := goproxy.NewProxyHttpServer()

	if c.Request.Method == http.MethodConnect {
		db = db.Where(&models.Proxy{Connect: true})
	}

	if filter := c.Request.Header.Get("X-Rproxy-Filter"); len(filter) > 0 {
		v, err := url.ParseQuery(filter)
		if err != nil {
			c.Writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		for k, vs := range v {
			switch k {
			case "type":
				db = db.Where(&models.Proxy{ProxyType: vs[0]})
			case "ip":
				db = db.Where(&models.Proxy{IP: vs[0]})
			case "port":
				if v, err := strconv.Atoi(vs[0]); err == nil {
					db = db.Where(&models.Proxy{Port: v})
				}
			}
		}
		c.Request.Header.Del("X-Rproxy-Filter")
	}

	// 每次取三条测试
	var ps []models.Proxy
	limit := 3 // default limit is 3
	if v := c.Request.Header.Get("X-Rproxy-Limit"); len(v) > 0 {
		if vl, err := strconv.Atoi(v); err == nil && vl > 0 {
			limit = vl
		}
		c.Request.Header.Del("X-Rproxy-Limit")
	}

	if err := db.Order("RANDOM()").Limit(limit).Find(&ps).Error; err != nil || len(ps) == 0 {
		c.Writer.WriteHeader(http.StatusInternalServerError)
		c.Writer.Write([]byte("no alive proxy"))
		return
	}

	// 尝试连接
	ch := make(chan *models.Proxy, 1)
	hasSign := false
	var signLock sync.Mutex
	for _, p := range ps {
		go func(p models.Proxy) {
			log.Printf("fetch %s from %s", c.Request.RequestURI, p.ProxyURL)

			var conn net.Conn
			var err error

			conn, err = net.Dial("tcp", fmt.Sprintf("%s:%d", p.IP, p.Port))

			if err != nil {
				p.FailedCount++
				p.LastFailedTime.Time = time.Now()
				p.LastFailedTime.Valid = true
				p.LastError = err.Error()
				models.GetDB().Save(&p)
				return
			}

			defer conn.Close()

			if !hasSign {
				signLock.Lock()
				hasSign = true
				signLock.Unlock()
				ch <- &p
			}

			p.SuccessCount++
			p.LastSuccessTime.Time = time.Now()
			p.LastSuccessTime.Valid = true
			models.GetDB().Save(&p)
		}(p)
	}

	select {
	case <-c.Request.Context().Done():
		return
	case p := <-ch:
		if c.Request.Method == http.MethodConnect {
			proxy.ConnectDial = proxy.NewConnectDialToProxy(p.ProxyURL)
		} else {
			if p.ProxyType == "socks4" {
				// 需要支持吗？
				proxy.Tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
					return socks.Dial(p.ProxyURL)(network, addr)
				}
			} else {
				proxy.Tr.Proxy = func(req *http.Request) (*url.URL, error) {
					return url.Parse(p.ProxyURL)
				}
				proxy.Tr.IdleConnTimeout = time.Second * 10
			}
		}

		// 设置超时机制
		if proxy.Tr != nil {
			proxy.Tr.TLSHandshakeTimeout = defaultTimeOut
			proxy.Tr.ResponseHeaderTimeout = defaultTimeOut
			proxy.Tr.IdleConnTimeout = defaultTimeOut
			proxy.Tr.ExpectContinueTimeout = defaultTimeOut
		}
	case <-time.After(defaultTimeOut):
		c.Writer.WriteHeader(http.StatusInternalServerError)
		c.Writer.Write([]byte("no alive proxy"))
		return
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}
