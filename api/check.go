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

// checkProtocolHost 第一个返回参数是是否为代理，第二个返回参数是返回的header
func checkProtocolHost(protocol string, host string) (bool, http.Header) {
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
			return false, nil
		}
		req.Header.Set(defaultCheckHeader, Version)

		var resp *http.Response
		if resp, err = client.Do(req); err == nil {
			defer resp.Body.Close()

			var header string
			header, err = defaultCheckFunc(resp)
			if err != nil {
				log.Println("check host failed, host:", host, ", decode err:", err)
				return false, nil
			}
			if len(header) > 0 {
				return true, resp.Header.Clone()
			}
		}
	}
	return false, nil
}

// checkUrl 第一个返回参数是是否为代理，第二个返回参数是返回的header
func checkUrl(u string) (bool, http.Header) {
	uParsed, err := url.Parse(u)
	if err != nil {
		log.Println("check url failed, url:", u, ", url parse err:", err)
		return false, nil
	}

	protocol := strings.ToLower(uParsed.Scheme)
	return checkProtocolHost(protocol, uParsed.Host)
}

// checkHost 第一个返回是代理的完整url，第二个返回是header
func checkHost(host string) (string, http.Header) {
	if strings.Contains(host, "443") {
		if ok, header := checkProtocolHost("https", host); ok {
			return fmt.Sprintf("%s://%s", "https", host), header
		}
	} else {
		for _, protocol := range []string{"http", "socks5", "https", "socks4"} {
			if ok, header := checkProtocolHost(protocol, host); ok {
				return fmt.Sprintf("%s://%s", protocol, host), header
			}
		}
	}

	return "", nil
}

// checkIpPort 第一个返回是代理的完整url，第二个返回是header
func checkIpPort(ip string, port string) (string, http.Header) {
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
func checkProxyOfUrl(u string, header http.Header) *models.Proxy {
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
		if v := header.Get(key); len(v) > 0 {
			proxyLevel = models.ProxyAnonymityAnonymous
		}
	}
	for key := range header {
		if v := header.Get(key); len(v) > 0 {
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
		var record map[string]interface{}
		ip := net.ParseIP(uParsed.Host)
		if ip == nil {
			ips, err := net.LookupIP(uParsed.Host)
			if err == nil && len(ips) > 0 {
				ip = ips[0]
			}
		}
		if ip != nil {
			err = ipdb.Get().Lookup(ip, &record)
			if err == nil {
				country = record["country"].(map[string]interface{})["iso_code"].(string)
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
		SuccessCount: 0,
		FailedCount:  0,
	}
}

// GetPublicIP 获取公网IP列表
func GetPublicIP() string {
	once.Do(func() {
		resp, err := http.Get("https://httpbin.org/get")
		//ips, err := net.LookupIP(defaultPublicHost)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not get IPs: %v\n", err)
			os.Exit(1)
		}
		//for _, ip := range ips {
		//	myPublicIP = append(myPublicIP, ip.String())
		//	//fmt.Printf("google.com. IN A %s\n", ip.String())
		//}
		defer resp.Body.Close()
		var r struct {
			Origin string `json:"origin"`
		}
		if err = json.NewDecoder(resp.Body).Decode(&r); err != nil {
			fmt.Fprintf(os.Stderr, "Could not get IPs: %v\n", err)
			os.Exit(1)
		}
		myPublicIP = r.Origin
	})

	return myPublicIP
}

func checkHandler(c *gin.Context) {
	var proxyUrl string
	var isProxy bool
	var header http.Header
	if proxyUrl = c.Query("url"); len(proxyUrl) > 0 {
		// ?url=https://1.1.1.1:443
		if isProxy, header = checkUrl(proxyUrl); !isProxy {
			c.JSON(200, map[string]interface{}{
				"code":    500,
				"message": fmt.Sprintf("not valid proxy of url: %s", proxyUrl),
			})
			return
		}
	} else if host := c.Query("host"); len(host) > 0 {
		// ?host=1.1.1.1:80
		if proxyUrl, header = checkHost(host); len(proxyUrl) == 0 {
			c.JSON(200, map[string]interface{}{
				"code":    500,
				"message": fmt.Sprintf("not valid proxy of host: %s", host),
			})
			return
		}
	} else if port := c.Query("port"); len(port) > 0 {
		// ?ip=1.1.1.1&port=80
		ip := c.Query("ip")
		if proxyUrl, header = checkIpPort(ip, port); len(proxyUrl) == 0 {
			c.JSON(200, map[string]interface{}{
				"code":    500,
				"message": fmt.Sprintf("not valid proxy of ip/port : [%s:%s]", ip, port),
			})
			return
		}
	} else {
		c.JSON(500, errors.New("param failed"))
		return
	}

	p := checkProxyOfUrl(proxyUrl, header)
	if p == nil {
		c.JSON(200, map[string]interface{}{
			"code":    500,
			"message": fmt.Sprintf("not valid proxy of url: %s", proxyUrl),
		})
		return
	}

	if err := models.GetDB().Where(models.Proxy{ProxyURL: proxyUrl}).FirstOrInit(p).Save(p).Error; err != nil {
		log.Println("[WARNING] save proxy failed, url:", proxyUrl, ", err:", err)
	}

	c.JSON(200, map[string]interface{}{
		"code": 200,
		"data": "ok",
	})
}
