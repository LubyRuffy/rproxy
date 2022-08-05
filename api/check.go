package api

import (
	"errors"
	"fmt"
	"github.com/LubyRuffy/rproxy/models"
	"github.com/gin-gonic/gin"
)

func checkUrl(u string) bool {
	return false
}

func checkHost(host string) string {
	return ""
}

func checkProxyOfUrl(u string) *models.Proxy {
	return &models.Proxy{
		IP:           "",
		Port:         0,
		ProxyType:    "",
		ProxyURL:     u,
		Http:         false,
		Connect:      false,
		ProxyLevel:   0,
		SuccessCount: 0,
		FailedCount:  0,
	}
}

func checkHandler(c *gin.Context) {
	var proxyUrl string
	if proxyUrl = c.Query("url"); len(proxyUrl) > 0 {
		if !checkUrl(proxyUrl) {
			c.JSON(200, map[string]interface{}{
				"code":    500,
				"message": fmt.Sprintf("not valid proxy of url: %s", proxyUrl),
			})
			return
		}
	} else if host := c.Query("host"); len(host) > 0 {
		if proxyUrl = checkHost(host); len(proxyUrl) == 0 {
			c.JSON(200, map[string]interface{}{
				"code":    500,
				"message": fmt.Sprintf("not valid proxy of host: %s", host),
			})
			return
		}
	} else {
		c.JSON(500, errors.New("param failed"))
		return
	}

	p := checkProxyOfUrl(proxyUrl)
	if p == nil {
		c.JSON(200, map[string]interface{}{
			"code":    500,
			"message": fmt.Sprintf("not valid proxy of url: %s", proxyUrl),
		})
		return
	}

	c.JSON(200, map[string]interface{}{
		"code": 200,
		"data": "ok",
	})
}
