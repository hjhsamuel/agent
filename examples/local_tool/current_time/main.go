package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	_ "time/tzdata" // 内嵌时区数据，兼容缺少时区数据库的系统
)

type Request struct {
	Timezone string `json:"timezone"`
}

type Response struct {
	Time  string `json:"time,omitempty"`
	Error string `json:"error,omitempty"`
}

func run(req Request) Response {
	if req.Timezone == "" {
		req.Timezone = "Asia/Shanghai"
	}

	location, err := time.LoadLocation(req.Timezone)
	if err != nil {
		return Response{
			Error: fmt.Sprintf("无效时区：%s", req.Timezone),
		}
	}

	return Response{
		Time: time.Now().In(location).Format(time.RFC3339),
	}
}

func main() {
	var req Request
	var resp Response

	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		resp.Error = "输入必须是合法 JSON：" + err.Error()
	} else {
		resp = run(req)
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// 输入错误也作为工具结果返回。服务只在退出码为 0 时解析 stdout。
}
