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
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

var (
	// 任务队列
	taskQueue = make(chan *database.Action, 100)
	// 控制每个ID同时只有一个任务运行
	runningTasks sync.Map
)

// 运行中的任务信息
type runningTaskInfo struct {
	Action *database.Action
	Ctx    context.Context
	Cancel context.CancelFunc
}

// Action配置结构体
type ActionConfig struct {
	Type     string `json:"type"`
	Num      int    `json:"num"`
	ScriptID int    `json:"script_id"`
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
	year2000 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	err := config.Db.Where("is_deleted = ? AND ((interval >= 0 AND next_run <= ?) OR (interval < 0 AND next_run < ?))", false, time.Now(), year2000).Find(&actions).Error
	if err != nil {
		log.Println("查询Action失败:", err)
		return
	}

	for i := range actions {
		select {
		case taskQueue <- &actions[i]:
			log.Printf("任务 %s 已加入队列", actions[i].Name)
		default:
			log.Printf("任务队列已满，跳过任务 %s", actions[i].Name)
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
func executeTask(actionRecord *database.Action) {
	// 检查该ID是否已有任务在运行
	if _, exists := runningTasks.Load(actionRecord.ID); exists {
		log.Printf("任务 %s (ID: %d) 已在运行中，跳过执行", actionRecord.Name, actionRecord.ID)
		return
	}

	timeout := time.Duration(actionRecord.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > 60*time.Second {
		timeout = 60 * time.Second
	}

	taskCtx, taskCancel := context.WithTimeout(context.Background(), timeout)

	// 创建运行任务信息
	taskInfo := &runningTaskInfo{
		Action: actionRecord,
		Ctx:    taskCtx,
		Cancel: taskCancel,
	}

	runningTasks.Store(actionRecord.ID, taskInfo)
	defer func() {
		runningTasks.Delete(actionRecord.ID)
		taskCancel()
	}()

	actionRecord.Status = "RUNNING"
	actionRecord.UpdatedAt = time.Now()
	config.Db.Save(actionRecord)

	log.Printf("开始执行任务 %s (ID: %d)", actionRecord.Name, actionRecord.ID)

	done := make(chan error, 1)
	go func() {
		done <- executeActionCode(actionRecord, taskCtx)
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("任务 %s (ID: %d) 执行失败: %v", actionRecord.Name, actionRecord.ID, err)
			actionRecord.Status = "ERROR"
			actionRecord.Error = err.Error()
		} else {
			log.Printf("任务 %s (ID: %d) 执行成功", actionRecord.Name, actionRecord.ID)
			actionRecord.Status = "SUCCESS"
			actionRecord.Error = "" // 清空错误信息
			actionRecord.NextRun = time.Now().Add(time.Duration(actionRecord.Interval) * time.Second)
		}
	case <-taskCtx.Done():
		log.Printf("任务 %s (ID: %d) 执行超时", actionRecord.Name, actionRecord.ID)
		actionRecord.Status = "TIMEOUT"
		actionRecord.Error = "任务执行超时"
	}

	// 保存任务状态
	config.Db.Save(actionRecord)
}

// 执行Action代码
func executeActionCode(actionRecord *database.Action, ctx context.Context) error {
	// 解析配置
	var actionConfig ActionConfig
	if actionRecord.Config != "" {
		err := json.Unmarshal([]byte(actionRecord.Config), &actionConfig)
		if err != nil {
			log.Printf("解析Action配置失败: %v", err)
		}
	}

	// 如果是exec_script类型，直接处理脚本运行次数增加，不需要执行代码
	if actionConfig.Type == "exec_script" {
		if actionConfig.ScriptID > 0 && actionConfig.Num > 0 {
			err := increaseExploitRunTimes(actionConfig.ScriptID, actionConfig.Num)
			if err != nil {
				// 将错误信息写入Action的Error字段
				actionRecord.Error = err.Error()
				config.Db.Save(actionRecord)
				return fmt.Errorf("增加exploit运行次数失败: %v", err)
			}
			// 成功时清空错误信息
			actionRecord.Error = ""
			config.Db.Save(actionRecord)
		}
		return nil
	}

	// 如果是template类型，不运行，显示提示信息
	if actionConfig.Type == "template" {
		log.Printf("模版 %s 无需运行", actionRecord.Name)
		return nil
	}

	// 其他类型需要执行代码
	match := regexp.MustCompile(`^data:(code\/(?:python2|python3|golang|bash));base64,(.*)$`).FindStringSubmatch(actionRecord.Code)
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

	var new_output string
	var flags []string

	// 如果是flag_submiter类型，获取flags
	if actionConfig.Type == "flag_submiter" && actionConfig.Num > 0 {
		flags, err = getFlagsForSubmission(actionConfig.Num)
		if err != nil {
			return fmt.Errorf("获取flags失败: %v", err)
		}
		if len(flags) == 0 {
			log.Printf("没有待提交的flags")
			return nil
		}
	}

	switch fileType {
	case "code/python2":
		new_output, err = executePythonCode(code, ctx, "python2", flags)
	case "code/python3":
		new_output, err = executePythonCode(code, ctx, "python3", flags)
	case "code/golang":
		new_output, err = executeGolangCode(code, ctx, flags)
	case "code/bash":
		new_output, err = executeBashCode(code, ctx, flags)
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
		config.Db.Save(actionRecord)
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
		return "", fmt.Errorf("python执行错误: %v, stderr: %s", err, stderr.String())
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
			return "", fmt.Errorf("go编译错误: %v", err)
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
			return "", fmt.Errorf("go运行时错误: %v", err)
		}
		return goibuf.String(), nil
	case <-ctx.Done():
		return "", fmt.Errorf("go代码执行超时")
	}
}

// 执行Bash代码
func executeBashCode(code string, ctx context.Context, flags []string) (string, error) {
	// 创建临时脚本文件
	tmpFile, err := os.CreateTemp("", "0e7_bash_*.sh")
	if err != nil {
		return "", fmt.Errorf("创建临时文件失败: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// 如果有flags，在脚本开头注入flags变量
	if len(flags) > 0 {
		flagsJSON, err := json.Marshal(flags)
		if err != nil {
			return "", fmt.Errorf("序列化flags失败: %v", err)
		}
		// 在脚本开头注入flags变量
		code = fmt.Sprintf("#!/bin/bash\nflags='%s'\n%s", string(flagsJSON), code)
	} else {
		code = "#!/bin/bash\n" + code
	}

	// 写入代码到临时文件
	_, err = tmpFile.WriteString(code)
	if err != nil {
		return "", fmt.Errorf("写入临时文件失败: %v", err)
	}
	tmpFile.Close()

	// 设置执行权限
	err = os.Chmod(tmpFile.Name(), 0755)
	if err != nil {
		return "", fmt.Errorf("设置执行权限失败: %v", err)
	}

	// 执行bash脚本
	cmd := exec.CommandContext(ctx, "bash", tmpFile.Name())
	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	err = cmd.Run()
	if err != nil {
		return "", fmt.Errorf("bash执行错误: %v, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
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
	// 首先尝试解析为数组格式
	var results []FlagSubmitResult
	err := json.Unmarshal([]byte(output), &results)
	if err == nil {
		// 数组格式解析成功
		return processFlagResultsArray(results)
	}

	// 如果数组格式解析失败，尝试解析为对象格式
	var resultMap map[string]string
	err = json.Unmarshal([]byte(output), &resultMap)
	if err != nil {
		return fmt.Errorf("解析提交结果失败，既不是数组格式也不是对象格式: %v", err)
	}

	// 对象格式解析成功，转换为标准格式
	return processFlagResultsMap(resultMap)
}

// 处理数组格式的flag提交结果
func processFlagResultsArray(results []FlagSubmitResult) error {
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

// 处理对象格式的flag提交结果
func processFlagResultsMap(resultMap map[string]string) error {
	// 更新数据库中的flag状态
	for flagValue, status := range resultMap {
		var flag database.Flag
		err := config.Db.Where("flag = ?", flagValue).First(&flag).Error
		if err != nil {
			log.Printf("查找flag %s 失败: %v", flagValue, err)
			continue
		}

		// 更新状态，对象格式通常没有详细消息
		flag.Status = status
		flag.Msg = "" // 对象格式通常没有消息
		flag.UpdatedAt = time.Now()

		err = config.Db.Save(&flag).Error
		if err != nil {
			log.Printf("更新flag %s 状态失败: %v", flagValue, err)
		} else {
			log.Printf("更新flag %s 状态为: %s", flagValue, status)
		}
	}

	return nil
}

// 增加exploit的运行次数
func increaseExploitRunTimes(exploitID int, increaseNum int) error {
	var exploit database.Exploit
	err := config.Db.Where("id = ? AND is_deleted = ?", exploitID, false).First(&exploit).Error
	if err != nil {
		return fmt.Errorf("找不到ID为%d的执行脚本，可能已被删除或不存在", exploitID)
	}

	// 解析当前运行次数
	currentTimes, err := strconv.Atoi(exploit.Times)
	if err != nil {
		// 如果解析失败，默认为-2（无限运行）
		currentTimes = -2
	}

	// 如果当前是无限运行(-2)或停止(-1)，则保持原状态
	if currentTimes == -2 || currentTimes == -1 {
		log.Printf("Exploit %s (ID: %d) 当前状态为 %d，不增加运行次数", exploit.Name, exploitID, currentTimes)
		return nil
	}

	// 增加运行次数
	newTimes := currentTimes + increaseNum
	exploit.Times = strconv.Itoa(newTimes)

	err = config.Db.Save(&exploit).Error
	if err != nil {
		return fmt.Errorf("更新exploit运行次数失败: %v", err)
	}

	log.Printf("Exploit %s (ID: %d) 运行次数从 %d 增加到 %d", exploit.Name, exploitID, currentTimes, newTimes)
	return nil
}
