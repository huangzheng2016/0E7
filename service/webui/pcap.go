package webui

import (
	"0E7/service/config"
	"0E7/service/database"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func pcap_upload(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   "upload error",
			})
		} else {
			err = savefile(file, c)
			if err != nil {
				c.JSON(400, gin.H{
					"message": "fail",
					"error":   "file save error",
				})
				return
			} else {
				c.JSON(200, gin.H{
					"message": "success",
					"error":   "upload success",
				})
				return
			}
		}
		return
	}
	files := form.File["files"]
	var err_list []string
	for _, file := range files {
		err = savefile(file, c)
		if err != nil {
			err_list = append(err_list, err.Error())
		}
	}
	files = form.File["file"]
	for _, file := range files {
		err = savefile(file, c)
		if err != nil {
			err_list = append(err_list, err.Error())
		}
	}
	if err_list != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   err_list,
		})
		return
	}
	c.JSON(200, gin.H{
		"message": "success",
		"error":   "",
	})
}

func savefile(file *multipart.FileHeader, c *gin.Context) error {
	ext := filepath.Ext(file.Filename)
	if ext != ".pcap" && ext != ".pcapng" {
		return errors.New("file type error")
	}
	newFilename := strings.TrimSuffix(file.Filename, ext) + "_" + uuid.New().String() + ext
	err := c.SaveUploadedFile(file, "pcap/"+newFilename)
	return err
}

// flow_download 提供gzip压缩的flow JSON数据、下载文件或获取文件信息
func flow_download(c *gin.Context) {
	var err error
	flowPath := c.PostForm("flow_path")
	download := c.PostForm("d") // 下载参数
	info := c.PostForm("i")     // 信息参数

	// 调试信息
	fmt.Printf("flow_download 参数: flow_path=%s, d=%s, i=%s\n", flowPath, download, info)

	if flowPath == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "flow_path parameter is required",
		})
		return
	}

	// 路径校验：确保只能访问flow目录下的文件，防止路径遍历攻击
	cleanPath := filepath.Clean(flowPath)
	flowDir := filepath.Clean("flow")

	// 检查路径是否在flow目录下
	relPath, err := filepath.Rel(flowDir, cleanPath)
	if err != nil || strings.Contains(relPath, "..") {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "invalid flow path",
		})
		return
	}

	// 检查文件是否存在
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		c.JSON(404, gin.H{
			"message": "fail",
			"error":   "flow file not found",
		})
		return
	}

	if fileInfo.IsDir() {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "path is a directory, not a file",
		})
		return
	}

	// 如果请求文件信息
	if info == "true" {
		c.JSON(200, gin.H{
			"message": "success",
			"result": gin.H{
				"size": fileInfo.Size(),
				"path": cleanPath,
			},
		})
		return
	}

	var fileData []byte
	// 默认行为：返回JSON数据
	if filepath.Ext(cleanPath) == ".gz" {
		file, err := os.Open(cleanPath)
		if err != nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   "read file failed",
			})
		}
		defer file.Close()
		reader, err := gzip.NewReader(file)
		if err != nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   "read file failed",
			})
		}
		defer reader.Close()
		fileData, err = io.ReadAll(reader)
	} else {
		file, err := os.Open(cleanPath)
		if err != nil {
			c.JSON(400, gin.H{
				"message": "fail",
				"error":   "read file failed",
			})
		}
		defer file.Close()
		fileData, err = io.ReadAll(file)
	}

	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "read file failed",
		})
	}
	// 如果请求下载文件
	if download == "true" {
		pcapId := c.PostForm("pcap_id")
		filename := fmt.Sprintf("pcap_%s.json", pcapId)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.Data(200, "application/octet-stream", fileData)
		return
	} else {
		c.Data(200, "application/json", fileData)
	}
}

// pcap_show 获取流量列表
func pcap_show(c *gin.Context) {
	page, _ := strconv.Atoi(c.PostForm("page"))
	pageSize, _ := strconv.Atoi(c.PostForm("page_size"))

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	offset := (page - 1) * pageSize

	// 构建查询条件
	query := config.Db.Model(&database.Pcap{})

	// 添加搜索条件
	if srcIP := c.PostForm("src_ip"); srcIP != "" {
		query = query.Where("src_ip LIKE ?", "%"+srcIP+"%")
	}
	if dstIP := c.PostForm("dst_ip"); dstIP != "" {
		query = query.Where("dst_ip LIKE ?", "%"+dstIP+"%")
	}
	if tags := c.PostForm("tags"); tags != "" {
		query = query.Where("tags LIKE ?", "%"+tags+"%")
	}

	// 获取总数
	var total int64
	err := query.Count(&total).Error
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "查询失败: " + err.Error(),
		})
		return
	}

	// 获取数据
	var pcaps []database.Pcap
	err = query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&pcaps).Error
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "查询失败: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "success",
		"result":  pcaps,
		"total":   total,
	})
}

// pcap_get_by_id 根据ID获取流量详情
func pcap_get_by_id(c *gin.Context) {
	idStr := c.PostForm("id")
	if idStr == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "ID参数不能为空",
		})
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "无效的ID",
		})
		return
	}

	var pcap database.Pcap
	err = config.Db.Where("id = ?", id).First(&pcap).Error
	if err != nil {
		c.JSON(404, gin.H{
			"message": "fail",
			"error":   "流量记录不存在",
		})
		return
	}

	c.JSON(200, gin.H{
		"message": "success",
		"result":  pcap,
	})
}
