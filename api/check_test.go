package api

import (
	"github.com/LubyRuffy/rproxy/models"
	"github.com/stretchr/testify/assert"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestStart_insertProxyToDb(t *testing.T) {
	// 准备数据库
	var size int64
	dbfile := filepath.Join(os.TempDir(), time.Now().Format("20060102150405.sqlite"))
	defer os.Remove(dbfile)
	db, err := models.SetupDB(dbfile)
	assert.Nil(t, err)
	assert.NotNil(t, db)

	proxyUrl := "http://127.0.0.1:8080"
	uParsed, _ := url.Parse(proxyUrl)
	port, _ := strconv.Atoi(uParsed.Port())
	proxyInfo := &models.Proxy{
		IP:           uParsed.Hostname(),
		Port:         port,
		ProxyType:    uParsed.Scheme,
		ProxyURL:     uParsed.String(),
		Http:         true,
		Connect:      true,
		Country:      "CN",
		ProxyLevel:   models.ProxyAnonymityElite,
		Latency:      10000000,
		SuccessCount: 0,
		FailedCount:  0,
	}

	// 测试不带用户信息
	err = insertProxyToDb(proxyInfo, 0)
	assert.Nil(t, err)
	// proxy表有数据，userproxy没有数据
	assert.Nil(t, models.GetDB().Model(&models.Proxy{}).Count(&size).Error)
	assert.Equal(t, int64(1), size)
	assert.Nil(t, models.GetDB().Model(&models.UserProxy{}).Count(&size).Error)
	assert.Equal(t, int64(0), size)

	// 测试带用户信息
	err = insertProxyToDb(proxyInfo, 1)
	assert.Nil(t, err)
	// proxy表有数据，userproxy也有数据
	assert.Nil(t, models.GetDB().Model(&models.Proxy{}).Count(&size).Error)
	assert.Equal(t, int64(1), size)
	assert.Nil(t, models.GetDB().Model(&models.UserProxy{}).Count(&size).Error)
	assert.Equal(t, int64(1), size)

	// 测试带用户信息，重复数据
	err = insertProxyToDb(proxyInfo, 1)
	assert.Nil(t, err)
	// proxy表有数据，userproxy也有数据
	assert.Nil(t, models.GetDB().Model(&models.Proxy{}).Count(&size).Error)
	assert.Equal(t, int64(1), size)
	assert.Nil(t, models.GetDB().Model(&models.UserProxy{}).Count(&size).Error)
	assert.Equal(t, int64(1), size)
}
