package server

import (
	"github.com/gin-gonic/gin"
	"github.com/traefik/yaegi/interp"
)

var programs map[int]*interp.Program

func Register(router *gin.Engine) {

	programs = make(map[int]*interp.Program)
	go heartbeat()
}
