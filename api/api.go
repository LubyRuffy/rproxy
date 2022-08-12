package api

import (
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"strings"
)

var (
	srv         *http.Server // http服务器
	Version     = "v0.1.1"
	Prefix      = "/api"
	authUserKey = "token" // 存在context中的token主键

	// TokenAuth 认证函数，可以覆盖
	TokenAuth = func(token string) bool {
		return true
	}
)

func statusHandler(c *gin.Context) {
	//c.JSON() c.IndentedJSON()
	c.JSON(200, map[string]interface{}{
		"status":  "ok",
		"version": Version,
	})
}

func agentTokenAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if authLine := c.Request.Header.Get("Authorization"); authLine != "" {
			// token xxx
			auth := strings.SplitN(authLine, " ", 2)
			if len(auth) == 2 && TokenAuth(auth[1]) {
				c.Set(authUserKey, auth[1])
				return
			}
		}

		if TokenAuth("") {
			c.Set(authUserKey, "")
			return
		}

		c.AbortWithStatusJSON(403, map[string]interface{}{
			"code":    403,
			"message": "invalid auth",
		})
	}
}

func meHandler(c *gin.Context) {
	c.String(200, "%s", c.GetString(authUserKey))
}

// Start 启动服务器
func Start(addr string) error {
	if viper.GetBool("debug.gin") {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 检查公网IP
	GetPublicIP()

	router := gin.Default()
	pprof.Register(router, "dev/pprof") // http pprof, default is "debug/pprof"
	router.GET("/status", statusHandler)
	router.Any("/", proxyHandler)

	v1 := router.Group(Prefix+"/v1", agentTokenAuth())
	v1.GET("/me", meHandler)
	v1.GET("/check", checkHandler)
	v1.GET("/list", listHandler)

	log.Println("api server listened at:", addr)
	//return router.Run(addr)

	srv = &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				proxyServeHTTP(w, r)
			} else {
				router.ServeHTTP(w, r)
			}
		}),
	}
	return srv.ListenAndServe()
}

// Stop 停止服务器
func Stop() error {
	return srv.Close()
}
