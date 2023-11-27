package downloader

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"mapdownloader/config"
	"mapdownloader/internal/pool"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/patrickmn/go-cache"
	"github.com/robertkrimen/otto"
)

type Tile struct {
	CountFail   int
	TilesType   int
	TilesCol    string
	TilesRow    string
	TilesLevel  string
	TilesBinary []byte
}

type MapInfo struct {
	Type     interface{} `json:"type"`
	MinZ     int         `json:"minZ"`
	MaxZ     int         `json:"maxZ"`
	DbPath   string
	MinLng   string `json:"minLng"`
	MaxLng   string `json:"maxLng"`
	MinLat   string `json:"minLat"`
	MaxLat   string `json:"maxLat"`
	Language string `json:"lang"`
}

type DownLoader struct {
	db        *sql.DB
	jobs      map[int][]Tile
	rawjobs   []Tile
	jsVM      *otto.Otto
	mapInfo   MapInfo
	provider  map[int]string
	netClient *http.Client

	capPipe   int
	capQueue  int
	maxWorker int

	errTiles     int
	taskerrTile  int
	doneTiles    int
	taskdoneTile int
	totalTiles   int

	divideTiles map[int]int
	curtast     int
	dividor     int
	c           *cache.Cache
	recover     bool
	addModel    bool
	existTable  int
	init_table  bool
	taskId      int
	BaseStr     string
	Uid         string
	PreSize     float64
}

func NewDownLoader(info MapInfo, capPipe, capQueue, maxWorker, dividor int, recover bool, addModel bool) *DownLoader {
	vm := otto.New()
	vm.Run(config.CoordTransformLib)

	client := &http.Client{}
	client.Transport = &http.Transport{
		MaxIdleConnsPerHost: 5000,
		MaxConnsPerHost:     5000,
		IdleConnTimeout:     time.Second * 60,
		DisableKeepAlives:   false,
	}
	client.Timeout = 60 * time.Second

	cache := cache.New(cache.NoExpiration, cache.NoExpiration)
	cache.LoadFile(config.CHACHE_PATH)

	return &DownLoader{
		jobs:       make(map[int][]Tile, dividor),
		jsVM:       vm,
		mapInfo:    info,
		netClient:  client,
		totalTiles: 0,
		capPipe:    capPipe,
		capQueue:   capQueue,
		maxWorker:  maxWorker,
		dividor:    dividor,
		c:          cache,
		recover:    recover,
		taskId:     1,
		addModel:   addModel,
		existTable: 0,
	}
}

func (dl *DownLoader) GetTaskInfo() int {
	jobs := make([]Tile, 0)
	var value string
	typ := reflect.TypeOf(dl.mapInfo.Type)
	switch typ.String() {
	case `string`:
		{
			value, _ = dl.mapInfo.Type.(string)
		}
	case `float64`:
		{
			tmp, _ := dl.mapInfo.Type.(float64)
			value = strconv.FormatFloat(tmp, 'f', -1, 64)
			fmt.Println(tmp, reflect.TypeOf(value))
		}
	}
	if value == "0" {
		jobs = append(jobs, dl.getTilesList([]int{0})...)
	} else if value == "1" {
		jobs = append(jobs, dl.getTilesList([]int{1, 2})...)

	} else if value == "2" {
		jobs = append(jobs, dl.getTilesList([]int{0, 1, 2})...)
	} else if value == "3" {
		jobs = append(jobs, dl.getTilesList([]int{3, 4})...)
	}
	dl.rawjobs = jobs
	dl.test(jobs)
	return dl.totalTiles
}

func (dl *DownLoader) GetDownPercent() (float64, int, int) {
	return float64(dl.doneTiles+dl.errTiles) / float64(dl.totalTiles), dl.doneTiles, dl.errTiles
}

