package server

import "log"

func flag() {

	defer func() {
		if err := recover(); err != nil {
			log.Println("Flag error: ", err)
		}
		jobsMutex.Lock()
		jobs["flag"] = false
		jobsMutex.Unlock()
	}()
}
