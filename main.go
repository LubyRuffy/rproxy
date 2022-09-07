package main

import (
	"fmt"
	"github.com/LubyRuffy/myip/ipdb"
	"github.com/LubyRuffy/rproxy/api"
	"github.com/LubyRuffy/rproxy/models"
	"github.com/spf13/viper"
	"log"
	"os"
	"path/filepath"
)

func main() {
	// 加载配置文件
	viper.SetDefault("addr", ":8088")
	viper.SetDefault("dbfile", "rproxy.sqlite")
	viper.SetDefault("debug.dbsql", false)
	viper.AddConfigPath(filepath.Dir(os.Args[0]))
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")
	if err := viper.ReadInConfig(); err == nil {
		log.Println("load config from file:", viper.ConfigFileUsed())
	}

	// 连接数据库
	_, err := models.SetupDB(fmt.Sprintf("%s?journal_mode=%s&busy_timeout=%s", viper.GetString("dbfile"), "WAL", "9999999"))
	if err != nil {
		log.Println("connect db failed:", err)
	}

	// 检查数据库
	go ipdb.UpdateIpDatabase()

	// 启动web
	if err = api.Start(viper.GetString("addr")); err != nil {
		panic(err)
	}
}
