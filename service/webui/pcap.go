package webui

import (
	"0E7/service/config"
	"0E7/service/database"
	"0E7/service/pcap"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// calculateFileMD5 计算文件的MD5值
func calculateFileMD5(file *multipart.FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, src); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

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
					"error":   err.Error(),
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

	// 处理批量上传
	var successCount int
	var err_list []string
	var skippedCount int

	// 处理 "files" 字段（批量上传）
	files := form.File["files"]
	for _, file := range files {
		err = savefile(file, c)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				skippedCount++
			} else {
				err_list = append(err_list, fmt.Sprintf("%s: %s", file.Filename, err.Error()))
			}
		} else {
			successCount++
		}
	}

	// 处理 "file" 字段（单个文件上传）
	files = form.File["file"]
	for _, file := range files {
		err = savefile(file, c)
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				skippedCount++
			} else {
				err_list = append(err_list, fmt.Sprintf("%s: %s", file.Filename, err.Error()))
			}
		} else {
			successCount++
		}
	}

	// 构建响应
	response := gin.H{
		"message":       "success",
		"success_count": successCount,
		"skipped_count": skippedCount,
		"error_count":   len(err_list),
	}

	if len(err_list) > 0 {
		response["errors"] = err_list
		response["message"] = "partial_success"
	}

	if successCount == 0 && skippedCount == 0 {
		response["message"] = "fail"
	}

	c.JSON(200, response)
}

