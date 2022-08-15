package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/LubyRuffy/rproxy/models"
	"github.com/elazarl/goproxy"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"h12.io/socks"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var (
	defaultTimeOut = time.Second * 15
)

func proxyServeHTTP(w http.ResponseWriter, r *http.Request) {
	if viper.GetBool("auth") {
		if authLine := r.Header.Get("X-Rproxy-Token"); authLine != "" {
			if len(authLine) == 0 || !TokenAuth(nil, authLine) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
		}
	}

	db := models.GetDB()
	proxy := goproxy.NewProxyHttpServer()

	if r.Method == http.MethodConnect {
		db = db.Where(&models.Proxy{Connect: true})
	}

	if filter := r.Header.Get("X-Rproxy-Filter"); len(filter) > 0 {
		v, err := url.ParseQuery(filter)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		for k, vs := range v {
			switch k {
			case "type":
				db = db.Where(&models.Proxy{ProxyType: vs[0]})
			}
		}
		r.Header.Del("X-Rproxy-Filter")
	}

	// 每次取三条测试
	var ps []models.Proxy
	if err := db.Order("RANDOM()").Limit(3).Find(&ps).Error; err != nil || len(ps) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	dialFn := func(ctx context.Context, network, addr string) (net.Conn, error) {
		ch := make(chan net.Conn, 1)
		hasSign := false
		var signLock sync.Mutex
		for _, p := range ps {
			go func(p models.Proxy) {
				log.Printf("fetch %s from %s", r.RequestURI, p.ProxyURL)

				var conn net.Conn
				var err error

				switch p.ProxyType {
				case "socks5", "socks4":
					conn, err = socks.Dial(p.ProxyURL)(network, addr)
				case "http", "https":
					if r.Method == http.MethodConnect {
						conn, err = proxy.NewConnectDialToProxy(p.ProxyURL)(network, addr)
					} else {
						conn, err = net.Dial(network, fmt.Sprintf("%s:%d", p.IP, p.Port))
					}
				}

				if err != nil {
					p.FailedCount++
					p.LastFailedTime.Time = time.Now()
					p.LastFailedTime.Valid = true
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
					p.LastSuccessTime.Valid = true
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

	proxy.ConnectDial = func(network string, addr string) (net.Conn, error) {
		return dialFn(context.Background(), network, addr)
	}
	proxy.Tr.DialContext = dialFn

	proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		if resp != nil {
			log.Println(resp.StatusCode, resp.Request.URL, resp.Request.RequestURI)
		}
		return resp
	})
	proxy.ServeHTTP(w, r)
}

func proxyHandler(c *gin.Context) {
	proxyServeHTTP(c.Writer, c.Request)
}
