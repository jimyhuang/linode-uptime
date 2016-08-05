package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"gopkg.in/gcfg.v1"
)

const configName = "linode-uptime.ini"

var config *configuration

type configuration struct {
	APIUri          string `gcfg:"uri"`
	BasicAuthName   string `gcfg:"username"`
	BasicAuthPasswd string `gcfg:"password"`
}
type Inventory struct {
	Meta  map[string]map[string]map[string]string `json:"_meta"`
	Hosts []string                                `json:"hosts"`
}
type RequestBody map[string]string
type Checks []*json.RawMessage
type check struct {
	name     string
	url      string
	isPaused bool
	isUp     bool
}

func main() {
	var err error
	config, err = getConfig()
	if err != nil {
		fatal(err)
	}

	reqBody := RequestBody{}
	body, e := apiRequest("GET", "checks", reqBody)

	var chk Checks
	var uptimeCheck = make(map[string]string)
	err = json.Unmarshal(body, &chk)
	if err != nil {
		fatal(err)
	}

	for _, ch := range chk {
		var ck interface{}
		json.Unmarshal(*ch, &ck)
		c := ck.(map[string]interface{})
		uptimeName := c["name"].(string)
		if c["isPaused"] != true {
			uptimeCheck[uptimeName] = c["_id"].(string)
		} else {
			uptimeCheck[uptimeName] = ""
		}
	}

	invByte, e := ioutil.ReadFile("/tmp/inventory.json")
	if e != nil {
		fatal("File error")
	}
	var inv = Inventory{}
	json.Unmarshal(invByte, &inv)
	for _, node := range inv.Meta["hostvars"] {
		label := node["host_label"]
		_, ok := uptimeCheck[label]
		if ok == false {
			fmt.Printf("N:%s\n", label)
			// PUT
			reqUri := "checks"
			reqBody := makeReqBody()
			reqBody["name"] = label
			reqBody["url"] = "http://" + node["host_public_ip"] + "/live/live.htm"
			apiRequest("PUT", reqUri, reqBody)
		} else if uptimeCheck[label] != "" {
			// POST
			fmt.Printf("U:%s\n", uptimeCheck[label])
			reqUri := "checks/" + uptimeCheck[label]
			reqBody := makeReqBody()
			reqBody["name"] = label
			reqBody["url"] = "http://" + node["host_public_ip"] + "/live/live.htm"
			apiRequest("POST", reqUri, reqBody)
		}
	}

	return
}

func getConfig() (*configuration, error) {
	// first check directory where the executable is located
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return nil, err
	}
	path := dir + "/" + configName
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// fallback to working directory. This is usefull when using `go run`
		path = configName
	}

	var config struct {
		Uptime configuration
	}

	err = gcfg.ReadFileInto(&config, path)
	if err != nil {
		return nil, err
	}

	return &config.Uptime, nil
}

func apiRequest(method string, uri string, requestBody map[string]string) ([]byte, error) {
	var err error
	var req *http.Request
	var resp *http.Response
	requestUri := config.APIUri + "/" + uri
	if method == "POST" || method == "PUT" {
		postValues := url.Values{}
		for key, value := range requestBody {
			postValues.Set(key, value)
		}
		postDataStr := postValues.Encode()
		postDataBytes := []byte(postDataStr)
		postBytesReader := bytes.NewReader(postDataBytes)
		req, err = http.NewRequest(method, requestUri, postBytesReader)
	} else if method == "GET" {
		req, err = http.NewRequest(method, requestUri, nil)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(config.BasicAuthName, config.BasicAuthPasswd)

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	return body, err
}

func makeReqBody() RequestBody {
	return map[string]string{
		"name":          "",
		"url":           "http://",
		"type":          "http",
		"alertTreshold": "2",
		"maxTime":       "5000",
		"interval":      "120",
	}
}

func fatal(v interface{}) {
	fmt.Fprintln(os.Stderr, v)
	os.Exit(1)
}
