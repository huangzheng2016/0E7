package update

import (
	"0E7/service/config"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
)

var Sha256Hash []string

func calculateSha256(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	sha256Hash := sha256.New()
	if _, err := io.Copy(sha256Hash, file); err != nil {
		return err
	}
	sha256Sum := sha256Hash.Sum(nil)
	sha256 := hex.EncodeToString(sha256Sum[:])
	Sha256Hash = append(Sha256Hash, sha256)
	log.Println(filePath, "\tSHA256:", sha256)
	return nil
}
func InitUpdate() {
	if config.Server_mode {
		root := "."
		pattern := regexp.MustCompile(`^0e7_[^_]+_[^_]+$`)
		err := filepath.WalkDir(root, func(path string, info os.DirEntry, err error) error {
			if err != nil {
				log.Printf("Folder Walk Error: %v\n", err)
				return nil
			}
			if !info.IsDir() {
				if pattern.MatchString(info.Name()) {
					err = calculateSha256(info.Name())
					if err != nil {
						log.Println(err)
					}
				}
			}
			return nil
		})
		if err != nil {
			log.Println(err)
		}
	} else {
		absPath, err := filepath.Abs(os.Args[0])
		if err != nil {
			log.Printf("OS PATH ERROR: %v\n", err)
			return
		}
		appName := filepath.Base(absPath)
		err = calculateSha256(appName)
		if err != nil {
			log.Println(err)
		}
	}
	//log.Println("HASH:", Sha256Hash)
}
func CheckStatus() {
	log.Println("CheckStatus: ", time.Now().Format(time.DateTime))
	filename := "0e7_" + runtime.GOOS + "_" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	newFilename := "new_" + filename

	absPath, err := filepath.Abs(os.Args[0])
	if err != nil {
		log.Printf("OS PATH ERROR: %v\n", err)
		return
	}
	appName := filepath.Base(absPath)
	if appName == newFilename {
		log.Println("Detected update mode: replacing old file and starting new version")
		removeUpdate(filename)
		copy_update(newFilename, filename)

		// 获取可执行文件的目录作为工作目录，而不是当前工作目录
		execPath, err := os.Executable()
		if err != nil {
			log.Printf("Failed to get executable path: %v", err)
			// 回退到使用当前工作目录
			execPath, _ = os.Getwd()
		}
		wdPath := filepath.Dir(execPath)

		targetPath := filepath.Join(wdPath, filename)

		// 等待文件复制完成
		time.Sleep(100 * time.Millisecond)

		// 验证目标文件是否存在且可执行（带重试）
		maxRetries := 5
		for i := 0; i < maxRetries; i++ {
			if runtime.GOOS != "windows" {
				info, err := os.Stat(targetPath)
				if err != nil {
					if i < maxRetries-1 {
						log.Printf("Target file not found (attempt %d/%d): %v, retrying...", i+1, maxRetries, err)
						time.Sleep(200 * time.Millisecond)
						continue
					}
					log.Printf("Target file not found after %d attempts: %v", maxRetries, err)
					return
				}
				// 检查文件模式是否有执行权限
				if info.Mode()&0111 == 0 {
					log.Println("Warning: File does not have execute permission, attempting to fix...")
					err = os.Chmod(targetPath, 0755)
					if err != nil {
						log.Printf("Failed to set execute permission: %v", err)
						return
					}
					// 重新检查权限
					info, _ = os.Stat(targetPath)
					if info.Mode()&0111 == 0 {
						log.Printf("Failed to set execute permission after retry")
						return
					}
				}
				// 验证文件大小不为0
				if info.Size() == 0 {
					log.Printf("Target file is empty, copy may have failed")
					return
				}
			} else {
				// Windows下也验证文件存在
				info, err := os.Stat(targetPath)
				if err != nil {
					if i < maxRetries-1 {
						log.Printf("Target file not found (attempt %d/%d): %v, retrying...", i+1, maxRetries, err)
						time.Sleep(200 * time.Millisecond)
						continue
					}
					log.Printf("Target file not found after %d attempts: %v", maxRetries, err)
					return
				}
				if info.Size() == 0 {
					log.Printf("Target file is empty, copy may have failed")
					return
				}
			}
			break
		}

		// 使用绝对路径确保能找到文件
		absTargetPath, err := filepath.Abs(targetPath)
		if err != nil {
			log.Printf("Failed to get absolute path: %v", err)
			return
		}

		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			// Windows: 使用绝对路径启动，cmd.Dir已设置工作目录
			// start /b 在后台启动程序，不等待其退出
			cmd = exec.Command("cmd.exe", "/C", "start", "/b", absTargetPath)
		} else {
			// Unix-like: 直接执行程序并设置后台运行
			cmd = exec.Command(absTargetPath)
			// 设置进程组，使新进程独立于当前进程组
			setUnixProcAttr(cmd)
		}

		cmd.Dir = wdPath
		// 分离标准输入输出，避免继承
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil

		log.Printf("Starting updated process: %s (working dir: %s)", absTargetPath, wdPath)
		err = cmd.Start()
		if err != nil {
			log.Printf("Failed to start updated process: %v", err)
			return
		}

		// 延长等待时间并改进启动验证
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		// 使用更长的等待时间（2秒），给程序足够的初始化时间
		select {
		case <-time.After(2 * time.Second):
			// 进程仍在运行，说明启动成功
			log.Println("Updated process started successfully")
			cmd.Process.Release() // 释放资源，不再等待进程退出
		case err := <-done:
			if err != nil {
				log.Printf("Updated process exited immediately with error: %v", err)
				// 检查进程是否真的启动失败
				log.Println("Warning: Process exited quickly, but update may still succeed")
				// 不直接返回，继续执行退出流程
			} else {
				log.Println("Updated process exited normally")
			}
		}

		log.Println("Exiting temporary update process...")
		os.Exit(0)
	} else if appName == filename {
		// 正常模式：清理可能存在的更新缓存文件
		removeUpdate(newFilename)
	}
}
func removeUpdate(filename string) {
	log.Printf("Removing update cache: %s", filename)
	maxRetries := 10
	retryDelay := 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := os.Remove(filename)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				log.Printf("File %s does not exist, removal complete", filename)
				return
			}
			if i < maxRetries-1 {
				log.Printf("Remove fail (attempt %d/%d): %v, retrying in %v...", i+1, maxRetries, err, retryDelay)
				time.Sleep(retryDelay)
				retryDelay *= 2 // 指数退避，最多等待 5.1 秒
				if retryDelay > 5*time.Second {
					retryDelay = 5 * time.Second
				}
			} else {
				log.Printf("Failed to remove %s after %d attempts: %v", filename, maxRetries, err)
				return
			}
		} else {
			log.Printf("Successfully removed %s", filename)
			return
		}
	}
}
func copy_update(sourceFile string, destinationFile string) {
	log.Printf("Copying %s to %s", sourceFile, destinationFile)

	// 使用绝对路径
	absSourceFile, err := filepath.Abs(sourceFile)
	if err != nil {
		log.Printf("Failed to get absolute path for source: %v", err)
		absSourceFile = sourceFile
	}

	absDestFile, err := filepath.Abs(destinationFile)
	if err != nil {
		log.Printf("Failed to get absolute path for destination: %v", err)
		absDestFile = destinationFile
	}

	maxRetries := 3
	var source *os.File
	var sourceInfo os.FileInfo

	// 打开源文件，带重试
	for i := 0; i < maxRetries; i++ {
		source, err = os.Open(absSourceFile)
		if err != nil {
			if i < maxRetries-1 {
				log.Printf("Failed to open source file (attempt %d/%d): %v, retrying...", i+1, maxRetries, err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			log.Printf("Failed to open source file %s after %d attempts: %v", absSourceFile, maxRetries, err)
			return
		}
		break
	}
	defer source.Close()

	// 获取源文件信息
	sourceInfo, err = source.Stat()
	if err != nil {
		log.Printf("Failed to get source file info: %v", err)
		return
	}

	// 处理目标文件（可能是正在运行的可执行文件）
	// 在Windows下，正在运行的可执行文件会被锁定，无法直接删除或重命名
	// 我们需要尝试不同的策略来替换它
	maxReplaceRetries := 10
	replaceRetryDelay := 200 * time.Millisecond

	for i := 0; i < maxReplaceRetries; i++ {
		if runtime.GOOS == "windows" {
			// Windows: 尝试多种方式删除/重命名旧文件
			if _, err := os.Stat(absDestFile); err == nil {
				// 文件存在，尝试重命名为临时名称
				tempName := absDestFile + ".old"
				err = os.Rename(absDestFile, tempName)
				if err != nil {
					// 重命名失败，可能文件被锁定，尝试直接删除
					err = os.Remove(absDestFile)
					if err != nil {
						// 删除也失败，说明文件被锁定（程序还在运行）
						if i < maxReplaceRetries-1 {
							log.Printf("Target file is locked (attempt %d/%d), waiting for old process to exit...", i+1, maxReplaceRetries)
							time.Sleep(replaceRetryDelay)
							replaceRetryDelay *= 2 // 指数退避
							if replaceRetryDelay > 2*time.Second {
								replaceRetryDelay = 2 * time.Second
							}
							continue
						}
						log.Printf("Failed to remove/rename locked target file after %d attempts: %v", maxReplaceRetries, err)
						log.Println("Warning: Old process may still be running. Will attempt to create new file anyway.")
						// 继续尝试，有时候即使旧文件存在也能创建新文件（Windows可能会自动处理）
					} else {
						// 删除成功，清理完成
						log.Printf("Successfully removed old target file")
						break
					}
				} else {
					// 重命名成功，延迟删除临时文件
					log.Printf("Successfully renamed old target file to %s", tempName)
					go func() {
						time.Sleep(5 * time.Second) // 等待更长时间确保旧进程退出
						os.Remove(tempName)
						log.Printf("Cleaned up temporary file: %s", tempName)
					}()
					break
				}
			} else {
				// 文件不存在，可以直接创建
				break
			}
		} else {
			// 非Windows系统（Linux/Unix）：使用原子性替换策略
			// Linux下可以使用原子性替换，不需要先删除旧文件
			// 但如果旧文件存在且可能被锁定，可以尝试先检查
			if _, err := os.Stat(absDestFile); err == nil {
				// 文件存在，在Linux下即使程序正在运行也可以删除（文件会被标记为已删除但进程仍可使用）
				// 但我们仍然需要删除旧文件以便创建新文件
				err = os.Remove(absDestFile)
				if err != nil && !os.IsNotExist(err) {
					// 删除失败且不是因为文件不存在，可能是权限问题或其他问题
					if i < maxReplaceRetries-1 {
						log.Printf("Failed to remove target file (attempt %d/%d): %v, retrying...", i+1, maxReplaceRetries, err)
						time.Sleep(replaceRetryDelay)
						replaceRetryDelay *= 2
						if replaceRetryDelay > 2*time.Second {
							replaceRetryDelay = 2 * time.Second
						}
						continue
					}
					log.Printf("Failed to remove target file after %d attempts: %v", maxReplaceRetries, err)
					log.Println("Warning: Will attempt atomic replacement instead.")
					// 继续执行，尝试使用原子性替换
				}
			}
			break
		}
	}

	// 创建目标文件，使用原子性替换策略
	// 在Linux下，先创建临时文件，然后使用原子性替换，避免文件不存在的窗口期
	var tempDestFile string
	var destination *os.File

	if runtime.GOOS == "windows" {
		// Windows: 直接创建目标文件
		tempDestFile = absDestFile
	} else {
		// Linux/Unix: 先创建临时文件，然后原子性替换
		tempDestFile = absDestFile + ".tmp"
	}

	// 创建文件，带重试
	for i := 0; i < maxRetries; i++ {
		destination, err = os.Create(tempDestFile)
		if err != nil {
			if i < maxRetries-1 {
				log.Printf("Failed to create destination file (attempt %d/%d): %v, retrying...", i+1, maxRetries, err)
				time.Sleep(100 * time.Millisecond)
				continue
			}
			log.Printf("Failed to create destination file %s after %d attempts: %v", tempDestFile, maxRetries, err)
			return
		}
		break
	}
	defer destination.Close()

	copied, err := io.Copy(destination, source)
	if err != nil {
		os.Remove(tempDestFile) // 复制失败时清理临时文件
		log.Printf("Failed to copy file: %v", err)
		return
	}

	// 验证复制的大小
	if copied != sourceInfo.Size() {
		os.Remove(tempDestFile)
		log.Printf("File size mismatch: expected %d, got %d", sourceInfo.Size(), copied)
		return
	}

	// 在非 Windows 系统上设置执行权限（在替换前设置）
	if runtime.GOOS != "windows" {
		err = os.Chmod(tempDestFile, 0755)
		if err != nil {
			os.Remove(tempDestFile)
			log.Printf("Failed to set executable permission: %v", err)
			return
		}
	}

	// 确保文件写入磁盘
	err = destination.Sync()
	if err != nil {
		log.Printf("Warning: Failed to sync file to disk: %v", err)
	}

	err = destination.Close()
	if err != nil {
		log.Printf("Warning: Failed to close destination file: %v", err)
	}

	// 在Linux下使用原子性替换
	if runtime.GOOS != "windows" {
		// 使用rename进行原子性替换（这是Linux下的标准做法）
		err = os.Rename(tempDestFile, absDestFile)
		if err != nil {
			os.Remove(tempDestFile) // 替换失败时清理临时文件
			log.Printf("Failed to atomically replace file: %v", err)
			log.Printf("Attempting direct write as fallback...")
			// 回退到直接创建目标文件
			tempDestFile = absDestFile
			destination, err = os.Create(tempDestFile)
			if err != nil {
				log.Printf("Fallback failed: %v", err)
				return
			}
			// 重新复制
			source.Seek(0, 0) // 重置源文件指针
			copied, err = io.Copy(destination, source)
			destination.Close()
			if err != nil || copied != sourceInfo.Size() {
				os.Remove(tempDestFile)
				log.Printf("Fallback copy failed: %v", err)
				return
			}
			// 设置权限
			err = os.Chmod(tempDestFile, 0755)
			if err != nil {
				log.Printf("Warning: Failed to set executable permission: %v", err)
			}
		} else {
			log.Printf("Atomically replaced %s with %s", absDestFile, tempDestFile)
		}
	} else {
		// Windows: 文件已经直接创建到目标位置
		tempDestFile = absDestFile
	}

	log.Printf("Successfully copied %s to %s (%d bytes)", absSourceFile, absDestFile, copied)
}