func (dl *DownLoader) Start(finish chan bool) bool {
	if dl.totalTiles == 0 {
		return false
	}
	if dl.mapInfo.Language == "zh" {
		dl.provider = config.PROVIDER_CN
	} else {
		dl.provider = config.PROVIDER_EN
	}
	dl.initDB()
	// dl.cleanDB()

	tilesPipe := make(chan Tile, dl.capPipe)
	done := make(chan bool)
	taskdone := make(chan bool)

	pool := pool.NewDispatcher(dl.maxWorker, dl.capQueue)
	pool.Run()
	// go dl.saveTiles(tilesPipe, done, taskdone)
	var start int
	if dl.recover {
		fmt.Println("进入cache")
		val, _ := dl.c.Get("start")
		fmt.Println("获取 cache data", val)

		donetiles, _ := dl.c.Get("doneTiles")
		start = val.(int)
		dl.doneTiles = donetiles.(int)
	}
	for key := start; key < dl.dividor; key++ {
		ctx, cancel := context.WithCancel(context.Background())
		dl.curtast = key
		dl.c.Set("start", key, -1)
		dl.c.SaveFile(config.CHACHE_PATH)
		fmt.Printf("-------------------------Process %d task start Exec------------------", dl.curtast)
		go dl.saveTiles(tilesPipe, done, taskdone)
		errTiles := make(chan Tile, len(dl.jobs[key]))
		for _, v := range dl.jobs[key] {
			tile := v
			job := func() {
				var err error
				url := strings.Replace(dl.provider[tile.TilesType], "{x}", tile.TilesRow, 1)
				url = strings.Replace(url, "{y}", tile.TilesCol, 1)
				url = strings.Replace(url, "{z}", tile.TilesLevel, 1)
				if tile.TilesBinary, err = dl.getTileBinary(url, tile.TilesRow, tile.TilesCol, tile.TilesLevel); err != nil {
					errTiles <- tile
				} else {
					tilesPipe <- tile
				}
			}
			pool.JobQueue <- job
		}
		go dl.errProcess(ctx, errTiles, tilesPipe, pool)
		<-taskdone
		fmt.Println("Process exec success")
		cancel()
	}
	<-done
	dl.setTask()
	// dl.db.Exec(config.CreateIndex)
	dl.db.Close()
	// fmt.Println("set task start")
	// if fi, err := os.Stat("mapTiles.db"); err != nil {
	// 	fmt.Println("读取文件失败")
	// } else {
	// 	//文件byte 转 Mb,更新记录
	// 	res := math.Ceil(((float64(fi.Size()) - dl.PreSize) / (1024 * 1024)))
	// 	common.UdataDowloadInfo(dl.Uid, fmt.Sprint(res), dl.BaseStr)
	// }
	fmt.Println("All task success")
	finish <- true
	return true
}

func (dl *DownLoader) saveTiles(pipe chan Tile, done chan bool, taskdone chan bool) {
	var tablename, taskname string
	tx, _ := dl.db.Begin()
	tablename = "map" + strconv.Itoa(dl.curtast+dl.existTable)
	taskname = "task" + strconv.Itoa(dl.curtast+dl.existTable)
	stmt, _ := tx.Prepare("INSERT INTO " + tablename + "(zoom_level,tile_column,tile_row,tile_type,tile_data) values(?,?,?,?,?);")
	st1, _ := tx.Prepare("INSERT INTO " + taskname + "(id,count,version,date,maxY,minY) values(?,?,?,?,?,?);")

	defer dl.db.Exec(`
		CREATE INDEX map_index` + strconv.Itoa(dl.curtast+dl.existTable) + ` ON ` + tablename + ` (
		zoom_level,
		tile_column,
		tile_row,
		tile_type,
		tile_data
		);`)

	defer tx.Commit()

	for {
		select {
		case ti := <-pipe:
			if _, err := stmt.Exec(ti.TilesLevel, ti.TilesCol, ti.TilesRow, ti.TilesType, ti.TilesBinary); err != nil {
				fmt.Println("saveTile err", err)
			} else {
				dl.doneTiles++
				dl.taskdoneTile++
			}
		default:
			if dl.taskdoneTile+dl.taskerrTile == dl.divideTiles[dl.curtast] && dl.doneTiles != 0 {
				dl.taskdoneTile = 0
				dl.taskerrTile = 0
				date := time.Now().Format("2006-01-02 15:04:05")
				st1.Exec(1, dl.divideTiles[dl.curtast], config.VERSION, date, dl.jobs[dl.curtast][len(dl.jobs[dl.curtast])-1].TilesCol, dl.jobs[dl.curtast][0].TilesCol)
				taskdone <- true

				if dl.doneTiles+dl.errTiles == dl.totalTiles {
					done <- true
				}
				dl.c.Set("doneTiles", dl.doneTiles, -1)
				dl.c.SaveFile(config.CHACHE_PATH)
				return
			}
		}
	}
}

