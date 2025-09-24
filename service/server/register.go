package server

import (
	"sync"

	"github.com/gin-gonic/gin"
)

var programs sync.Map

func Register(router *gin.Engine) {
	go StartActionScheduler()
}
