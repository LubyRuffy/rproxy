package api

import (
	"errors"
	"fmt"
	"github.com/LubyRuffy/gorestful"
	"github.com/LubyRuffy/rproxy/models"
	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"gorm.io/gorm"
	"log"
	"net/http"
	"strings"
)

var (
	srv         *http.Server // http服务器
	Version     = "v0.1.2"
	Prefix      = "/api"
	authUserKey = "token"  // 存在context中的token主键
	authUserId  = "userId" // 存在context中的token主键

	// TokenAuth 认证函数，可以覆盖
	TokenAuth = func(c *gin.Context, token string) bool {
		var user models.User
		if err := models.GetDB().Find(&user, "token=?", token).Error; err == nil && user.ID > 0 {
			log.Println(user.Email, "auth ok")
			if c != nil {
				c.Set(authUserKey, user.Email)
				c.Set(authUserId, user.ID)
			}

			return true
		}
		return false
	}
)

// userId 获取当前登录的user id
func userId(c *gin.Context) uint {
	if v, exists := c.Get(authUserId); exists {
		return v.(uint)
	}
	return 0
}

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

func loadRestApi(router *gin.Engine) {

	type MyClaims struct {
		jwt.RegisteredClaims
		UID      uint
		Username string
		Token    string
	}

	var userInfo func(c *gin.Context) *MyClaims
	var am *gorestful.AuthMiddle
	if viper.GetBool("auth") {
		am = &gorestful.AuthMiddle{
			GetUser: func(c *gin.Context) string {
				if info := userInfo(c); info != nil {
					return info.Username
				}
				return ""
			},
			AuthMode: &gorestful.EmbedLogin{
				OpenRegister: true,
				Register: func(c *gin.Context, e *gorestful.EmbedLogin, formMap map[string]string) error {
					var user models.User
					err := mapstructure.Decode(formMap, &user)
					if err != nil {
						return err
					}

					err = models.GetDB().Where(&user).Find(&user).Error
					if err == nil && user.ID > 0 {
						return errors.New("exists user")
					}
					err = models.GetDB().Model(&user).Save(&user).Error
					return err
				},
				RouterGroup: router.Group("/"),
				LoginFields: []*gorestful.LoginField{
					{
						Name:        "email",
						DisplayName: "Email",
						Type:        "text",
					},
					{
						Name:        "token",
						DisplayName: "Token",
						Type:        "text",
					},
				},
				CheckLogin: func(c *gin.Context, e *gorestful.EmbedLogin, formMap map[string]string) (string, bool) {
					var checkUser models.User
					if err := mapstructure.Decode(formMap, &checkUser); err != nil {
						return "", false
					}

					var user models.User
					if err := models.GetDB().Where(&checkUser).Find(&user).Error; err == nil && user.ID > 0 {
						log.Println(user.Email, "auth ok")

						t := jwt.NewWithClaims(jwt.SigningMethodHS512, MyClaims{
							UID:      user.ID,
							Username: user.Email,
							Token:    user.Token,
						})

						if tokenString, err := t.SignedString(e.Key); err == nil {
							return tokenString, true
						} else {
							log.Println("jwt failed:", err)
						}
						return "", false
					}

					return "", false
				},
			},
		}

		userInfo = func(c *gin.Context) *MyClaims {
			if tokenString := c.Request.Header.Get(am.HeaderKey); tokenString != "" {
				if len(am.HeaderValuePrefix) > 0 && strings.Contains(tokenString, am.HeaderValuePrefix) {
					tokenString = strings.Split(tokenString, am.HeaderValuePrefix)[1]
				}
				token, err := jwt.ParseWithClaims(tokenString, &MyClaims{}, func(token *jwt.Token) (interface{}, error) {
					// Don't forget to validate the alg is what you expect:
					if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
					}

					// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
					return am.AuthMode.(*gorestful.EmbedLogin).Key, nil
				})
				if err == nil {
					if claims, ok := token.Claims.(*MyClaims); ok && token.Valid {
						//log.Println(claims.Username)
						return claims
					}
				}
				log.Println("login failed:", err)
			}
			return nil
		}
	}

	res, err := gorestful.NewResource(
		gorestful.WithGinEngine(router),
		gorestful.WithGormDb(func(c *gin.Context) *gorm.DB {
			if uid := userId(c); uid > 0 {
				return models.GetDB().Model(&models.Proxy{}).
					Joins("join user_proxies on user_proxies.proxy_id=proxies.id").
					Where("user_proxies.proxy_id>0 and user_proxies.user_id=?", uid)
			}
			panic("not valid user")
			return models.GetDB().Model(&models.Proxy{})
		}),
		gorestful.WithUserStruct(func() interface{} {
			return &models.Proxy{}
		}),
		gorestful.WithApiRouterGroup(router.Group("/api", func(c *gin.Context) {
			if claims := userInfo(c); claims != nil {
				c.Set(authUserKey, claims.Token)
				c.Set(authUserId, claims.UID)
				return
			}

			c.AbortWithStatusJSON(403, map[string]interface{}{
				"code":    403,
				"message": "invalid auth",
			})
		})),
		gorestful.WithID("proxies.id"),
		gorestful.WithAfterInsert(func(c *gin.Context, id uint) error {
			return models.GetDB().Save(&models.UserProxy{UserID: userId(c), ProxyID: id}).Error
		}),
		gorestful.WithAuthMiddle(am))
	if err != nil {
		panic(err)
	}
	gorestful.AddResourceApiPageToGin(res)
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
	v1.GET("/check", checkHandler)

	loadRestApi(router)

	log.Println("api server listened at:", addr)
	//return router.Run(addr)

	srv = &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			//if r.Method == http.MethodConnect {
			if len(r.RequestURI) > 0 && r.RequestURI[0] != '/' {
				//agentTokenAuth()(&gin.Context{Request: r})
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
