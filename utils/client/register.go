package client

var set_pipreqs map[string]bool
var exploit_id, exploit_output map[string]string

func Register() {
	set_pipreqs = make(map[string]bool)
	exploit_id = make(map[string]string)
	exploit_output = make(map[string]string)

	heartbeat_delay = 5
	go heartbeat()
}
