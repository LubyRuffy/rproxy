package checkproxy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/patrickmn/go-cache"
	"h12.io/socks"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	Version = "v0.1.6"

	defaultTimeOut = time.Second * 15

	defaultCheckUrl    = "http://ip.bmh.im/h"
	defaultCheckHeader = "Rproxy"
	defaultCheckFunc   = func(resp *http.Response) (r interface{}, err error) {
		var rs respStruct
		if resp.StatusCode != http.StatusOK {
			return nil, errors.New(headerString(resp))
		}

		if err = json.NewDecoder(resp.Body).Decode(&rs); err != nil {
			return
		}

		if strings.Contains(rs.Header, defaultCheckHeader) {
			return &rs, nil
		}

		return nil, errors.New(headerString(resp))
	}

	defaultHTTPsCheckUrl  = "https://p.bmh.im"
	defaultHTTPsCheckFunc = func(resp *http.Response) bool {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return false
		}
		if strings.Contains(string(b), "p.bmh.im") && strings.Contains(string(b), "Invalid URL") {
			return true
		}
		return false
	}

	globalCache = cache.New(1*time.Hour, 10*time.Minute) // 全局缓存
)

type respStruct struct {
	Header   string                 `json:"header"`
	Ip       string                 `json:"ip"`
	Upstream string                 `json:"upstream"`
	Geo      map[string]interface{} `json:"geo"`
}

func defaultHttpClient(tr *http.Transport) *http.Client {
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	tr.TLSHandshakeTimeout = defaultTimeOut
	tr.ResponseHeaderTimeout = defaultTimeOut
	tr.IdleConnTimeout = defaultTimeOut
	tr.ExpectContinueTimeout = defaultTimeOut
	return &http.Client{
		Transport: tr,
		Timeout:   defaultTimeOut,
	}
}

// TransportFunc takes an address to a proxy server and returns a fully
// populated http Transport
type TransportFunc func(addr string) *http.Transport

// Transports is a map of proxy TransportFuncs keyed by their protocol
var Transports = map[string]TransportFunc{
	"http": func(addr string) *http.Transport {
		u, _ := url.Parse("http://" + addr)
		return &http.Transport{
			Proxy: http.ProxyURL(u),
		}
	},
	"https": func(addr string) *http.Transport {
		u, _ := url.Parse("https://" + addr)
		return &http.Transport{
			Proxy: http.ProxyURL(u),
		}
	},
	"socks4": func(addr string) *http.Transport {
		return &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return socks.Dial("socks4://"+addr)("socks4", addr)
		},
		}
	},
	"socks4a": func(addr string) *http.Transport {
		return &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return socks.Dial("socks4a://"+addr)("socks4a", addr)
		},
		}
	},
	"socks5": func(addr string) *http.Transport {
		u, _ := url.Parse("socks5://" + addr)
		return &http.Transport{
			Proxy: http.ProxyURL(u),
		}
	},
}

func SupportHttps(uParsed *url.URL) bool {
	var err error
	if transportFunc, ok := Transports[uParsed.Scheme]; ok {
		client := defaultHttpClient(transportFunc(uParsed.Host))

		var resp *http.Response
		resp, err = client.Get(defaultHTTPsCheckUrl)
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		return defaultHTTPsCheckFunc(resp)
	}
	return false
}

