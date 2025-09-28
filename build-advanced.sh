#!/bin/bash

# 0E7 高级自动构建脚本
# 支持多平台构建、清理、压缩等功能

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 显示帮助信息
show_help() {
    echo "0E7 构建脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help          显示帮助信息"
    echo "  -c, --clean         构建前清理dist目录"
    echo "  -z, --compress      构建后压缩文件"
    echo "  -a, --all           构建所有支持的平台（包括额外架构）"
    echo "  -p, --platform      指定平台 (windows/linux/darwin)"
    echo "  -r, --release       发布模式构建（优化大小）"
    echo "  -v, --verbose       详细输出"
    echo ""
    echo "示例:"
    echo "  $0                  # 构建主要平台"
    echo "  $0 -c               # 清理后构建"
    echo "  $0 -a -z            # 构建所有平台并压缩"
    echo "  $0 -p windows       # 只构建Windows"
    echo "  $0 -r               # 发布模式构建"
}

# 默认参数
CLEAN=false
COMPRESS=false
BUILD_ALL=false
PLATFORM=""
RELEASE=false
VERBOSE=false

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        -z|--compress)
            COMPRESS=true
            shift
            ;;
        -a|--all)
            BUILD_ALL=true
            shift
            ;;
        -p|--platform)
            PLATFORM="$2"
            shift 2
            ;;
        -r|--release)
            RELEASE=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        *)
            print_error "未知参数: $1"
            show_help
            exit 1
            ;;
    esac
done

print_info "=== 0E7 构建脚本 ==="

# 清理构建文件
if [ "$CLEAN" = true ]; then
    print_info "清理构建文件..."
    rm -f 0e7_*
    print_success "清理完成"
fi

# 构建前端
print_info "1. 构建前端..."
cd frontend
print_info "  安装前端依赖..."
if [ "$VERBOSE" = true ]; then
    npm install
else
    npm install > /dev/null 2>&1
fi

print_info "  构建前端资源..."
if [ "$VERBOSE" = true ]; then
    npm run build-only
else
    npm run build-only > /dev/null 2>&1
fi
cd ..
print_success "前端构建完成"

# 整理Go模块
print_info "2. 整理Go模块..."
go mod tidy
print_success "Go模块整理完成"

# 准备构建
print_info "3. 准备构建..."
print_success "构建目录: 根目录"

# 设置构建标志
BUILD_FLAGS=""
if [ "$RELEASE" = true ]; then
    BUILD_FLAGS="-ldflags='-s -w'"
    print_info "使用发布模式构建（优化大小）"
fi

# 定义构建目标平台
if [ "$BUILD_ALL" = true ]; then
    # 所有支持的平台
    PLATFORMS=(
        "windows/amd64"
        "windows/386"
        "linux/amd64"
        "linux/386"
        "linux/arm64"
        "linux/arm"
        "darwin/amd64"
        "darwin/arm64"
        "freebsd/amd64"
        "freebsd/386"
        "openbsd/amd64"
        "openbsd/386"
        "netbsd/amd64"
        "netbsd/386"
        "solaris/amd64"
    )
    print_info "构建所有支持的平台"
else
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
    print_info "构建当前系统: $CURRENT_OS/$CURRENT_ARCH"
fi

# 如果指定了平台，只构建该平台
if [ -n "$PLATFORM" ]; then
    PLATFORMS=()
    case $PLATFORM in
        windows)
            PLATFORMS=("windows/amd64" "windows/386")
            ;;
        linux)
            PLATFORMS=("linux/amd64" "linux/386" "linux/arm64")
            ;;
        darwin)
            PLATFORMS=("darwin/amd64" "darwin/arm64")
            ;;
        *)
            print_error "不支持的平台: $PLATFORM"
            exit 1
            ;;
    esac
    print_info "只构建 $PLATFORM 平台"
fi

