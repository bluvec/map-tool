package common

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

var Cfg Config

type Config struct {
	Port     string `yaml:"port"`
	Url      string `yaml:"url"`
	StoneSvr string `yaml:"stoneSvr"`
	GoogleMp string `yaml:"googlemp"`
	GoogleMs string `yaml:"googlems"`
	GaodeImg string `yaml:"gaodeimg"`
	GaodeAno string `yaml:"gaodeano"`
	Lan      string `yaml:"language"`
}

type Body struct {
	History string `json:"history"`
	Account string `json:"account"`
	UserId  string `json:"uuid"`
}

func init() {

	//读取yaml文件到缓存中
	config, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		fmt.Print("config load err: ", err)
	}

	//yaml文件内容影射到结构体中
	err1 := yaml.Unmarshal(config, &Cfg)
	if err1 != nil {
		fmt.Println("error")
	}

}

//update mapInfo
func UdataDowloadInfo(uuid, size string, strBase string) {
	var body Body
	var temp = make(map[string]string)
	str := fmt.Sprintf(size + "_" + strBase)
	key := time.Now().Format("2006-01-02_15:04:05")
	temp[key] = str
	res, _ := json.Marshal(temp)
	body.History = string(res)
	body.Account = size
	body.UserId = uuid
	v, _ := json.Marshal(body)
	if _, err := Post(Cfg.StoneSvr+"/v1/map/update", "application/json", string(v)); err != nil {
		panic("json send err")
	}
}

func Post(url string, contentType string, body string) (string, error) {
	res, err := http.Post(url, contentType, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	content, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
