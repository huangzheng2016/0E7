package client

import (
	"github.com/traefik/yaegi/interp"
)

var set_pipreqs map[string]bool
var exploit_id, exploit_output map[string]string
var programs map[string]*interp.Program

type Tmonitor struct {
	types    string
	data     string
	interval int
}

var monitor_list map[int]Tmonitor

func Register() {
	set_pipreqs = make(map[string]bool)
	exploit_id = make(map[string]string)
	exploit_output = make(map[string]string)
	programs = make(map[string]*interp.Program)
	monitor_list = make(map[int]Tmonitor)

	heartbeat_delay = 5
	go heartbeat()
}
