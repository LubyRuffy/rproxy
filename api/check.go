package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/LubyRuffy/myip/ipdb"
	"github.com/LubyRuffy/rproxy/models"
	"github.com/gin-gonic/gin"
	"h12.io/socks"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	once               sync.Once
	myPublicIP         string // 公网ip，用于检查代理是否匿名
	defaultCheckUrl    = "http://ip.bmh.im/h"
	defaultCheckHeader = "Rproxy"
	defaultCheckFunc   = func(resp *http.Response) (header string, err error) {
		var r struct {
			Header string `json:"header"`
			Ip     string `json:"ip"`
		}
		//body, err := ioutil.ReadAll(resp.Body)
		//if err != nil {
		//	return false, err
		//}
		//log.Println(string(body))
		if err = json.NewDecoder(resp.Body).Decode(&r); err != nil {
			return
		}

		if strings.Contains(r.Header, defaultCheckHeader) {
			return r.Header, nil
		}

		return
	}

	defaultHTTPsCheckUrl  = "https://p.bmh.im"
	defaultHTTPsCheckFunc = func(resp *http.Response) bool {
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return false
		}
		if strings.Contains(string(b), "p.bmh.im") && strings.Contains(string(b), "Invalid URL") {
			return true
		}
		return false
	}
)

// TransportFunc takes an address to a proxy server and returns a fully
// populated http Transport
type TransportFunc func(addr string) *http.Transport