func (dl *DownLoader) initDB() {
	isExist := dl.exists(dl.mapInfo.DbPath)
	if isExist {
		if fi, err := os.Stat("mapTiles.db"); err != nil {
			fmt.Println("读取文件失败")
		} else {
			dl.PreSize = float64(fi.Size())
		}
	}
	dl.db, _ = sql.Open("sqlite3", dl.mapInfo.DbPath+"?_sync=2&_journal=MEMORY"+"&_busy_timeout=9999999")
	if !isExist {
		for i := 0; i < dl.dividor; i++ {
			tablename := "map" + strconv.Itoa(i)
			taskname := "task" + strconv.Itoa(i)
			dl.db.Exec(`
			CREATE TABLE IF NOT EXISTS ` + tablename + ` (
				zoom_level  INTEGER,
				tile_column INTEGER,
				tile_row    INTEGER,
				tile_id     INTEGER PRIMARY KEY AUTOINCREMENT,
				tile_type   INT,
				tile_data   BLOB
			);`)

			dl.db.Exec(`
			CREATE TABLE IF NOT EXISTS ` + taskname + ` (
				id    STRING UNIQUE,
				count INT,
				version DOUBLE,
				date  DATE,
				maxY INT,
				minY INT
			);`)
		}

		dl.db.Exec(`
				CREATE TABLE IF NOT EXISTS ` + "task" + ` (
					id    INTEGER PRIMARY KEY AUTOINCREMENT,
					type  INT,
					count INT,
					version DOUBLE,
					language STRING,
					date  DATE,
					maxLevel INT,
					minLevel INT
				);`)
		// dl.db.Exec(config.TileTable)
		// dl.db.Exec(config.TaskTable)
	}
	if dl.addModel && !dl.init_table && !dl.recover {
		var existTable int
		resulst := dl.db.QueryRow(`SELECT count(name)as count FROM sqlite_master WHERE type = "table" and name like "map%"`)
		if err := resulst.Scan(&existTable); err != nil {
			fmt.Println("读取现有数据库资料错误", err)
			return
		}
		dl.existTable = existTable
		for i := 0; i < dl.dividor; i++ {
			tablename := "map" + strconv.Itoa(i+dl.existTable)
			taskname := "task" + strconv.Itoa(i+dl.existTable)
			dl.db.Exec(`
			CREATE TABLE IF NOT EXISTS ` + tablename + ` (
				zoom_level  INTEGER,
				tile_column INTEGER,
				tile_row    INTEGER,
				tile_id     INTEGER PRIMARY KEY AUTOINCREMENT,
				tile_type   INT,
				tile_data   BLOB
			);`)

			dl.db.Exec(`
			CREATE TABLE IF NOT EXISTS ` + taskname + ` (
				id    STRING UNIQUE,
				count INT,
				version DOUBLE,
				date  DATE,
				maxY INT,
				minY INT
			);`)
		}
		dl.c.Set("existTable", dl.existTable, -1)
		dl.c.SaveFile(config.CHACHE_PATH)
	}
	if dl.recover {
		if existTable, ok := dl.c.Get("existTable"); ok {
			dl.existTable = existTable.(int)
		}

		val, ok := dl.c.Get("start")
		if !ok {
			fmt.Println("获取cache数据失败!!!")
		}

		fmt.Println("获取cache数据", val)

		mapname := "map" + strconv.Itoa(val.(int)+dl.existTable)
		taskname := "task" + strconv.Itoa(val.(int)+dl.existTable)
		//清空上次数据库残留
		dl.cleanDB(mapname, taskname)
	}

}

func (dl *DownLoader) cleanDB(mapname, taskname string) {
	var err error
	clean := func(query string) {
		if err != nil {
			fmt.Println("clean err")
			return
		}
		_, err = dl.db.Exec(query)
	}
	clean("DELETE FROM " + mapname)
	clean("DELETE FROM " + taskname)
	clean("update sqlite_sequence set seq = 0 where name = " + mapname)
	// clean("delete from sqlite_sequence where name = " + mapname)
	// clean("delete from sqlite_sequence")
	// clean("VACUUM")
}

func (dl *DownLoader) setTask() {
	date := time.Now().Format("2006-01-02 15:04:05")
	stmt, _ := dl.db.Prepare("INSERT INTO " + "task" + "(type,count,version,language,date,maxLevel,minLevel) values(?,?,?,?,?,?,?);")

	stmt.Exec(dl.mapInfo.Type, dl.totalTiles-dl.errTiles, config.VERSION, dl.mapInfo.Language, date, dl.mapInfo.MaxZ, dl.mapInfo.MinZ)
	defer stmt.Close()
	return
}

func (dl *DownLoader) getTilesList(maptype []int) []Tile {
	jobs := make([]Tile, 0)
	for z := dl.mapInfo.MinZ; z <= dl.mapInfo.MaxZ; z++ {
		minX, minY := dl.getTilesCoordinate(dl.mapInfo.MinLng, dl.mapInfo.MinLat, z)
		maxX, maxY := dl.getTilesCoordinate(dl.mapInfo.MaxLng, dl.mapInfo.MaxLat, z)
		for i := minX; i <= maxX; i++ {
			for j := minY; j <= maxY; j++ {
				for _, k := range maptype {
					jobs = append(jobs, Tile{
						TilesRow:   strconv.Itoa(i),
						TilesCol:   strconv.Itoa(j),
						TilesType:  k,
						TilesLevel: strconv.Itoa(z),
					})
				}

			}
		}
	}
	return jobs
}

