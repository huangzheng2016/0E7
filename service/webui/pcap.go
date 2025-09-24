package webui

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"mime/multipart"
	"path/filepath"
	"strings"
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
