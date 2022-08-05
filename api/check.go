package api

import (
	"errors"
	"github.com/gin-gonic/gin"
)

func checkHandler(c *gin.Context) {
	if url := c.Query("url"); len(url) > 0 {

	} else if host := c.Query("host"); len(host) > 0 {

	} else {
		c.JSON(500, errors.New("param failed"))
		return
	}
	c.JSON(200, map[string]interface{}{
		"code": 200,
		"data": "ok",
	})
}
