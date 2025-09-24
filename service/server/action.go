package server

import (
	"0E7/service/config"
	"0E7/service/database"
	"0E7/utils"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

var (
	// 任务队列
	taskQueue = make(chan database.Action, 100)
	// 控制每个ID同时只有一个任务运行
	runningTasks sync.Map // map[uint]*runningTaskInfo
)

// 运行中的任务信息
type runningTaskInfo struct {
	Action *database.Action
	Ctx    context.Context
	Cancel context.CancelFunc
}

// Action配置结构体
type ActionConfig struct {
	Type string `json:"type"`
	Num  int    `json:"num"`
}

// Flag提交结果结构体
type FlagSubmitResult struct {
	Flag   string `json:"flag"`
	Status string `json:"status"`
	Msg    string `json:"msg"`
}

// 启动Action调度器
func StartActionScheduler() {
	// 启动任务执行器
	go taskExecutor()

	// 启动定时器，每5秒查询一次数据库
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		checkAndQueueActions()
	}
}

// 检查并排队待执行的任务
func checkAndQueueActions() {
	var actions []database.Action
	err := config.Db.Where("interval >= 0 AND code != '' AND status IN ('pending', 'completed') AND updated_at <= ?", time.Now()).Find(&actions).Error
	if err != nil {
		log.Println("查询Action失败:", err)
		return
	}

	for _, action := range actions {
		select {
		case taskQueue <- action:
			log.Printf("任务 %s 已加入队列", action.Name)
		default:
			log.Printf("任务队列已满，跳过任务 %s", action.Name)
		}
	}
}

// 任务执行器
func taskExecutor() {
	for task := range taskQueue {
		executeTask(task)
	}
}

// 执行单个任务
func executeTask(actionRecord database.Action) {
	// 检查该ID是否已有任务在运行
	if _, exists := runningTasks.Load(actionRecord.ID); exists {
		log.Printf("任务 %s (ID: %d) 已在运行中，跳过执行", actionRecord.Name, actionRecord.ID)
		return
	}

	// 检查任务是否已经超时
	if actionRecord.Status == "timeout" {
		log.Printf("任务 %s 已超时，跳过执行", actionRecord.Name)
		return
	}

	// 创建带超时的上下文，限制最多60秒
	timeout := time.Duration(actionRecord.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second // 默认30秒超时
	}
	if timeout > 60*time.Second {
		timeout = 60 * time.Second // 最多60秒超时
	}

	taskCtx, taskCancel := context.WithTimeout(context.Background(), timeout)

	// 创建运行任务信息
	taskInfo := &runningTaskInfo{
		Action: &actionRecord,
		Ctx:    taskCtx,
		Cancel: taskCancel,
	}

	// 标记该ID的任务为运行中
	runningTasks.Store(actionRecord.ID, taskInfo)
	defer func() {
		// 清理运行任务记录
		runningTasks.Delete(actionRecord.ID)
		taskCancel()
	}()

	// 更新任务状态为运行中
	actionRecord.Status = "running"
	actionRecord.UpdatedAt = time.Now()
	config.Db.Save(&actionRecord)

	log.Printf("开始执行任务 %s (ID: %d)", actionRecord.Name, actionRecord.ID)

	// 在goroutine中执行任务，以便可以取消
	done := make(chan error, 1)
	go func() {
		done <- executeActionCode(actionRecord, taskCtx)
	}()

	// 等待任务完成或超时
	select {
	case err := <-done:
		if err != nil {
			log.Printf("任务 %s (ID: %d) 执行失败: %v", actionRecord.Name, actionRecord.ID, err)
			actionRecord.Status = "error"
		} else {
			log.Printf("任务 %s (ID: %d) 执行成功", actionRecord.Name, actionRecord.ID)
			actionRecord.Status = "completed"
			// 更新下次执行时间
			actionRecord.UpdatedAt = time.Now().Add(time.Duration(actionRecord.Interval) * time.Second)
		}
	case <-taskCtx.Done():
		log.Printf("任务 %s (ID: %d) 执行超时", actionRecord.Name, actionRecord.ID)
		actionRecord.Status = "timeout"
	}

	// 保存任务状态
	config.Db.Save(&actionRecord)
}

// 执行Action代码
func executeActionCode(actionRecord database.Action, ctx context.Context) error {
	match := regexp.MustCompile(`^data:(code\/(?:python2|python3|golang));base64,(.*)$`).FindStringSubmatch(actionRecord.Code)
	if match == nil {
		return fmt.Errorf("代码格式错误")
	}

	fileType := match[1]
	data := match[2]
	code_decode, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("Base64解码错误: %v", err)
	}
	code := string(code_decode)

	// 解析配置
	var actionConfig ActionConfig
	if actionRecord.Config != "" {
		err = json.Unmarshal([]byte(actionRecord.Config), &actionConfig)
		if err != nil {
			log.Printf("解析Action配置失败: %v", err)
		}
	}

	var new_output string
	var flags []string

	// 如果是flag_submiter类型，获取flags
	if actionConfig.Type == "flag_submiter" && actionConfig.Num > 0 {
		flags, err = getFlagsForSubmission(actionConfig.Num)
		if err != nil {
			return fmt.Errorf("获取flags失败: %v", err)
		}
	}

	switch fileType {
	case "code/python2":
		new_output, err = executePythonCode(code, ctx, "python2", flags)
	case "code/python3":
		new_output, err = executePythonCode(code, ctx, "python3", flags)
	case "code/golang":
		new_output, err = executeGolangCode(code, ctx, flags)
	default:
		return fmt.Errorf("未知的文件类型: %s", fileType)
	}

	if err != nil {
		return err
	}

	// 如果是flag_submiter类型，处理提交结果
	if actionConfig.Type == "flag_submiter" {
		err = processFlagSubmissionResults(new_output, flags)
		if err != nil {
			log.Printf("处理flag提交结果失败: %v", err)
		}
	}

	// 如果输出有变化，更新数据库
	if new_output != actionRecord.Output {
		actionRecord.Output = new_output
		config.Db.Save(&actionRecord)
		log.Printf("Action %s 输出更新: %s", actionRecord.Name, new_output)
	}

	return nil
}

