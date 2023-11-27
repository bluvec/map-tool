package route

import (
	"io"
	test "mapdownloader/internal/asset"
	"os"
	"time"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Register struct {
}

func Run(port string) error {
	// r := gin.Default()

	// 日志关闭颜色
	gin.DisableConsoleColor()

	// 创建日志文件并设置为 gin.DefaultWriter
	f, _ := os.Create("gin.log")
	gin.DefaultWriter = io.MultiWriter(f)

	// std output
	// gin.DefaultWriter = io.MultiWriter(f, os.Stdout)

	// r := gin.New()
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "PUT", "PATCH", "POST", "DELETE"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// r.Use(LoggerMiddleware())

	// r.Use(static.Serve("/", static.LocalFile("/cmd/svr/build/index.html", true)))
	fsCss := assetfs.AssetFS{Asset: test.Asset, AssetDir: test.AssetDir, AssetInfo: test.AssetInfo, Prefix: "build/static/css", Fallback: "index.html"}
	fsFonts := assetfs.AssetFS{Asset: test.Asset, AssetDir: test.AssetDir, AssetInfo: test.AssetInfo, Prefix: "build", Fallback: "index.html"}
	fsImg := assetfs.AssetFS{Asset: test.Asset, AssetDir: test.AssetDir, AssetInfo: test.AssetInfo, Prefix: "build/images", Fallback: "index.html"}
	fsJs := assetfs.AssetFS{Asset: test.Asset, AssetDir: test.AssetDir, AssetInfo: test.AssetInfo, Prefix: "build/static/js", Fallback: "index.html"}
	fs := assetfs.AssetFS{Asset: test.Asset, AssetDir: test.AssetDir, AssetInfo: test.AssetInfo, Prefix: "build", Fallback: "index.html"}

	r.StaticFS("/favicon.ico", &fs)
	r.StaticFS("static/css", &fsCss)
	r.StaticFS("fonts", &fsFonts)
	r.StaticFS("images", &fsImg)
	r.StaticFS("static/js", &fsJs)

	r.GET("/dashboard", func(c *gin.Context) {
		c.Writer.WriteHeader(200)
		indexHtml, _ := test.Asset("build/index.html")
		_, _ = c.Writer.Write(indexHtml)
		c.Writer.Header().Add("Accept", "text/html")
		c.Writer.Flush()
	})

	r.GET("/sign", func(c *gin.Context) {
		c.Writer.WriteHeader(200)
		indexHtml, _ := test.Asset("build/index.html")
		_, _ = c.Writer.Write(indexHtml)
		c.Writer.Header().Add("Accept", "text/html")
		c.Writer.Flush()
	})

	r.GET("/", func(c *gin.Context) {
		c.Writer.WriteHeader(200)
		indexHtml, _ := test.Asset("build/index.html")
		_, _ = c.Writer.Write(indexHtml)
		c.Writer.Header().Add("Accept", "text/html")
		c.Writer.Flush()
	})

	v1 := r.Group("/v1")
	{
		v1.POST("/login", login)
		v1.POST("/optional", downloaderConfig)
		v1.GET("/mapinfo/:info", mapInfo)
		v1.GET("/start/:uuid", start)
		v1.GET("stop", stop)
		v1.GET("state", stateInfo)
		v1.GET("confirm", confirmInfo)
	}

	// test
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"code":    1,
			"message": "pong",
		})
	})

	err := r.Run(port)
	return err // listen and serve on 0.0.0.0:8080
}
