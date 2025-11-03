package client

import (
	"0E7/service/config"
	"context"
	"log"
	"sync"
	"time"

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
	workerSemaphore = semaphore.NewWeighted(int64(config.Client_worker))

	go heartbeat()
	if !config.Client_only_monitor {
		go exploitLoop()
	}

	if config.Client_monitor {
		go monitorLoop()
	}
}

// exploitLoop 独立运行 exploit，根据配置的时间间隔执行
func exploitLoop() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Exploit Loop Error:", err)
			go exploitLoop()
		}
	}()

	// 启动 goroutine 监听 jobsChan，处理 exploit 执行
	go func() {
		for range jobsChan {
			go func() {
				workerSemaphore.Acquire(context.Background(), 1)
				defer workerSemaphore.Release(1)
				exploit()
			}()
		}
	}()

	// 根据配置的时间间隔循环调用 exploit
	interval := time.Duration(config.Client_exploit_interval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		go func() {
			workerSemaphore.Acquire(context.Background(), 1)
			defer workerSemaphore.Release(1)
			exploit()
		}()
	}
}

func monitorLoop() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Monitor Loop Error:", err)
			go monitorLoop()
		}
	}()

	interval := 5 * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		monitor()
	}
}
