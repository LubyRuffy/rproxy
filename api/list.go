package api

import (
	"fmt"
	"github.com/LubyRuffy/rproxy/models"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func listHandler(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	sizeStr := c.DefaultQuery("size", "10")
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	var proxyList []models.Proxy
	if err = models.GetDB().Offset((page - 1) * size).Limit(size).Order("updated_at desc").Find(&proxyList).Error; err != nil {
		c.JSON(200, map[string]interface{}{
			"code":    500,
			"message": fmt.Sprintf("proxy list failed: %v", err),
		})
		return
	}

	c.JSON(200, map[string]interface{}{
		"code": 200,
		"data": map[string]interface{}{
			"lists": proxyList,
			"page":  page,
			"size":  size,
			//"total": count,
		},
	})
}
