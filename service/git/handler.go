package git

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	gitBaseDir            = "git"
	gitUploadPackService  = "git-upload-pack"
	gitReceivePackService = "git-receive-pack"
)

// initGitDir 确保 git 目录存在
func initGitDir() error {
	if _, err := os.Stat(gitBaseDir); os.IsNotExist(err) {
		err := os.MkdirAll(gitBaseDir, 0755)
		if err != nil {
			return fmt.Errorf("无法创建 git 目录: %v", err)
		}
	}
	return nil
}

// initRepo 初始化一个 Git 仓库（如果不存在）
func initRepo(repoName string) error {
	repoPath := filepath.Join(gitBaseDir, repoName)

	// 检查仓库是否存在（bare 仓库的特征：存在 HEAD 和 objects 目录）
	headPath := filepath.Join(repoPath, "HEAD")
	objectsPath := filepath.Join(repoPath, "objects")
	if _, err := os.Stat(headPath); err == nil {
		if _, err := os.Stat(objectsPath); err == nil {
			return nil // 仓库已存在（bare 仓库）
		}
	}

	// 检查是否存在同名文件或非仓库目录
	if info, err := os.Stat(repoPath); err == nil && !info.IsDir() {
		return fmt.Errorf("仓库名称已存在但不是一个目录")
	}

	// 创建仓库目录
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return fmt.Errorf("无法创建仓库目录: %v", err)
	}

	// 初始化 Git 仓库
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = repoPath
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("无法初始化 Git 仓库: %v, stderr: %s", err, stderr.String())
	}

	log.Printf("已初始化 Git 仓库: %s", repoName)
	return nil
}

// updateHEADIfNeeded 检查并更新 HEAD 指向一个存在的分支
// 如果 HEAD 指向的分支不存在，则更新为第一个找到的分支
func updateHEADIfNeeded(repoPath string) error {
	headPath := filepath.Join(repoPath, "HEAD")
	refsHeadsPath := filepath.Join(repoPath, "refs", "heads")

	// 读取当前的 HEAD 内容
	headContent, err := os.ReadFile(headPath)
	if err != nil {
		return fmt.Errorf("无法读取 HEAD 文件: %v", err)
	}

	headContentStr := strings.TrimSpace(string(headContent))
	// HEAD 文件格式通常是: ref: refs/heads/main 或 ref: refs/heads/master
	if !strings.HasPrefix(headContentStr, "ref: refs/heads/") {
		// HEAD 不是指向分支引用，可能是 detached HEAD，不需要更新
		return nil
	}

	// 提取分支名称
	branchName := strings.TrimPrefix(headContentStr, "ref: refs/heads/")
	branchRefPath := filepath.Join(refsHeadsPath, branchName)

	// 检查分支是否存在
	if _, err := os.Stat(branchRefPath); err == nil {
		// 分支存在，不需要更新
		return nil
	}

	// 分支不存在，查找第一个存在的分支
	entries, err := os.ReadDir(refsHeadsPath)
	if err != nil {
		// refs/heads 目录不存在或为空，这是正常的（空仓库）
		return nil
	}

	// 查找第一个有效的分支引用
	var firstBranch string
	for _, entry := range entries {
		if !entry.IsDir() {
			firstBranch = entry.Name()
			break
		}
	}

	if firstBranch == "" {
		// 没有找到任何分支，保持原样
		return nil
	}

	// 更新 HEAD 指向找到的第一个分支
	newHEADContent := fmt.Sprintf("ref: refs/heads/%s\n", firstBranch)
	if err := os.WriteFile(headPath, []byte(newHEADContent), 0644); err != nil {
		return fmt.Errorf("无法更新 HEAD 文件: %v", err)
	}

	log.Printf("已更新 HEAD: %s -> refs/heads/%s", branchName, firstBranch)
	return nil
}

// validateRepoName 验证仓库名称是否合法
func validateRepoName(repoName string) bool {
	if repoName == "" || len(repoName) > 255 {
		return false
	}
	// 禁止包含路径分隔符和特殊字符
	if strings.Contains(repoName, "/") || strings.Contains(repoName, "..") {
		return false
	}
	// 禁止以 . 开头或结尾
	if strings.HasPrefix(repoName, ".") || strings.HasSuffix(repoName, ".") {
		return false
	}
	// 禁止包含空格和控制字符
	if strings.ContainsAny(repoName, " \t\n\r") {
		return false
	}
	return true
}