func (dl *DownLoader) test(jobs []Tile) {
	dl.totalTiles = len(jobs)
	n := dl.totalTiles / dl.dividor
	sort.SliceStable(jobs, func(i, j int) bool {
		s1, _ := strconv.Atoi(jobs[i].TilesCol)
		s2, _ := strconv.Atoi(jobs[j].TilesCol)
		return s1 < s2
	})
	if dl.divideTiles == nil {
		dl.divideTiles = make(map[int]int)
	}

	for i := 0; i < dl.dividor; i++ {
		if i == dl.dividor-1 {
			dl.jobs[i] = jobs[i*n:]
			dl.divideTiles[i] = len(dl.jobs[i])
			// return

		} else {
			dl.jobs[i] = jobs[i*n : (i+1)*n]
			dl.divideTiles[i] = len(dl.jobs[i])
		}

	}
	return
}

func (dl *DownLoader) SplitTable(count int) {
	jobs := dl.rawjobs
	dl.dividor = count
	dl.totalTiles = len(jobs)
	n := dl.totalTiles / dl.dividor
	sort.SliceStable(jobs, func(i, j int) bool {
		s1, _ := strconv.Atoi(jobs[i].TilesCol)
		s2, _ := strconv.Atoi(jobs[j].TilesCol)
		return s1 < s2
	})
	if dl.divideTiles == nil {
		dl.divideTiles = make(map[int]int)
	}

	for i := 0; i < dl.dividor; i++ {
		if i == dl.dividor-1 {
			dl.jobs[i] = jobs[i*n:]
			dl.divideTiles[i] = len(dl.jobs[i])
			// return

		} else {
			dl.jobs[i] = jobs[i*n : (i+1)*n]
			dl.divideTiles[i] = len(dl.jobs[i])
		}

	}
	return
}

func (dl *DownLoader) getTilesCoordinate(lng, lat string, z int) (x, y int) {
	flng, _ := strconv.ParseFloat(lng, 64)
	flat, _ := strconv.ParseFloat(lat, 64)
	result, _ := dl.jsVM.Call("TileLnglatTransform.TileLnglatTransformGoogle.lnglatToTile", nil, flng, flat, z)
	tileX, _ := result.Object().Get("tileX")
	tileY, _ := result.Object().Get("tileY")
	x, _ = strconv.Atoi(tileX.String())
	y, _ = strconv.Atoi(tileY.String())
	return
}

func (dl *DownLoader) getTileBinary(url, x, y, z string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9")
	req.Header.Add("Accept-Encoding", "gzip, deflate")
	req.Header.Add("Accept-Language", "zh,en-US;q=0.9,en;q=0.8,zh-CN;q=0.7")
	req.Header.Add("Cache-Control", "max-age=0")
	req.Header.Add("Connection", "Keep-Alive")
	req.Header.Add("Upgrade-Insecure-Requests", "1")
	req.Header.Add("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/84.0.4147.125 Safari/537.36")

	resp, err := dl.netClient.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if resp.StatusCode == 200 && len(data) != 0 {
		// fmt.Println(len(buf.Bytes()))
		return data, err
	} else {
		// fmt.Println("数据为空")
		return nil, fmt.Errorf("数据为空")
	}

}

func (dl *DownLoader) exists(path string) bool {
	if _, err := os.Stat(path); err != nil {
		return os.IsExist(err)
	} else {
		return true
	}
}

func (dl *DownLoader) errProcess(ctx context.Context, errtile chan Tile, oktile chan Tile, pool *pool.Dispatcher) {
	var retryTimes = 10
	// fmt.Println("等待进入错误处理")
	for {
		select {
		case <-ctx.Done():
			return
		case tile := <-errtile:
			tile.CountFail++
			// fmt.Println("读取err", tile)
			if tile.CountFail > retryTimes {
				// fmt.Println("no region tile")
				dl.errTiles++
				dl.taskerrTile++
			} else {
				job := func() {
					var err error
					url := strings.Replace(dl.provider[tile.TilesType], "{x}", tile.TilesRow, 1)
					url = strings.Replace(url, "{y}", tile.TilesCol, 1)
					url = strings.Replace(url, "{z}", tile.TilesLevel, 1)
					if tile.TilesBinary, err = dl.getTileBinary(url, tile.TilesRow, tile.TilesCol, tile.TilesLevel); err != nil {
						errtile <- tile
					} else {
						oktile <- tile
					}

				}
				pool.JobQueue <- job
			}
		}

	}
}
