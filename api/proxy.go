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

func proxyServeHTTP(w http.ResponseWriter, r *http.Request) {
	//if viper.GetBool("auth") {
	//	if authLine := r.Header.Get("X-Rproxy-Token"); authLine != "" {
	//		if len(authLine) == 0 || !TokenAuth(nil, authLine) {
	//			w.WriteHeader(http.StatusForbidden)
	//			return
	//		}
	//	}
	//}

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
			case "ip":
				db = db.Where(&models.Proxy{IP: vs[0]})
			case "port":
				if v, err := strconv.Atoi(vs[0]); err == nil {
					db = db.Where(&models.Proxy{Port: v})
				}
			}
		}
		r.Header.Del("X-Rproxy-Filter")
	}

	// 每次取三条测试
	var ps []models.Proxy
	limit := 3 // default limit is 3
	if v := r.Header.Get("X-Rproxy-Limit"); len(v) > 0 {
		if vl, err := strconv.Atoi(v); err == nil && vl > 0 {
			limit = vl
		}
		r.Header.Del("X-Rproxy-Limit")
	}

	if err := db.Order("RANDOM()").Limit(limit).Find(&ps).Error; err != nil || len(ps) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("no alive proxy"))
		return
	}

	// 尝试连接
	ch := make(chan *models.Proxy, 1)
	hasSign := false
	var signLock sync.Mutex
	for _, p := range ps {
		go func(p models.Proxy) {
			log.Printf("fetch %s from %s", r.RequestURI, p.ProxyURL)

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
	case <-r.Context().Done():
		return
	case p := <-ch:
		if r.Method == http.MethodConnect {
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
			}

		}
	case <-time.After(defaultTimeOut):
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("no alive proxy"))
		return
	}

	//dialFn := func(ctx context.Context, network, addr string) (net.Conn, error) {
	//	ch := make(chan net.Conn, 1)
	//	hasSign := false
	//	var signLock sync.Mutex
	//	for _, p := range ps {
	//		go func(p models.Proxy) {
	//			log.Printf("fetch %s from %s", r.RequestURI, p.ProxyURL)
	//
	//			var conn net.Conn
	//			var err error
	//
	//			switch p.ProxyType {
	//			case "socks5", "socks4":
	//				conn, err = socks.Dial(p.ProxyURL)(network, addr)
	//			case "http", "https":
	//				if r.Method == http.MethodConnect {
	//					conn, err = proxy.NewConnectDialToProxy(p.ProxyURL)(network, addr)
	//				} else {
	//					conn, err = net.Dial(network, fmt.Sprintf("%s:%d", p.IP, p.Port))
	//				}
	//			}
	//
	//			if err != nil {
	//				p.FailedCount++
	//				p.LastFailedTime.Time = time.Now()
	//				p.LastFailedTime.Valid = true
	//				p.LastError = err.Error()
	//				models.GetDB().Save(&p)
	//				return
	//			}
	//
	//			if !hasSign {
	//				signLock.Lock()
	//				hasSign = true
	//				signLock.Unlock()
	//				ch <- conn
	//				p.SuccessCount++
	//				p.LastSuccessTime.Time = time.Now()
	//				p.LastSuccessTime.Valid = true
	//				models.GetDB().Save(&p)
	//			}
	//		}(p)
	//	}
	//
	//	select {
	//	case <-ctx.Done():
	//		return nil, errors.New("context done")
	//	case conn := <-ch:
	//		return conn, nil
	//	case <-time.After(defaultTimeOut):
	//	}
	//	return nil, errors.New("no alive proxy")
	//}
	//
	//proxy.ConnectDial = func(network string, addr string) (net.Conn, error) {
	//	return dialFn(context.Background(), network, addr)
	//}
	//proxy.Tr.DialContext = dialFn

	//proxy.OnRequest().DoFunc(func(req *http.Request, pctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
	//	log.Println(req.URL, req.RequestURI)
	//	return req, nil
	//})
	//
	//proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
	//	if resp != nil {
	//		log.Println(resp.StatusCode, resp.Request.URL, resp.Request.RequestURI)
	//	}
	//	return resp
	//})
	proxy.ServeHTTP(w, r)
}

func proxyHandler(c *gin.Context) {
	proxyServeHTTP(c.Writer, c.Request)
}