// validateHash 验证 Git 对象哈希值格式
// hash1: 2位十六进制（SHA-1 的前2位）
// hash2: 38位十六进制（SHA-1 的后38位）
func validateHash(hash string, expectedLen int) bool {
	if len(hash) != expectedLen {
		return false
	}
	for _, r := range hash {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// handleInfoRefs 处理 info/refs 请求
func handleInfoRefs(c *gin.Context) {
	repoName := c.Param("repo")
	if !validateRepoName(repoName) {
		c.String(400, "无效的仓库名称")
		return
	}

	service := c.Query("service")
	if service != gitUploadPackService && service != gitReceivePackService {
		c.String(400, "无效的 service 参数")
		return
	}

	// 初始化 git 目录
	if err := initGitDir(); err != nil {
		log.Printf("初始化 git 目录失败: %v", err)
		c.String(500, "服务器错误")
		return
	}

	// 初始化仓库（如果不存在）
	if err := initRepo(repoName); err != nil {
		log.Printf("初始化仓库失败: %v", err)
		c.String(500, "无法初始化仓库")
		return
	}

	repoPath := filepath.Join(gitBaseDir, repoName)

	// 构建 git 命令（将 service 从 "git-upload-pack" 或 "git-receive-pack" 转换为子命令名称）
	// service 去掉 "git-" 前缀，变成 "upload-pack" 或 "receive-pack"
	gitSubcommand := strings.TrimPrefix(service, "git-")
	cmd := exec.Command("git", gitSubcommand, "--stateless-rpc", "--advertise-refs", ".")
	cmd.Dir = repoPath

	// 执行命令并获取输出
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("执行 git %s 失败: %v, stderr: %s", service, err, stderr.String())
		c.String(500, "服务器错误")
		return
	}

	// 设置响应头
	c.Header("Content-Type", fmt.Sprintf("application/x-%s-advertisement", service))
	c.Header("Cache-Control", "no-cache")

	// 写入 Smart HTTP Protocol 的响应格式
	// pkt-line 格式：<4位十六进制长度><数据>
	// 长度包括4字节长度头本身 + 数据长度
	serviceLine := fmt.Sprintf("# service=%s\n", service)
	// 长度计算：4（长度头本身）+ len(serviceLine)
	serviceLineLen := 4 + len(serviceLine)
	serviceLineHeader := fmt.Sprintf("%04x", serviceLineLen)
	c.Writer.Write([]byte(serviceLineHeader))
	c.Writer.Write([]byte(serviceLine))

	// 写入 flush packet（0000）
	c.Writer.Write([]byte("0000"))

	// 然后写入 git 命令的输出（已经是 pkt-line 格式）
	c.Writer.Write(stdout.Bytes())
}

// handleUploadPack 处理 git-upload-pack 请求（用于 clone/fetch）
func handleUploadPack(c *gin.Context) {
	repoName := c.Param("repo")
	if !validateRepoName(repoName) {
		c.String(400, "无效的仓库名称")
		return
	}

	// 初始化 git 目录
	if err := initGitDir(); err != nil {
		log.Printf("初始化 git 目录失败: %v", err)
		c.String(500, "服务器错误")
		return
	}

	// 初始化仓库（如果不存在）
	if err := initRepo(repoName); err != nil {
		log.Printf("初始化仓库失败: %v", err)
		c.String(500, "无法初始化仓库")
		return
	}

	repoPath := filepath.Join(gitBaseDir, repoName)

	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(400, "无法读取请求体")
		return
	}

	// 构建 git 命令（使用子命令名称 "upload-pack"，不是 "git-upload-pack"）
	cmd := exec.Command("git", "upload-pack", "--stateless-rpc", ".")
	cmd.Dir = repoPath

	// 设置输入输出
	cmd.Stdin = bytes.NewReader(body)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行命令
	if err := cmd.Run(); err != nil {
		// 记录详细错误信息
		log.Printf("执行 git %s 失败: %v", gitUploadPackService, err)
		if stderr.Len() > 0 {
			log.Printf("Git stderr: %s", stderr.String())
		}
		if stdout.Len() > 0 {
			log.Printf("Git stdout: %s", stdout.String())
		}

		// git-upload-pack 即使有错误，也可能在 stdout 中有部分输出
		// 返回所有输出让客户端处理
		c.Header("Content-Type", fmt.Sprintf("application/x-%s-result", gitUploadPackService))
		c.Header("Cache-Control", "no-cache")

		// 返回 stdout（git-upload-pack 的错误通常也在 stdout 中）
		if stdout.Len() > 0 {
			c.Data(200, "", stdout.Bytes())
		} else {
			c.String(500, "服务器错误")
		}
		return
	}

	// 设置响应头
	c.Header("Content-Type", fmt.Sprintf("application/x-%s-result", gitUploadPackService))
	c.Header("Cache-Control", "no-cache")

	// 写入输出
	c.Data(200, "", stdout.Bytes())
}

