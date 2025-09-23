package client

import (
	"sync"
)

var set_pipreqs sync.Map
var programs sync.Map

type Tmonitor struct {
	types    string
	data     string
	interval int
}

var monitor_list map[int]Tmonitor

func Register() {
	monitor_list = make(map[int]Tmonitor)

	heartbeat_delay = 5
	go heartbeat()
}
