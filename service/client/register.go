package client

import (
	"0E7/service/config"
	"sync"

	"golang.org/x/sync/semaphore"
)

var set_pipreqs sync.Map
var programs sync.Map

type Tmonitor struct {
	types    string
	data     string
	interval int
}

var monitor_list sync.Map

func Register() {
	// 初始化 worker 信号量，限制并发 worker 数量
	workerSemaphore = semaphore.NewWeighted(int64(config.Client_worker))

	go heartbeat()
}