// 执行Python代码
func executePythonCode(code string, ctx context.Context, pythonVersion string, flags []string) (string, error) {
	var cmd *exec.Cmd

	// 根据Python版本选择命令
	switch pythonVersion {
	case "python2":
		cmd = exec.CommandContext(ctx, "python2", "-c", code)
	case "python3":
		cmd = exec.CommandContext(ctx, "python3", "-c", code)
	default:
		cmd = exec.CommandContext(ctx, "python", "-c", code)
	}

	// 如果有flags，作为参数传入
	if len(flags) > 0 {
		// 将flags转换为JSON字符串并转义
		flagsJSON, err := json.Marshal(flags)
		if err != nil {
			return "", fmt.Errorf("序列化flags失败: %v", err)
		}
		cmd.Args = append(cmd.Args, string(flagsJSON))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("Python执行错误: %v, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// 执行Golang代码
func executeGolangCode(code string, ctx context.Context, flags []string) (string, error) {
	var goibuf bytes.Buffer
	goi := interp.New(interp.Options{Stdout: &goibuf})
	goi.Use(stdlib.Symbols)

	// 如果有flags，在代码中注入flags变量
	if len(flags) > 0 {
		flagsJSON, err := json.Marshal(flags)
		if err != nil {
			return "", fmt.Errorf("序列化flags失败: %v", err)
		}
		// 在代码开头注入flags变量
		code = fmt.Sprintf("var flags = %s\n%s", string(flagsJSON), code)
	}

	md5 := utils.GetMd5FromString(code)
	var program *interp.Program
	var err error

	if value, ok := programs.Load(md5); !ok {
		program, err = goi.Compile(code)
		if err != nil {
			return "", fmt.Errorf("Go编译错误: %v", err)
		}
		programs.Store(md5, program)
	} else {
		program = value.(*interp.Program)
	}

	// 在goroutine中执行，以便可以被取消
	done := make(chan error, 1)
	go func() {
		_, err := goi.Execute(program)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return "", fmt.Errorf("Go运行时错误: %v", err)
		}
		return goibuf.String(), nil
	case <-ctx.Done():
		return "", fmt.Errorf("Go代码执行超时")
	}
}

// 获取当前运行的任务列表
func GetRunningTasks() map[uint]*runningTaskInfo {
	result := make(map[uint]*runningTaskInfo)
	runningTasks.Range(func(key, value interface{}) bool {
		id := key.(uint)
		taskInfo := value.(*runningTaskInfo)
		result[id] = taskInfo
		return true
	})
	return result
}

// 取消指定ID的任务
func CancelTask(id uint) bool {
	if value, exists := runningTasks.Load(id); exists {
		taskInfo := value.(*runningTaskInfo)
		taskInfo.Cancel()
		log.Printf("已取消任务 %s (ID: %d)", taskInfo.Action.Name, id)
		return true
	}
	return false
}

// 获取任务运行状态
func IsTaskRunning(id uint) bool {
	_, exists := runningTasks.Load(id)
	return exists
}

// 获取待提交的flags
func getFlagsForSubmission(num int) ([]string, error) {
	var flags []database.Flag
	err := config.Db.Where("status = ?", "QUEUE").Limit(num).Find(&flags).Error
	if err != nil {
		return nil, err
	}

	var flagStrings []string
	for _, flag := range flags {
		flagStrings = append(flagStrings, flag.Flag)
	}

	return flagStrings, nil
}

// 处理flag提交结果
func processFlagSubmissionResults(output string, submittedFlags []string) error {
	// 解析输出结果
	var results []FlagSubmitResult
	err := json.Unmarshal([]byte(output), &results)
	if err != nil {
		return fmt.Errorf("解析提交结果失败: %v", err)
	}

	// 更新数据库中的flag状态
	for _, result := range results {
		var flag database.Flag
		err := config.Db.Where("flag = ?", result.Flag).First(&flag).Error
		if err != nil {
			log.Printf("查找flag %s 失败: %v", result.Flag, err)
			continue
		}

		// 更新状态和消息
		flag.Status = result.Status
		flag.Msg = result.Msg
		flag.UpdatedAt = time.Now()

		err = config.Db.Save(&flag).Error
		if err != nil {
			log.Printf("更新flag %s 状态失败: %v", result.Flag, err)
		} else {
			log.Printf("更新flag %s 状态为: %s, 消息: %s", result.Flag, result.Status, result.Msg)
		}
	}

	return nil
}