// checkProtocolHost 第一个返回参数是是否为代理，第二个返回参数是返回的header
func checkProtocolHost(protocol string, host string, afterCallback func(string, string, string)) *ProxyResult {
	// 确定最近没有进行测试
	id := protocol + "://" + host
	if _, found := globalCache.Get(id); found {
		return nil
	} else {
		globalCache.Set(id, true, cache.DefaultExpiration)
	}

	var err error
	defer func() {
		if afterCallback != nil {
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			afterCallback(protocol, host, errStr)
		}

	}()

	if transportFunc, ok := Transports[protocol]; ok {
		client := defaultHttpClient(transportFunc(host))

		var req *http.Request
		req, err = http.NewRequest("GET", defaultCheckUrl, nil)
		if err != nil {
			log.Println("check host failed, host:", host, ", new request err:", err)
			return &ProxyResult{Error: err}
		}
		req.Header.Set(defaultCheckHeader, Version)

		var resp *http.Response

		startTime := time.Now()
		if resp, err = client.Do(req); err == nil {
			cost := time.Since(startTime)
			defer resp.Body.Close()

			var rs interface{}
			rs, err = defaultCheckFunc(resp)
			if err != nil {
				log.Println("check host failed, host:", host, ", decode err:", err)
				return &ProxyResult{Error: err}
			}

			if len(rs.(*respStruct).Ip) > 0 {
				proxyUrl := fmt.Sprintf("%s://%s", protocol, host)
				parsedUrl, _ := url.Parse(proxyUrl)
				supportConnect := true
				if protocol[:4] == "http" {
					supportConnect = SupportHttps(parsedUrl)
				}

				// 提取端口
				port := -1
				if portStr := parsedUrl.Port(); len(portStr) > 0 {
					port, err = strconv.Atoi(portStr)
					if err != nil {
						return nil
					}
				}
				if port == -1 {
					switch protocol {
					case "http":
						port = 80
					case "https":
						port = 443
					case "socks4", "socks5":
						port = 3128
					}
				}

				// 判断等级
				header := resp.Header.Clone()
				proxyLevel := ProxyAnonymityElite
				for _, key := range []string{"Via", "X-Forwarded-For", "X-RealIP", "X-RealIp"} {
					if v := header.Get(key); len(v) > 0 {
						proxyLevel = ProxyAnonymityAnonymous
						break
					}
				}
				for key := range header {
					if v := header.Get(key); len(v) > 0 {
						if ContainsPublicIP(v) {
							proxyLevel = ProxyAnonymityTransparent
							break
						}
					}
				}
				if ContainsPublicIP(rs.(*respStruct).Upstream) {
					proxyLevel = ProxyAnonymityTransparent
				}

				return &ProxyResult{
					Valid:          true,
					Header:         header,
					IP:             rs.(*respStruct).Ip,
					Port:           port,
					Geo:            rs.(*respStruct).Geo,
					Upstream:       rs.(*respStruct).Upstream,
					Cost:           cost,
					Url:            proxyUrl,
					SupportConnect: supportConnect,
					UrlParsed:      parsedUrl,
					ProxyLevel:     proxyLevel,
				}
			}
		}
	}
	return &ProxyResult{}
}

// checkHost 第一个返回是代理的完整url，第二个返回是header
func checkHost(host string, afterCallback func(string, string, string)) *ProxyResult {
	if strings.Contains(host, "://") {
		return checkUrl(host, afterCallback)
	}

	if host == "" {
		return nil
	}
	if !strings.Contains(host, ":") {
		return checkProtocolHost("http", host+":80", afterCallback)
	}
	if strings.Contains(host, "443") {
		if p := checkProtocolHost("https", host, afterCallback); p != nil && p.Valid {
			return p
		}
	} else {
		for _, protocol := range []string{
			"http",
			"socks5",
			"https",
			//"socks4",
		} {
			if p := checkProtocolHost(protocol, host, afterCallback); p != nil && p.Valid {
				return p
			}
		}
	}

	return nil
}

// checkUrl 第一个返回参数是是否为代理，第二个返回参数是返回的header
func checkUrl(u string, afterCallback func(string, string, string)) *ProxyResult {
	uParsed, err := url.Parse(u)
	if err != nil {
		log.Println("check url failed, url:", u, ", url parse err:", err)
		return &ProxyResult{Error: err}
	}
	if uParsed.Host == "" {
		log.Println("check url failed, url is invalid")
		return &ProxyResult{Error: errors.New("url is invalid")}
	}

	if uParsed.Scheme == "" {
		uParsed.Scheme = "http"
	}

	protocol := strings.ToLower(uParsed.Scheme)
	return checkProtocolHost(protocol, uParsed.Host, afterCallback)
}

// CheckUrl 根据url进行测试
func CheckUrl(u string, afterCallback func(string, string, string)) *ProxyResult {
	return checkUrl(u, afterCallback)
}

// CheckHost 根据host进行测试
func CheckHost(host string, afterCallback func(string, string, string)) *ProxyResult {
	return checkHost(host, afterCallback)
}

// CheckIpPort 根据ip和端口进行测试
func CheckIpPort(ip string, port string, afterCallback func(string, string, string)) *ProxyResult {
	return checkHost(fmt.Sprintf("%s:%s", ip, port), afterCallback)
}
