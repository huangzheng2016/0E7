package git

import (
	"log"
	"os/exec"

	"github.com/gin-gonic/gin"
)

// CheckGitCommand 检查系统中是否有 git 命令
func CheckGitCommand() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// CheckAndWarnGit 检查 git 命令并在缺失时警告
func CheckAndWarnGit() {
	if !CheckGitCommand() {
		log.Println("警告: 未检测到 git 命令，Git HTTP 服务可能无法正常工作")
		log.Println("提示: 请安装 Git 以启用 Git 仓库功能")
		log.Println("     安装方法:")
		log.Println("     - macOS: brew install git")
		log.Println("     - Ubuntu/Debian: sudo apt-get install git")
		log.Println("     - CentOS/RHEL: sudo yum install git")
		log.Println("     - Windows: https://git-scm.com/download/win")
	} else {
		// 验证 git 版本
		cmd := exec.Command("git", "--version")
		output, err := cmd.Output()
		if err == nil {
			log.Printf("Git 命令已就绪: %s", string(output))
		}
	}
}

// Register 注册 Git HTTP 服务路由
func Register(router *gin.Engine) {
	// Git Smart HTTP Protocol 路由
	// 格式: /git/{仓库名}/info/refs?service=git-upload-pack 或 git-receive-pack
	// handleInfoRefs 会根据是否有 service 参数来选择处理方式
	router.GET("/git/:repo/info/refs", func(c *gin.Context) {
		service := c.Query("service")
		if service != "" {
			handleInfoRefs(c)
		} else {
			handleInfoRefsOld(c)
		}
	})

	// Git upload pack (用于 clone/fetch)
	router.POST("/git/:repo/git-upload-pack", handleUploadPack)

	// Git receive pack (用于 push)
	router.POST("/git/:repo/git-receive-pack", handleReceivePack)

	// 其他 Git HTTP 支持
	router.GET("/git/:repo/HEAD", handleHead)

	// Objects 静态文件服务（用于旧版协议）
	router.GET("/git/:repo/objects/:hash1/:hash2", handleObjects)
}
