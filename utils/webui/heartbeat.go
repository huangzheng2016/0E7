package webui

import (
	"log"
	"time"
)

func heartbeat() {
	defer func() {
		if err := recover(); err != nil {
			log.Println("Heartbeat Error:", err)
			go heartbeat()
		}
	}()
	for {
		go update_action()
		time.Sleep(time.Second * time.Duration(1))
	}
}
