package client

import (
	"0E7/utils/config"
)

var conf config.Conf

func Register(sconf config.Conf) {
	conf = sconf
	go heartbeat()
}
