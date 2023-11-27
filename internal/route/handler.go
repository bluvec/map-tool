package route

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mapdownloader/internal/downloader"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type UserInfo struct {
	Downloader *downloader.DownLoader
	MapInfo    downloader.MapInfo
	Divided    int  `json:"div"`
	CapPipe    int  `json:"cap"`
	TaskQueue  int  `json:"task"`
	WorkerMax  int  `json:"work"`
	Recover    bool `json:"rec"`
	AddModel   bool `json:"add"`
	finish     chan bool
	TotalTile  int `json:"totalTile"`
	state      state
}

type state struct {
	Percent     float64 `json:"percent"`
	TileDone    int     `json:"tiledone"`
	TileErr     int     `json:"tileErr"`
	Complete    bool    `json:"complete"`
	Downloading bool    `json:"downloading"`
}

func NewUserInfo() *UserInfo {
	return &UserInfo{
		Divided:   1,
		CapPipe:   512,
		TaskQueue: 512,
		WorkerMax: 128,
		Recover:   false,
		AddModel:  false,
		finish:    make(chan bool),
	}
}

var user = NewUserInfo()

func login(c *gin.Context) {

}

func start(c *gin.Context) {
	uuid := c.Param("uuid")
	user.Downloader.Uid = uuid
	go user.Downloader.Start(user.finish)
	go checkProcess()
	c.JSON(http.StatusOK, gin.H{
		"code":    1,
		"message": "start download success",
	})
}

func stop(c *gin.Context) {
	//手动停止下载
}

func stateInfo(c *gin.Context) {

	c.JSON(http.StatusOK, gin.H{
		"code":    1,
		"message": "get download state success",
		"data":    user.state,
	})

}

func confirmInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    1,
		"message": "get cofirm success",
		"data":    *user,
	})
}

func mapInfo(c *gin.Context) {
	info := c.Param("info")
	if data, err := base64.StdEncoding.DecodeString(info); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "0",
			"message": "get mapInfo failue",
		})
	} else {
		mapInfo := downloader.MapInfo{}
		if err := json.Unmarshal(data, &mapInfo); err != nil {
			fmt.Println(err)
			c.JSON(http.StatusNotFound, gin.H{
				"code":    "0",
				"message": "json unmarshal faild",
			})
		} else {
			//save userInfo
			user.MapInfo = mapInfo
			user.MapInfo.DbPath = "./mapTiles.db"
			user.Downloader = downloader.NewDownLoader(user.MapInfo, user.CapPipe, user.TaskQueue, user.WorkerMax, user.Divided, user.Recover, user.AddModel)
			user.Downloader.BaseStr = info

			//获取下载瓦片总数量，以及预估大小
			user.TotalTile = user.Downloader.GetTaskInfo()
			c.JSON(http.StatusOK, gin.H{
				"code":    1,
				"message": "set MapInfo success",
				"data":    user.TotalTile,
			})
		}
	}
}

func downloaderConfig(c *gin.Context) {
	c.BindJSON(&user)
	data := *user
	user.Downloader = downloader.NewDownLoader(user.MapInfo, user.CapPipe, user.TaskQueue, user.WorkerMax, user.Divided, user.Recover, user.AddModel)
	user.Downloader.SplitTable(user.Divided)
	c.JSON(http.StatusOK, gin.H{
		"code":    1,
		"message": "downloader config succeess",
		"data":    data,
	})
}

func checkProcess() {
	for {
		time.Sleep(time.Millisecond * 500)
		percent, tileDone, tileErr := user.Downloader.GetDownPercent()
		var state = state{
			Percent:     percent,
			TileDone:    tileDone,
			TileErr:     tileErr,
			Complete:    false,
			Downloading: true,
		}
		if percent == float64(1) && tileDone+tileErr == user.TotalTile {
			select {
			case <-user.finish:
				// fmt.Println("下载完成")
				state.Complete = true
				state.Downloading = false
				user.state = state
				return
			default:
			}
		}
		user.state = state
	}
}
