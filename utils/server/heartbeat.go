package server

import (
	"log"
	"sync"
	"time"
)

var jobsMutex sync.Mutex
var jobs map[string]bool = make(map[string]bool)

func heartbeat() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Heartbeat Error:", err)
			go heartbeat()
		}
	}()
	for {
		jobsMutex.Lock()
		if jobs["action"] == false {
			jobs["action"] = true
			go action()
		}
		if jobs["flag"] == false {
			jobs["flag"] = true
			go flag()
		}
		jobsMutex.Unlock()
		time.Sleep(time.Second * time.Duration(1))
	}
}
