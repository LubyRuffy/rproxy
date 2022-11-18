package api

import (
	"errors"
	"fmt"
	"github.com/LubyRuffy/myip/ipdb"
	"github.com/LubyRuffy/rproxy/checkproxy"
	"github.com/LubyRuffy/rproxy/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
	"log"
	"net"
	"strings"
	"time"
)

var (
	EnableErrorCheckLog bool // 是否启用错误日志：在检查失败的情况下也记录日志
)

func afterCallback(protocol string, host string, errStr string) {
	if EnableErrorCheckLog {
		models.GetDB().Create(&models.CheckLog{
			ProxyType: protocol,
			Host:      host,
			Error:     errStr,
		})
	}
}

// checkProxyOfUrl 检查代理的一些属性
func checkProxyOfUrl(checkResult *checkproxy.ProxyResult) *models.Proxy {

	// country
	country := ""
	if ipdb.Get() != nil {
		ip := net.ParseIP(checkResult.IP)
		if ip == nil {
			ips, err := net.LookupIP(checkResult.UrlParsed.Hostname())
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
	} else if checkResult.Geo != nil {
		if v, ok := checkResult.Geo["country"]; ok {
			country = v.(string)
		}

	}

	return &models.Proxy{
		IP:           checkResult.UrlParsed.Hostname(),
		OutIP:        checkResult.IP,
		Port:         checkResult.Port,
		ProxyType:    checkResult.UrlParsed.Scheme,
		ProxyURL:     checkResult.Url,
		Http:         true,
		Connect:      checkResult.SupportConnect,
		IPv6:         strings.Count(checkResult.IP, ":") >= 2,
		Country:      country,
		ProxyLevel:   checkResult.ProxyLevel,
		Latency:      checkResult.Cost.Milliseconds(),
		SuccessCount: 0,
		FailedCount:  0,
	}
}

// insertProxyToDb 插入代理表，如果有用户信息，也要插入关联表
func insertProxyToDb(p *models.Proxy, uid uint) error {
	var findProxy models.Proxy
	if err := models.GetDB().Where(models.Proxy{ProxyURL: p.ProxyURL}).Find(&findProxy).Error; err == nil {
		p.ID = findProxy.ID
		p.CreatedAt = findProxy.CreatedAt
		p.SuccessCount = findProxy.SuccessCount + 1
		p.FailedCount = findProxy.FailedCount
		p.LastError = findProxy.LastError
		p.LastFailedTime = findProxy.LastFailedTime
		p.LastSuccessTime.Time = time.Now()
		p.LastSuccessTime.Valid = true
	}

	if err := models.GetDB().Where(models.Proxy{ProxyURL: p.ProxyURL}).Save(p).Error; err != nil {
		return err
	}

	// 写入关系表
	if uid > 0 {
		if err := models.GetDB().Clauses(clause.OnConflict{DoNothing: true}).Create(&models.UserProxy{
			UserID:  uid,
			ProxyID: p.ID,
		}).Error; err != nil {
			return err
		}
	}

	return nil
}

// fillProxyField 填充代理属性
func fillProxyField(checkResult *checkproxy.ProxyResult, uid uint) {
	p := checkProxyOfUrl(checkResult)
	if p == nil {
		return
	}

	if err := insertProxyToDb(p, uid); err != nil {
		log.Println("[WARNING] save user proxy failed, url:", checkResult.Url, ", err:", err)
	}
}

func checkHostAndInsertDB(host string, uid uint) {
	checkResult := checkproxy.CheckHost(host, afterCallback)
	if checkResult == nil || !checkResult.Valid {
		return
	}
	fillProxyField(checkResult, uid)
}

func checkHandler(c *gin.Context) {
	var checkResult *checkproxy.ProxyResult
	if proxyUrl := c.Query("url"); len(proxyUrl) > 0 {
		// ?url=https://1.1.1.1:443
		if checkResult = checkproxy.CheckUrl(proxyUrl, afterCallback); checkResult == nil || !checkResult.Valid {
			c.JSON(200, map[string]interface{}{
				"code":    500,
				"message": fmt.Sprintf("not valid proxy of url: %s", proxyUrl),
			})
			return
		}
	} else if host := c.Query("host"); len(host) > 0 {
		if strings.Contains(host, "://") {
			if checkResult = checkproxy.CheckUrl(host, afterCallback); checkResult == nil || !checkResult.Valid {
				c.JSON(200, map[string]interface{}{
					"code":    500,
					"message": fmt.Sprintf("not valid proxy of url: %s", proxyUrl),
				})
				return
			}
		} else {
			// ?host=1.1.1.1:80

			// 遍历协议时间比较长，都放到后台进行
			for i := 0; i < 3 && wp.WaitingQueueSize() > wp.Size(); i++ {
				time.Sleep(time.Second)
			}
			wp.Submit(func() {
				checkHostAndInsertDB(host, userId(c))
			})

			// 直接返回成功提示
			c.JSON(200, map[string]interface{}{
				"code": 200,
				"data": "submit ok, process in the background",
			})
			return

		}
	} else if port := c.Query("port"); len(port) > 0 {
		// ?ip=1.1.1.1&port=80
		ip := c.Query("ip")
		if checkResult = checkproxy.CheckIpPort(ip, port, afterCallback); checkResult == nil || !checkResult.Valid {
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

	if checkResult == nil || checkResult.Url == "" {
		c.JSON(500, errors.New("not proxy"))
		return
	}

	// 只要是代理，就返回ok，其他属性放到后台执行
	wp.Submit(func() {
		fillProxyField(checkResult, userId(c))
	})

	c.JSON(200, map[string]interface{}{
		"code": 200,
		"data": "ok",
	})
}
