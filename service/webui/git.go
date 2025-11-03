package webui

import (
	"0E7/service/config"
	"0E7/service/git"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// GitRepoInfo Git 仓库信息
type GitRepoInfo struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// git_repo_list 获取所有 Git 仓库列表
func git_repo_list(c *gin.Context) {
	gitDir := "git"
	
	// 检查 git 目录是否存在
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		c.JSON(200, gin.H{
			"status": "success",
			"data":   []GitRepoInfo{},
		})
		return
	}

	var repos []GitRepoInfo
	
	// 获取服务器 URL
	serverURL := config.Server_url
	if serverURL == "" {
		// 如果没有配置，使用默认值
		if config.Server_tls {
			serverURL = fmt.Sprintf("https://localhost:%s", config.Server_port)
		} else {
			serverURL = fmt.Sprintf("http://localhost:%s", config.Server_port)
		}
	}

	// 遍历 git 目录
	entries, err := os.ReadDir(gitDir)
	if err != nil {
		log.Printf("读取 git 目录失败: %v", err)
		c.JSON(500, gin.H{
			"status": "error",
			"msg":    "读取仓库列表失败",
		})
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		repoPath := filepath.Join(gitDir, entry.Name())
		headPath := filepath.Join(repoPath, "HEAD")
		objectsPath := filepath.Join(repoPath, "objects")

		// 检查是否是有效的 bare 仓库
		if _, err := os.Stat(headPath); err != nil {
			continue
		}
		if _, err := os.Stat(objectsPath); err != nil {
			continue
		}

		// 读取描述文件
		descriptionPath := filepath.Join(repoPath, "description")
		description := "Unnamed repository; edit this file 'description' to name the repository."
		if descData, err := ioutil.ReadFile(descriptionPath); err == nil {
			desc := strings.TrimSpace(string(descData))
			if desc != "" && desc != "Unnamed repository; edit this file 'description' to name the repository." {
				description = desc
			} else {
				description = "" // 如果还是默认值，显示为空
			}
		}

		// 构建仓库 URL
		repoURL := fmt.Sprintf("%s/git/%s", serverURL, entry.Name())

		repos = append(repos, GitRepoInfo{
			Name:        entry.Name(),
			URL:         repoURL,
			Description: description,
		})
	}

	c.JSON(200, gin.H{
		"status": "success",
		"data":   repos,
	})
}

// git_repo_update_description 更新仓库描述
func git_repo_update_description(c *gin.Context) {
	repoName := c.PostForm("name")
	description := c.PostForm("description")

	if repoName == "" {
		c.JSON(400, gin.H{
			"status": "error",
			"msg":    "仓库名称不能为空",
		})
		return
	}

	// 如果以 .git 结尾，去除后缀
	repoName = strings.TrimSuffix(repoName, ".git")

	// 验证仓库名称（使用统一的验证函数）
	if !git.ValidateRepoName(repoName) {
		c.JSON(400, gin.H{
			"status": "error",
			"msg":    "无效的仓库名称，只能包含字母、数字、连字符(-)和下划线(_)，且不能以连字符或下划线开头或结尾",
		})
		return
	}

	repoPath := filepath.Join("git", repoName)
	descriptionPath := filepath.Join(repoPath, "description")

	// 检查仓库是否存在
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		c.JSON(404, gin.H{
			"status": "error",
			"msg":    "仓库不存在",
		})
		return
	}

	// 写入描述文件
	if err := ioutil.WriteFile(descriptionPath, []byte(description), 0644); err != nil {
		log.Printf("更新仓库描述失败: %v", err)
		c.JSON(500, gin.H{
			"status": "error",
			"msg":    "更新描述失败",
		})
		return
	}

	c.JSON(200, gin.H{
		"status": "success",
		"msg":    "描述更新成功",
	})
}

// git_repo_delete 删除仓库
func git_repo_delete(c *gin.Context) {
	repoName := c.PostForm("name")

	if repoName == "" {
		c.JSON(400, gin.H{
			"status": "error",
			"msg":    "仓库名称不能为空",
		})
		return
	}

	// 如果以 .git 结尾，去除后缀
	repoName = strings.TrimSuffix(repoName, ".git")

	// 验证仓库名称（使用统一的验证函数）
	if !git.ValidateRepoName(repoName) {
		c.JSON(400, gin.H{
			"status": "error",
			"msg":    "无效的仓库名称，只能包含字母、数字、连字符(-)和下划线(_)，且不能以连字符或下划线开头或结尾",
		})
		return
	}

	repoPath := filepath.Join("git", repoName)

	// 检查仓库是否存在
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		c.JSON(404, gin.H{
			"status": "error",
			"msg":    "仓库不存在",
		})
		return
	}

	// 删除仓库目录
	if err := os.RemoveAll(repoPath); err != nil {
		log.Printf("删除仓库失败: %v", err)
		c.JSON(500, gin.H{
			"status": "error",
			"msg":    "删除仓库失败",
		})
		return
	}

	c.JSON(200, gin.H{
		"status": "success",
		"msg":    "仓库删除成功",
	})
}