// handleReceivePack 处理 git-receive-pack 请求（用于 push）
func handleReceivePack(c *gin.Context) {
	repoName := c.Param("repo")
	if !validateRepoName(repoName) {
		c.String(400, "无效的仓库名称")
		return
	}

	// 初始化 git 目录
	if err := initGitDir(); err != nil {
		log.Printf("初始化 git 目录失败: %v", err)
		c.String(500, "服务器错误")
		return
	}

	// 初始化仓库（如果不存在）
	if err := initRepo(repoName); err != nil {
		log.Printf("初始化仓库失败: %v", err)
		c.String(500, "无法初始化仓库")
		return
	}

	repoPath := filepath.Join(gitBaseDir, repoName)

	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(400, "无法读取请求体")
		return
	}

	// 构建 git 命令（使用子命令名称 "receive-pack"，不是 "git-receive-pack"）
	cmd := exec.Command("git", "receive-pack", "--stateless-rpc", ".")
	cmd.Dir = repoPath

	// 设置输入输出
	cmd.Stdin = bytes.NewReader(body)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 执行命令
	if err := cmd.Run(); err != nil {
		// 记录详细错误信息
		log.Printf("执行 git %s 失败: %v", gitReceivePackService, err)
		if stderr.Len() > 0 {
			log.Printf("Git stderr: %s", stderr.String())
		}
		if stdout.Len() > 0 {
			log.Printf("Git stdout: %s", stdout.String())
		}

		// 即使命令失败，也要返回 stdout，因为 git-receive-pack 可能会在 stderr 中返回错误
		// 但在协议层面仍然返回成功（200），让 git 客户端自己处理错误
		// 如果返回错误状态码，git 客户端可能无法解析响应
		c.Header("Content-Type", fmt.Sprintf("application/x-%s-result", gitReceivePackService))
		c.Header("Cache-Control", "no-cache")

		// 返回 stdout 和 stderr 的合并输出
		// git-receive-pack 会将错误信息通过协议返回
		combinedOutput := append(stdout.Bytes(), stderr.Bytes()...)
		if len(combinedOutput) > 0 {
			c.Data(200, "", combinedOutput)
		} else {
			// 如果没有输出，返回一个错误响应（pkt-line 格式）
			c.Data(200, "", []byte("0031ERR refs/heads/main: failed to update\n"))
		}
		return
	}

	// push 成功后，确保 HEAD 指向一个存在的分支
	// 这可以避免 "warning: remote HEAD refers to nonexistent ref" 警告
	if err := updateHEADIfNeeded(repoPath); err != nil {
		log.Printf("更新 HEAD 失败: %v（可忽略）", err)
	}

	// 设置响应头
	c.Header("Content-Type", fmt.Sprintf("application/x-%s-result", gitReceivePackService))
	c.Header("Cache-Control", "no-cache")

	// 写入输出
	c.Data(200, "", stdout.Bytes())
}

// handleInfoRefsOld 处理旧版协议（可选）
func handleInfoRefsOld(c *gin.Context) {
	repoName := c.Param("repo")
	if !validateRepoName(repoName) {
		c.String(400, "无效的仓库名称")
		return
	}

	// 初始化 git 目录
	if err := initGitDir(); err != nil {
		log.Printf("初始化 git 目录失败: %v", err)
		c.String(500, "服务器错误")
		return
	}

	// 初始化仓库（如果不存在）
	if err := initRepo(repoName); err != nil {
		log.Printf("初始化仓库失败: %v", err)
		c.String(500, "无法初始化仓库")
		return
	}

	repoPath := filepath.Join(gitBaseDir, repoName)

	// 执行 git update-server-info（用于旧版协议）
	cmd := exec.Command("git", "update-server-info")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		log.Printf("执行 git update-server-info 失败: %v", err)
	}

	// 读取 info/refs 文件
	infoRefsPath := filepath.Join(repoPath, "info", "refs")
	data, err := os.ReadFile(infoRefsPath)
	if err != nil {
		c.String(404, "仓库不存在或无效")
		return
	}

	c.Data(200, "text/plain; charset=utf-8", data)
}

// handleHead 处理 HEAD 请求（返回默认分支）
func handleHead(c *gin.Context) {
	repoName := c.Param("repo")
	if !validateRepoName(repoName) {
		c.String(400, "无效的仓库名称")
		return
	}

	repoPath := filepath.Join(gitBaseDir, repoName)
	headPath := filepath.Join(repoPath, "HEAD")

	data, err := os.ReadFile(headPath)
	if err != nil {
		c.String(404, "仓库不存在或无效")
		return
	}

	c.Data(200, "text/plain", data)
}

// handleObjects 处理 objects 请求（用于旧版协议）
func handleObjects(c *gin.Context) {
	repoName := c.Param("repo")
	if !validateRepoName(repoName) {
		c.String(400, "无效的仓库名称")
		return
	}

	hash1 := c.Param("hash1")
	hash2 := c.Param("hash2")

	// 验证哈希值格式（hash1 是 2 位，hash2 是 38 位，组合成 40 位 SHA-1）
	if !validateHash(hash1, 2) || !validateHash(hash2, 38) {
		c.String(400, "无效的哈希值格式")
		return
	}

	// 直接从文件系统提供静态文件
	repoPath := filepath.Join(gitBaseDir, repoName)
	objectPath := filepath.Join(repoPath, "objects", hash1, hash2)

	// 安全检查：确保路径在仓库内（防止路径遍历攻击）
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		c.String(500, "服务器错误")
		return
	}
	absObjectPath, err := filepath.Abs(objectPath)
	if err != nil {
		c.String(500, "服务器错误")
		return
	}
	if !strings.HasPrefix(absObjectPath, absRepoPath) {
		c.String(400, "无效的路径")
		return
	}

	data, err := os.ReadFile(objectPath)
	if err != nil {
		c.String(404, "对象不存在")
		return
	}

	c.Data(200, "application/octet-stream", data)
}