print_info "4. 开始Go构建..."
if [ "$BUILD_ALL" = false ] && [ -z "$PLATFORM" ]; then
    print_info "  检测到当前系统: $CURRENT_OS/$CURRENT_ARCH"
fi
print_info "  构建目标平台: ${PLATFORMS[*]}"

# 构建计数器
BUILD_COUNT=0
SUCCESS_COUNT=0

# 构建所有平台
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r os arch <<< "$platform"
    
    print_info "  构建 $os/$arch..."
    BUILD_COUNT=$((BUILD_COUNT + 1))
    
    # 设置输出文件名
    if [ "$os" = "windows" ]; then
        output_name="0e7_${os}_${arch}.exe"
    else
        output_name="0e7_${os}_${arch}"
    fi
    
    # 设置环境变量并构建
    if [ "$VERBOSE" = true ]; then
        GOOS=$os GOARCH=$arch go build $BUILD_FLAGS -o "$output_name" .
    else
        GOOS=$os GOARCH=$arch go build $BUILD_FLAGS -o "$output_name" . > /dev/null 2>&1
    fi
    
    if [ $? -eq 0 ]; then
        print_success "    $output_name 构建成功"
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    else
        print_error "    $output_name 构建失败"
    fi
done

print_info "5. 构建统计..."
print_info "  总构建数: $BUILD_COUNT"
print_success "  成功数: $SUCCESS_COUNT"
if [ $SUCCESS_COUNT -lt $BUILD_COUNT ]; then
    print_warning "  失败数: $((BUILD_COUNT - SUCCESS_COUNT))"
fi

# 压缩文件
if [ "$COMPRESS" = true ]; then
    print_info "6. 压缩构建文件..."
    for file in 0e7_*; do
        if [ -f "$file" ]; then
            print_info "  压缩 $file..."
            if command -v gzip >/dev/null 2>&1; then
                gzip -k "$file"
                print_success "    $file.gz 创建成功"
            else
                print_warning "    gzip 不可用，跳过压缩"
            fi
        fi
    done
    print_success "压缩完成"
fi

print_info "7. 构建完成！"
echo ""
print_info "=== 构建结果 ==="
print_info "构建文件位于根目录："
ls -la 0e7_*

echo ""
print_info "=== 文件说明 ==="
echo "0e7_linux_amd64        - Linux 64位"
echo "0e7_linux_386          - Linux 32位"
echo "0e7_linux_arm64        - Linux ARM64"
echo "0e7_linux_arm          - Linux ARM"
echo "0e7_darwin_amd64       - macOS Intel 64位"
echo "0e7_darwin_arm64       - macOS Apple Silicon 64位"
echo "0e7_windows_amd64.exe  - Windows 64位"
echo "0e7_windows_386.exe    - Windows 32位"
echo "0e7_freebsd_amd64      - FreeBSD 64位"
echo "0e7_freebsd_386        - FreeBSD 32位"
echo "0e7_openbsd_amd64      - OpenBSD 64位"
echo "0e7_openbsd_386        - OpenBSD 32位"
echo "0e7_netbsd_amd64       - NetBSD 64位"
echo "0e7_netbsd_386         - NetBSD 32位"
echo "0e7_solaris_amd64      - Solaris 64位"
echo ""

# 显示文件大小
print_info "=== 文件大小 ==="
for file in 0e7_*; do
    if [ -f "$file" ]; then
        size=$(du -h "$file" | cut -f1)
        echo "$(basename "$file"): $size"
    fi
done

echo ""
print_success "构建完成！🎉"

# 显示使用建议
if [ "$RELEASE" = false ]; then
    echo ""
    print_info "提示: 使用 -r 参数进行发布模式构建（优化文件大小）"
fi

if [ "$BUILD_ALL" = false ]; then
    echo ""
    print_info "提示: 使用 -a 参数构建所有支持的平台"
fi

echo ""
print_info "提示: 使用 -p <platform> 参数构建指定平台 (windows/linux/darwin)"