// Transports is a map of proxy TransportFuncs keyed by their protocol
var Transports = map[string]TransportFunc{
	"http": func(addr string) *http.Transport {
		u, _ := url.Parse("http://" + addr)
		return &http.Transport{Proxy: http.ProxyURL(u), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	},
	"https": func(addr string) *http.Transport {
		u, _ := url.Parse("https://" + addr)
		return &http.Transport{Proxy: http.ProxyURL(u), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	},
	"socks4": func(addr string) *http.Transport {
		return &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return socks.Dial("socks4://"+addr)("socks4", addr)
		}, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	},
	"socks4a": func(addr string) *http.Transport {
		return &http.Transport{DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return socks.Dial("socks4a://"+addr)("socks4a", addr)
		}, TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	},
	"socks5": func(addr string) *http.Transport {
		u, _ := url.Parse("socks5://" + addr)
		return &http.Transport{Proxy: http.ProxyURL(u), TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	},
}

type proxyResult struct {
	Valid  bool
	Cost   time.Duration
	Error  error
	Header http.Header
	Url    string
}

// checkProtocolHost 第一个返回参数是是否为代理，第二个返回参数是返回的header
func checkProtocolHost(protocol string, host string) *proxyResult {
	var err error
	defer func() {
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		models.GetDB().Save(&models.CheckLog{
			ProxyType: protocol,
			Host:      host,
			Error:     errStr,
		})
	}()

	if transportFunc, ok := Transports[protocol]; ok {
		client := http.Client{
			Transport: transportFunc(host),
		}

		var req *http.Request
		req, err = http.NewRequest("GET", defaultCheckUrl, nil)
		if err != nil {
			log.Println("check host failed, host:", host, ", new request err:", err)
			return &proxyResult{Error: err}
		}
		req.Header.Set(defaultCheckHeader, Version)

		var resp *http.Response

		startTime := time.Now()
		if resp, err = client.Do(req); err == nil {
			cost := time.Since(startTime)
			defer resp.Body.Close()

			var header string
			header, err = defaultCheckFunc(resp)
			if err != nil {
				log.Println("check host failed, host:", host, ", decode err:", err)
				return &proxyResult{Error: err}
			}
			if len(header) > 0 {
				return &proxyResult{
					Valid:  true,
					Header: resp.Header.Clone(),
					Cost:   cost,
					Url:    fmt.Sprintf("%s://%s", protocol, host),
				}
			}
		}
	}
	return &proxyResult{}
}

// checkUrl 第一个返回参数是是否为代理，第二个返回参数是返回的header
func checkUrl(u string) *proxyResult {
	uParsed, err := url.Parse(u)
	if err != nil {
		log.Println("check url failed, url:", u, ", url parse err:", err)
		return &proxyResult{Error: err}
	}
	if uParsed.Host == "" {
		log.Println("check url failed, url is invalid")
		return &proxyResult{Error: errors.New("url is invalid")}
	}

	if uParsed.Scheme == "" {
		uParsed.Scheme = "http"
	}

	protocol := strings.ToLower(uParsed.Scheme)
	return checkProtocolHost(protocol, uParsed.Host)
}

// checkHost 第一个返回是代理的完整url，第二个返回是header
func checkHost(host string) *proxyResult {
	if !strings.Contains(host, ":") {
		return checkProtocolHost("http", host+":80")
	}
	if strings.Contains(host, "443") {
		if p := checkProtocolHost("https", host); p.Valid {
			return p
		}
	} else {
		for _, protocol := range []string{
			"http",
			"socks5",
			"https",
			//"socks4",
		} {
			if p := checkProtocolHost(protocol, host); p.Valid {
				return p
			}
		}
	}

	return nil
}

// checkIpPort 第一个返回是代理的完整url，第二个返回是header
func checkIpPort(ip string, port string) *proxyResult {
	return checkHost(fmt.Sprintf("%s:%s", ip, port))
}

func supportHttps(uParsed *url.URL) bool {
	var err error
	if transportFunc, ok := Transports[uParsed.Scheme]; ok {
		client := http.Client{
			Transport: transportFunc(uParsed.Host),
		}

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

// header 可以是空
func checkProxyOfUrl(u string, checkResult *proxyResult) *models.Proxy {
	uParsed, err := url.Parse(u)
	port := -1
	if portStr := uParsed.Port(); len(portStr) > 0 {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return nil
		}
	}
	if port == -1 {
		switch uParsed.Scheme {
		case "http":
			port = 80
		case "https":
			port = 443
		case "socks4", "socks5":
			port = 3128
		}
	}

	// 判断等级
	proxyLevel := models.ProxyAnonymityElite
	for _, key := range []string{"Via", "X-Forwarded-For", "X-RealIP", "X-RealIp"} {
		if v := checkResult.Header.Get(key); len(v) > 0 {
			proxyLevel = models.ProxyAnonymityAnonymous
		}
	}
	for key := range checkResult.Header {
		if v := checkResult.Header.Get(key); len(v) > 0 {
			if strings.Contains(v, myPublicIP) {
				proxyLevel = models.ProxyAnonymityTransparent
			}
		}
	}

	// 是否支持connect
	supportConnect := supportHttps(uParsed)

	// country
	country := ""
	if ipdb.Get() != nil {
		ip := net.ParseIP(uParsed.Host)
		if ip == nil {
			ips, err := net.LookupIP(uParsed.Host)
			if err == nil && len(ips) > 0 {
				ip = ips[0]
			}
		}
		if ip != nil {
			city, err := ipdb.Get().City(ip)
			if err == nil && city != nil {
				country = city.Country.IsoCode
			}
		}
	}

	return &models.Proxy{
		IP:           uParsed.Hostname(),
		Port:         port,
		ProxyType:    uParsed.Scheme,
		ProxyURL:     u,
		Http:         true,
		Connect:      supportConnect,
		Country:      country,
		ProxyLevel:   proxyLevel,
		Latency:      checkResult.Cost.Milliseconds(),
		SuccessCount: 0,
		FailedCount:  0,
	}
}

// GetPublicIP 获取公网IP列表
func GetPublicIP() string {
	once.Do(func() {
		resp, err := http.Get("https://stat.ripe.net/data/whats-my-ip/data.json")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not get IPs: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()
		var r struct {
			Data struct {
				IP string `json:"ip"`
			} `json:"data"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&r); err != nil {
			fmt.Fprintf(os.Stderr, "Could not get IPs: %v\n", err)
			os.Exit(1)
		}
		myPublicIP = r.Data.IP
	})

	return myPublicIP
}

func checkHandler(c *gin.Context) {
	var proxyUrl string
	var checkResult *proxyResult
	if proxyUrl = c.Query("url"); len(proxyUrl) > 0 {
		// ?url=https://1.1.1.1:443
		if checkResult = checkUrl(proxyUrl); checkResult == nil || !checkResult.Valid {
			c.JSON(200, map[string]interface{}{
				"code":    500,
				"message": fmt.Sprintf("not valid proxy of url: %s", proxyUrl),
			})
			return
		}
	} else if host := c.Query("host"); len(host) > 0 {
		if strings.Contains(host, "://") {
			if checkResult = checkUrl(host); checkResult == nil || !checkResult.Valid {
				c.JSON(200, map[string]interface{}{
					"code":    500,
					"message": fmt.Sprintf("not valid proxy of url: %s", proxyUrl),
				})
				return
			}
		} else {
			// ?host=1.1.1.1:80
			if checkResult = checkHost(host); checkResult == nil || !checkResult.Valid {
				c.JSON(200, map[string]interface{}{
					"code":    500,
					"message": fmt.Sprintf("not valid proxy of host: %s", host),
				})
				return
			}

		}
		proxyUrl = checkResult.Url

	} else if port := c.Query("port"); len(port) > 0 {
		// ?ip=1.1.1.1&port=80
		ip := c.Query("ip")
		if checkResult = checkIpPort(ip, port); checkResult == nil || !checkResult.Valid {
			c.JSON(200, map[string]interface{}{
				"code":    500,
				"message": fmt.Sprintf("not valid proxy of ip/port : [%s:%s]", ip, port),
			})
			return
		}
		proxyUrl = checkResult.Url
	} else {
		c.JSON(500, errors.New("param failed"))
		return
	}

	if proxyUrl == "" {
		c.JSON(500, errors.New("not proxy"))
		return
	}

	p := checkProxyOfUrl(proxyUrl, checkResult)
	if p == nil {
		c.JSON(200, map[string]interface{}{
			"code":    500,
			"message": fmt.Sprintf("not valid proxy of url: %s", proxyUrl),
		})
		return
	}

	var findProxy models.Proxy
	if err := models.GetDB().Where(models.Proxy{ProxyURL: proxyUrl}).Find(&findProxy).Error; err == nil {
		p.ID = findProxy.ID
	}

	if err := models.GetDB().Where(models.Proxy{ProxyURL: proxyUrl}).Save(p).Error; err != nil {
		log.Println("[WARNING] save proxy failed, url:", proxyUrl, ", err:", err)
	}

	c.JSON(200, map[string]interface{}{
		"code": 200,
		"data": "ok",
	})
}
