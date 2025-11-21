package main

import (
	"fmt"
	"log"
	"os"
	"time"

	postgrescli "binrc.com/dbcli/postgres-cli"
)

type Terminal struct {
	*os.File
}

func (t *Terminal) Read(p []byte) (n int, error) {
	return os.Stdin.Read(p)
}

func (t *Terminal) Write(p []byte) (n int, error) {
	return os.Stdout.Write(p)
}

func main() {
	term := &Terminal{os.Stdout}

	config := &postgrescli.Config{
		Host:     "localhost",
		Port:     5432,
		Username: "postgres",
		Password: "password",
		Database: "testdb",

		// SSL 配置
		SSLMode: "disable", // disable/require/verify-ca/verify-full

		// 超时配置
		ConnectTimeout:   15 * time.Second, // 连接超时
		StatementTimeout: 60 * time.Second, // 语句超时

		// 连接池配置
		MaxOpenConns:    20,           // 最大连接数
		MaxIdleConns:    10,           // 最大空闲连接
		ConnMaxLifetime: 2 * time.Hour, // 连接最大生命周期

		// 应用配置
		ApplicationName: "my-app", // 应用名称
		SearchPath:      "public,myschema", // 搜索路径
		TimeZone:        "Asia/Shanghai",   // 时区

		// 自定义参数
		CustomParams: "options=-c statement_timeout=30000",
	}

	cli := postgrescli.NewCLIWithConfig(term, config)

	if err := cli.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer cli.Close()

	fmt.Println("Connected with custom config!")

	if err := cli.Start(); err != nil {
		log.Fatalf("CLI error: %v", err)
	}
}


