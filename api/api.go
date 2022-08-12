package api

import (
	"github.com/LubyRuffy/rproxy/models"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"log"
	"net/http"
)

var (
	srv         *http.Server // http服务器
	Version     = "v0.1.1"
	Prefix      = "/api"
	authUserKey = "token" // 存在context中的token主键

	// TokenAuth 认证函数，可以覆盖
	TokenAuth = func(c *gin.Context, token string) bool {
		var user models.User
		if err := models.GetDB().Find(&user, "token=?", token).Error; err == nil && user.ID > 0 {
			log.Println(user.Email, "auth ok")
			c.Set(authUserKey, user.Email)
			return true
		}
		return false
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
		if viper.GetBool("auth") {
			if authLine := c.Request.Header.Get("X-Rproxy-Token"); authLine != "" {
				if len(authLine) > 0 && TokenAuth(c, authLine) {
					return
				}
			}

			c.AbortWithStatusJSON(403, map[string]interface{}{
				"code":    403,
				"message": "invalid auth",
			})
		}
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
	router.GET("/check", checkHandler)
	router.Any("/", proxyHandler)

	v1 := router.Group(Prefix+"/v1", agentTokenAuth())
	v1.GET("/me", meHandler)
	v1.GET("/list", listHandler)

	log.Println("api server listened at:", addr)
	//return router.Run(addr)

	srv = &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			//if r.Method == http.MethodConnect {
			if len(r.RequestURI) > 0 && r.RequestURI[0] != '/' {
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