func savefile(file *multipart.FileHeader, c *gin.Context) error {
	ext := filepath.Ext(file.Filename)
	if ext != ".pcap" && ext != ".pcapng" {
		return errors.New("file type error")
	}

	// 计算文件MD5
	fileMD5, err := calculateFileMD5(file)
	if err != nil {
		return fmt.Errorf("failed to calculate MD5: %v", err)
	}

	// 检查数据库中是否已存在相同MD5的文件
	var existingFile database.PcapFile
	err = config.Db.Where("md5 = ?", fileMD5).First(&existingFile).Error
	if err == nil {
		// 文件已存在，跳过上传
		return fmt.Errorf("file with same MD5 (%s) already exists: %s", fileMD5, existingFile.Filename)
	}

	// 生成新的文件名 时间Unix时间戳
	newFilename := strings.TrimSuffix(file.Filename, ext) + "_" + strconv.FormatInt(time.Now().Unix(), 10) + ext
	filePath := "pcap/" + newFilename

	// 先将文件信息保存到数据库（避免文件监控重复处理）
	pcapFile := database.PcapFile{
		Filename: filePath,
		ModTime:  time.Now(), // 使用当前时间作为修改时间
		FileSize: file.Size,
		MD5:      fileMD5,
	}
	err = config.Db.Create(&pcapFile).Error
	if err != nil {
		return fmt.Errorf("failed to save file info to database: %v", err)
	}

	// 保存文件到磁盘
	err = c.SaveUploadedFile(file, filePath)
	if err != nil {
		// 如果文件保存失败，删除数据库记录
		config.Db.Delete(&pcapFile)
		return fmt.Errorf("failed to save file: %v", err)
	}

	// 将上传的文件加入全局处理队列
	log.Printf("将上传的文件加入处理队列: %s", filePath)
	pcap.QueuePcapFile(filePath)

	return nil
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
	if ip := c.PostForm("ip"); ip != "" {
		// 同时搜索源IP和目标IP
		query = query.Where("src_ip LIKE ? OR dst_ip LIKE ?", "%"+ip+"%", "%"+ip+"%")
	}
	// 保持向后兼容性，如果传入了单独的src_ip或dst_ip参数
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

// pcap_download 下载pcap文件或获取文件信息
func pcap_download(c *gin.Context) {
	pcapId := c.PostForm("pcap_id")
	fileType := c.PostForm("type") // "raw", "original", "parsed"
	download := c.PostForm("d")    // 下载参数
	info := c.PostForm("i")        // 信息参数

	if pcapId == "" {
		c.JSON(400, gin.H{
			"message": "fail",
			"error":   "pcap_id parameter is required",
		})
		return
	}

	// 获取pcap记录
	var pcap database.Pcap
	err := config.Db.Where("id = ?", pcapId).First(&pcap).Error
	if err != nil {
		c.JSON(404, gin.H{
			"message": "fail",
			"error":   "pcap record not found",
		})
		return
	}

	var filePath string
	var fileName string

	if fileType == "raw" {
		// 下载原始文件（未解析的原始pcap文件）
		if pcap.Filename == "" {
			c.JSON(404, gin.H{
				"message": "fail",
				"error":   "raw pcap file not found",
			})
			return
		}
		filePath = pcap.Filename
		fileName = fmt.Sprintf("raw_%s.pcap", pcapId)
	} else if fileType == "original" {
		// 下载流量文件（重组后的pcap文件）
		if pcap.PcapFile == "" {
			c.JSON(404, gin.H{
				"message": "fail",
				"error":   "flow pcap file not found",
			})
			return
		}
		filePath = pcap.PcapFile
		fileName = fmt.Sprintf("flow_%s.pcap", pcapId)
	} else {
		// 下载解析文件（json文件）
		if pcap.FlowFile == "" {
			c.JSON(404, gin.H{
				"message": "fail",
				"error":   "parsed json file not found",
			})
			return
		}
		filePath = pcap.FlowFile
		fileName = fmt.Sprintf("parsed_%s.json", pcapId)
	}

	// 路径校验：确保只能访问指定目录下的文件
	cleanPath := filepath.Clean(filePath)

	// 检查文件是否存在
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		c.JSON(404, gin.H{
			"message": "fail",
			"error":   "file not found",
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

	// 读取文件
	var fileData []byte
	if filepath.Ext(cleanPath) == ".gz" {
		// 处理压缩文件
		file, err := os.Open(cleanPath)
		if err != nil {
			c.JSON(500, gin.H{
				"message": "fail",
				"error":   "read file failed",
			})
			return
		}
		defer file.Close()

		reader, err := gzip.NewReader(file)
		if err != nil {
			c.JSON(500, gin.H{
				"message": "fail",
				"error":   "decompress file failed",
			})
			return
		}
		defer reader.Close()

		fileData, err = io.ReadAll(reader)
		if err != nil {
			c.JSON(500, gin.H{
				"message": "fail",
				"error":   "read decompressed file failed",
			})
			return
		}
	} else {
		// 处理普通文件
		fileData, err = os.ReadFile(cleanPath)
		if err != nil {
			c.JSON(500, gin.H{
				"message": "fail",
				"error":   "read file failed",
			})
			return
		}
	}

	// 如果请求文件信息
	if info == "true" {
		result := gin.H{}

		// 获取原始文件大小
		if pcap.Filename != "" {
			if rawFileInfo, err := os.Stat(pcap.Filename); err == nil {
				result["raw_size"] = rawFileInfo.Size()
			}
		}

		// 获取流量文件大小
		if pcap.PcapFile != "" {
			if flowFileInfo, err := os.Stat(pcap.PcapFile); err == nil {
				result["flow_size"] = flowFileInfo.Size()
			}
		}

		// 获取解析文件大小
		if pcap.FlowFile != "" {
			if parsedFileInfo, err := os.Stat(pcap.FlowFile); err == nil {
				result["parsed_size"] = parsedFileInfo.Size()
			}
		}

		c.JSON(200, gin.H{
			"message": "success",
			"result":  result,
		})
		return
	}

	// 如果请求下载文件
	if download == "true" {
		// 设置下载头
		// 如果原始文件是.gz格式，下载时去掉.gz后缀
		if filepath.Ext(cleanPath) == ".gz" {
			// 去掉文件名中的.gz后缀
			fileName = strings.TrimSuffix(fileName, ".gz")
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		c.Header("Content-Type", "application/octet-stream")
		c.Data(200, "application/octet-stream", fileData)
		return
	}

	// 默认行为：返回JSON数据（仅对parsed类型）
	if fileType == "parsed" {
		c.Data(200, "application/json", fileData)
	} else {
		// 其他类型默认下载
		if filepath.Ext(cleanPath) == ".gz" {
			fileName = strings.TrimSuffix(fileName, ".gz")
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
		c.Header("Content-Type", "application/octet-stream")
		c.Data(200, "application/octet-stream", fileData)
	}
}
