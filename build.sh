#!/bin/bash

# 0E7 自动构建脚本
# 支持多平台构建：Windows, Linux, macOS (x64/ARM64)

set -e  # 遇到错误立即退出

echo "=== 0E7 自动构建脚本 ==="
echo "开始构建..."

# 构建前端
echo "1. 构建前端..."
cd frontend
echo "  安装前端依赖..."
npm install
echo "  构建前端资源..."
npm run build-only
cd ..
echo "  前端构建完成 ✓"

# 整理Go模块
echo "2. 整理Go模块..."
go mod tidy
echo "  Go模块整理完成 ✓"

# 准备构建
echo "3. 准备构建..."
echo "  构建目录: 根目录"

# 自动检测当前系统并构建对应版本
CURRENT_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH=$(uname -m)

# 转换架构名称
case $CURRENT_ARCH in
    x86_64)
        CURRENT_ARCH="amd64"
        ;;
    arm64|aarch64)
        CURRENT_ARCH="arm64"
        ;;
    i386|i686)
        CURRENT_ARCH="386"
        ;;
    armv7l)
        CURRENT_ARCH="arm"
        ;;
esac

# 转换系统名称
case $CURRENT_OS in
    linux)
        CURRENT_OS="linux"
        ;;
    darwin)
        CURRENT_OS="darwin"
        ;;
    freebsd)
        CURRENT_OS="freebsd"
        ;;
    openbsd)
        CURRENT_OS="openbsd"
        ;;
    netbsd)
        CURRENT_OS="netbsd"
        ;;
    *)
        CURRENT_OS="linux"  # 默认使用linux
        ;;
esac

# 定义构建目标平台（默认只构建当前系统）
PLATFORMS=(
    "$CURRENT_OS/$CURRENT_ARCH"
)

# 其他架构（注释掉，需要时可以启用）
# PLATFORMS_EXTRA=(
#     "windows/amd64"
#     "windows/386"
#     "linux/386"
#     "linux/arm64"
#     "linux/arm"
#     "freebsd/amd64"
#     "freebsd/386"
#     "openbsd/amd64"
#     "openbsd/386"
#     "netbsd/amd64"
#     "netbsd/386"
#     "solaris/amd64"
# )

echo "4. 开始Go构建..."
echo "  检测到当前系统: $CURRENT_OS/$CURRENT_ARCH"
echo "  构建目标平台: ${PLATFORMS[*]}"

# 构建主要平台
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r os arch <<< "$platform"
    
    echo "  构建 $os/$arch..."
    
    # 设置输出文件名
    if [ "$os" = "windows" ]; then
        output_name="0e7_${os}_${arch}.exe"
    else
        output_name="0e7_${os}_${arch}"
    fi
    
    # 设置环境变量并构建
    GOOS=$os GOARCH=$arch go build -o "$output_name" .
    
    if [ $? -eq 0 ]; then
        echo "    ✓ $output_name 构建成功"
    else
        echo "    ✗ $output_name 构建失败"
        exit 1
    fi
done

echo "5. 构建完成！"
echo ""
echo "=== 构建结果 ==="
echo "构建文件位于根目录："
ls -la 0e7_*

echo ""
echo "=== 文件说明 ==="
echo "当前构建: 0e7_${CURRENT_OS}_${CURRENT_ARCH}$([ "$CURRENT_OS" = "windows" ] && echo ".exe" || echo "")"
echo ""
echo "如需构建其他平台，请取消注释 PLATFORMS_EXTRA 数组中的相应平台"
echo ""

# 可选：显示文件大小
echo "=== 文件大小 ==="
for file in 0e7_*; do
    if [ -f "$file" ]; then
        size=$(du -h "$file" | cut -f1)
        echo "$(basename "$file"): $size"
    fi
done

echo ""
echo "构建完成！🎉"
