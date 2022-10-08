package main

import (
	"fmt"
	"github.com/LubyRuffy/myip/ipdb"
	"github.com/LubyRuffy/rproxy/api"
	"github.com/LubyRuffy/rproxy/models"
	"github.com/spf13/pflag"
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

	pflag.Bool("auth", false, "enable auth")
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	viper.AddConfigPath(filepath.Dir(os.Args[0]))
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")
	viper.Debug()
	if err := viper.ReadInConfig(); err == nil {
		log.Println("load config from file:", viper.ConfigFileUsed())
	}

	// 连接数据库
	_, err := models.SetupDB(fmt.Sprintf("%s?cache=shared&_journal_mode=WAL&mode=rwc&_busy_timeout=9999999", viper.GetString("dbfile")))
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
