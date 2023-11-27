package main

import (
	"fmt"
	"log"
	"mapdownloader/internal/common"
	"mapdownloader/internal/route"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
)

var commands = map[string]string{
	"windows": "explorer",
	"darwin":  "open",
	"linux":   "xdg-open",
}

func Open(uri string) error {
	// runtime.GOOS获取当前平台
	run, ok := commands[runtime.GOOS]
	if !ok {
		return fmt.Errorf("don't know how to open things on %s platform", runtime.GOOS)
	}

	cmd := exec.Command(run, uri)
	return cmd.Run()
}

func main() {
	// run server
	fmt.Println("|-----------------------------------|")
	fmt.Println("|        map_downloader_server      |")
	fmt.Println("|-----------------------------------|")
	fmt.Println("|  Go Http Server Start Successful  |")
	fmt.Println("|    Port:" + common.Cfg.Port + "     Pid:" + fmt.Sprintf("%d", os.Getpid()) + "         |")
	fmt.Println("|-----------------------------------|")
	fmt.Println("")

	//open browser
	// urle := "http://localhost:8080"
	url := common.Cfg.Url + common.Cfg.Port
	if common.Cfg.Lan == "en" {
		if err := Open(url); err != nil {
			fmt.Println("--------Failed to open browser----------")
			fmt.Println("-------try to open the address manually：----")
			fmt.Println("-------------", url, "----------")

		}
		fmt.Println("！！！！！！！！！！！！！！！！！！！！！！！！！！！！！！")
		fmt.Println("-！！！！！Download do not close this window！！！！！-")
		fmt.Println("！！！！！！！！！！！！！！！！！！！！！！！！！！！！！！")

		go func() {
			if err := route.Run(common.Cfg.Port); err != nil {
				fmt.Println("！！！！！！！！！！！！！！！！！！！！！！！！！！！！！！")
				fmt.Println("-！！！！！Failed to start, try to change the port-！！！！！")
				fmt.Println("！！！！！！！！！！！！！！！！！！！！！！！！！！！！！！！！！！！")
				panic("Please change the port and try again")
			}
		}()

	} else {
		if err := Open(url); err != nil {
			fmt.Println("--------打开浏览器失败----------")
			fmt.Println("-------尝试手动打开地址：----")
			fmt.Println("-----", url, "----------")

		}
		fmt.Println("！！！！！！！！！！！！！！！！！！！！")
		fmt.Println("-！！！！！下载请勿关闭此窗口！！！！！-")
		fmt.Println("！！！！！！！！！！！！！！！！！！！！")

		go func() {
			if err := route.Run(common.Cfg.Port); err != nil {
				fmt.Println("！！！！！！！！！！！！！！！！！！！！")
				fmt.Println("-！！！！！启动失败，尝试更换端口-！！！！！")
				fmt.Println("！！！！！！！！！！！！！！！！！！！！")
				panic("请更换端口重试")
			}
		}()
	}

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")
}
