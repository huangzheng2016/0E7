package webui

import (
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	// 日志缓冲区大小
	logBufferSize = 100
	// WebSocket写入超时
	writeWait = 10 * time.Second
	// WebSocket读取超时
	pongWait = 60 * time.Second
	// WebSocket ping间隔
	pingPeriod = (pongWait * 9) / 10
	// WebSocket最大消息大小
	maxMessageSize = 512
)

var (
	// WebSocket升级器
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true // 允许所有来源，生产环境应该限制
		},
	}

	// 日志广播器
	logBroadcaster  *LogBroadcaster
	broadcasterOnce sync.Once
)

// LogBroadcaster 日志广播器，使用worker模式
type LogBroadcaster struct {
	// 主channel，接收所有日志
	logChan chan string

	// 所有连接的WebSocket客户端
	clients map[*LogClient]bool

	// 客户端注册/注销channel
	register   chan *LogClient
	unregister chan *LogClient

	// 日志缓冲区（最新的100条）
	buffer   []string
	bufferMu sync.RWMutex

	// 广播器运行状态
	running bool
	mu      sync.RWMutex
}

// LogClient WebSocket客户端
type LogClient struct {
	// WebSocket连接
	conn *websocket.Conn

	// 该客户端的日志channel
	send chan string

	// 客户端ID（用于调试）
	id string
}

// GetLogBroadcaster 获取日志广播器单例
func GetLogBroadcaster() *LogBroadcaster {
	broadcasterOnce.Do(func() {
		logBroadcaster = &LogBroadcaster{
			logChan:    make(chan string, 1000), // 缓冲1000条日志
			clients:    make(map[*LogClient]bool),
			register:   make(chan *LogClient),
			unregister: make(chan *LogClient),
			buffer:     make([]string, 0, logBufferSize),
			running:    false,
		}
		go logBroadcaster.run()
	})
	return logBroadcaster
}

// run 运行广播器worker
func (lb *LogBroadcaster) run() {
	lb.mu.Lock()
	lb.running = true
	lb.mu.Unlock()

	defer func() {
		lb.mu.Lock()
		lb.running = false
		lb.mu.Unlock()
	}()

	for {
		select {
		case client := <-lb.register:
			lb.clients[client] = true
			// 发送缓存的日志给新客户端
			// 使用goroutine异步发送，避免阻塞主循环
			go func() {
				lb.bufferMu.RLock()
				cachedLogs := make([]string, len(lb.buffer))
				copy(cachedLogs, lb.buffer)
				lb.bufferMu.RUnlock()

				// 逐条发送缓存的日志
				for _, logMsg := range cachedLogs {
					select {
					case client.send <- logMsg:
						// 发送成功，继续下一条
					default:
						// 如果channel满了，跳过剩余的日志
						return
					}
				}
			}()

		case client := <-lb.unregister:
			if _, ok := lb.clients[client]; ok {
				delete(lb.clients, client)
				close(client.send)
			}

		case logMsg := <-lb.logChan:
			// 添加到缓冲区
			lb.bufferMu.Lock()
			lb.buffer = append(lb.buffer, logMsg)
			// 保持缓冲区大小不超过100条
			if len(lb.buffer) > logBufferSize {
				lb.buffer = lb.buffer[len(lb.buffer)-logBufferSize:]
			}
			lb.bufferMu.Unlock()

			// 广播给所有客户端
			for client := range lb.clients {
				select {
				case client.send <- logMsg:
				default:
					// 如果客户端channel满了，关闭连接
					delete(lb.clients, client)
					close(client.send)
				}
			}
		}
	}
}

// BroadcastLog 广播日志消息
func (lb *LogBroadcaster) BroadcastLog(message string) {
	lb.mu.RLock()
	running := lb.running
	lb.mu.RUnlock()

	if !running {
		return
	}

	select {
	case lb.logChan <- message:
	default:
		// 如果channel满了，丢弃这条日志
	}
}

// readPump 从WebSocket读取消息（主要用于处理ping/pong）
func (c *LogClient) readPump() {
	defer func() {
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

// writePump 向WebSocket写入消息
func (c *LogClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// 通道已关闭
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// 收集要发送的消息（包括第一条）
			messages := []string{message}

			// 批量收集队列中的其他消息（最多收集10条，避免单次发送过多）
			maxBatch := 10
			for i := 0; i < maxBatch; i++ {
				select {
				case msg := <-c.send:
					messages = append(messages, msg)
				default:
					// 没有更多消息了，跳出循环
					goto sendMessages
				}
			}

		sendMessages:

			// 逐条发送消息，确保每条消息都单独发送
			for _, msg := range messages {
				if err := c.conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					return
				}
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleLogWebSocket 处理WebSocket连接
func handleLogWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	broadcaster := GetLogBroadcaster()

	client := &LogClient{
		conn: conn,
		send: make(chan string, 256),
		id:   c.ClientIP(),
	}

	broadcaster.register <- client

	// 启动读写goroutines
	go client.writePump()
	go client.readPump()
}

// LogWriter 日志拦截Writer，将日志输出同时写入原始Writer和广播器
type LogWriter struct {
	original    io.Writer
	broadcaster *LogBroadcaster
}

// Write 实现io.Writer接口
func (lw *LogWriter) Write(p []byte) (n int, err error) {
	// 写入原始Writer
	n, err = lw.original.Write(p)
	if err != nil {
		return n, err
	}

	// 清理日志消息：去除ANSI转义码、多余的空白字符和末尾换行符
	message := cleanLogMessage(string(p))
	if len(message) > 0 {
		lw.broadcaster.BroadcastLog(message)
	}

	return n, nil
}

// cleanLogMessage 清理日志消息，去除ANSI转义码和多余的空白字符
func cleanLogMessage(msg string) string {
	// 去除末尾的换行符和空白字符
	msg = strings.TrimRight(msg, " \t\n\r")
	if len(msg) == 0 {
		return ""
	}

	// 去除ANSI转义码（颜色码等）
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	msg = ansiRegex.ReplaceAllString(msg, "")

	// 将所有制表符替换为空格
	msg = strings.ReplaceAll(msg, "\t", " ")

	// 去除行首的所有空白字符（包括空格和制表符）
	msg = strings.TrimLeft(msg, " \t")

	// 将多个连续空格（2个或更多）替换为单个空格
	spaceRegex := regexp.MustCompile(` +`)
	msg = spaceRegex.ReplaceAllString(msg, " ")

	// 去除行尾空白
	msg = strings.TrimRight(msg, " \t")

	return msg
}

// NewLogWriter 创建新的日志拦截Writer
func NewLogWriter(original io.Writer) io.Writer {
	broadcaster := GetLogBroadcaster()
	return &LogWriter{
		original:    original,
		broadcaster: broadcaster,
	}
}
