#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WAILS_DIR="${ROOT_DIR}/wails"

usage() {
  cat <<'EOF'
用法: ./build-wails.sh [选项]

选项:
  --platform <auto|mac|linux|windows|all>  指定目标平台，默认 auto（当前系统）
  --arch <auto|amd64|arm64|all>            指定架构，默认 auto（当前系统）
  --skip-backend                           跳过 build.sh（假设二进制已存在）
  -h, --help                               显示帮助
EOF
}

PLATFORM="auto"
ARCH="auto"
SKIP_BACKEND=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --platform)
      PLATFORM="$2"
      shift
      ;;
    --all)
      PLATFORM="all"
      ;;
    --arch)
      ARCH="$2"
      shift
      ;;
    --skip-backend)
      SKIP_BACKEND=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "未知参数: $1"
      usage
      exit 1
      ;;
  esac
  shift
done

detect_host_go_os() {
  local uname_s
  uname_s=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$uname_s" in
    darwin) echo "darwin" ;;
    linux) echo "linux" ;;
    msys*|mingw*|cygwin*) echo "windows" ;;
    *) echo "linux" ;;
  esac
}

detect_host_go_arch() {
  local uname_m
  uname_m=$(uname -m)
  case "$uname_m" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) echo "amd64" ;;
  esac
}

normalize_platform() {
  case "$1" in
    mac|darwin) echo "darwin" ;;
    linux) echo "linux" ;;
    windows|win) echo "windows" ;;
    *)
      echo ""
      ;;
  esac
}

HOST_GO_OS=$(detect_host_go_os)
HOST_GO_ARCH=$(detect_host_go_arch)

if [[ "$PLATFORM" == "auto" ]]; then
  PLATFORM="$HOST_GO_OS"
elif [[ "$PLATFORM" == "all" ]]; then
  PLATFORM="all"
else
  PLATFORM=$(normalize_platform "$PLATFORM")
  if [[ -z "$PLATFORM" ]]; then
    echo "不支持的平台参数"
    exit 1
  fi
fi

ARCH=$(echo "$ARCH" | tr '[:upper:]' '[:lower:]')

case "$ARCH" in
  auto|amd64|arm64|all) ;;
  *)
    echo "不支持的架构: $ARCH"
    exit 1
    ;;
esac

supported_combo() {
  case "$1/$2" in
    darwin/amd64|darwin/arm64|linux/amd64|windows/amd64) return 0 ;;
    *) return 1 ;;
  esac
}

resolve_arches() {
  local os="$1"
  case "$ARCH" in
    auto)
      if [[ "$os" == "$HOST_GO_OS" ]]; then
        echo "$HOST_GO_ARCH"
      else
        if [[ "$os" == "darwin" && "$HOST_GO_ARCH" == "arm64" ]]; then
          echo "arm64"
        else
          echo "amd64"
        fi
      fi
      ;;
    all)
      if [[ "$os" == "darwin" ]]; then
        echo "amd64 arm64"
      else
        echo "amd64"
      fi
      ;;
    *)
      echo "$ARCH"
      ;;
  esac
}

declare -a TARGETS=()
if [[ "$PLATFORM" == "all" ]]; then
  for os in darwin linux windows; do
    for arch in $(resolve_arches "$os"); do
      if supported_combo "$os" "$arch"; then
        TARGETS+=("$os/$arch")
      fi
    done
  done
else
  for arch in $(resolve_arches "$PLATFORM"); do
    if supported_combo "$PLATFORM" "$arch"; then
      TARGETS+=("$PLATFORM/$arch")
    else
      echo "不支持的组合: $PLATFORM/$arch"
      exit 1
    fi
  done
fi

if [[ ${#TARGETS[@]} -eq 0 ]]; then
  echo "未检测到需要构建的目标"
  exit 1
fi

declare -a UNIQUE_TARGETS=()
contains_target() {
  local needle="$1"
  local item
  if [[ ${#UNIQUE_TARGETS[@]} -gt 0 ]]; then
    for item in "${UNIQUE_TARGETS[@]}"; do
      if [[ "$item" == "$needle" ]]; then
        return 0
      fi
    done
  fi
  return 1
}
for target in "${TARGETS[@]}"; do
  if ! contains_target "$target"; then
    UNIQUE_TARGETS+=("$target")
  fi
done

if [[ $SKIP_BACKEND -eq 0 ]]; then
  TARGET_STRING=""
  for target in "${UNIQUE_TARGETS[@]}"; do
    if [[ -n "$TARGET_STRING" ]]; then
      TARGET_STRING+=","
    fi
    TARGET_STRING+="$target"
  done
  echo "==> 运行 build.sh 生成后端 (${TARGET_STRING})"
  TARGET_PLATFORMS="$TARGET_STRING" "${ROOT_DIR}/build.sh"
else
  echo "==> 跳过后端构建，使用现有二进制"
fi

ensure_dependencies() {
  echo "==> 检查 Wails CLI"
  if ! command -v wails &> /dev/null; then
    echo "错误: 未找到 wails 命令"
    echo "请先安装 Wails CLI:"
    echo "  go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    exit 1
  fi
  
  echo "==> 安装前端依赖"
  npm --prefix "${ROOT_DIR}/frontend" install --loglevel=error --fund=false
  
  echo "==> 检查 Wails Go 依赖"
  cd "${ROOT_DIR}"
  if ! go list -m github.com/wailsapp/wails/v2 &> /dev/null; then
    echo "==> 安装 Wails Go 依赖"
    go get github.com/wailsapp/wails/v2
  fi
}

map_arch_flag() {
  case "$1" in
    amd64) echo "amd64" ;;
    arm64) echo "arm64" ;;
    *) echo "$1" ;;
  esac
}

backend_filename() {
  local os="$1"
  local arch="$2"
  local suffix=""
  if [[ "$os" == "windows" ]]; then
    suffix=".exe"
  fi
  echo "0e7_${os}_${arch}${suffix}"
}

run_wails_build() {
  local os="$1"
  local arch="$2"
  local arch_flag
  arch_flag=$(map_arch_flag "$arch")

  echo "==> 构建 Wails 应用 ${os}/${arch}"

  local binary_name
  binary_name=$(backend_filename "$os" "$arch")
  local source_binary="${ROOT_DIR}/${binary_name}"
  
  if [[ ! -f "$source_binary" ]]; then
    echo "错误: 未找到后端二进制文件: ${source_binary}"
    exit 1
  fi

  cd "${WAILS_DIR}"
  
  mkdir -p build
  if [[ -f "${ROOT_DIR}/electron/build/icon.icns" ]]; then
    cp "${ROOT_DIR}/electron/build/icon.icns" "build/appicon.icns"
    echo "==> 已复制图标文件: appicon.icns"
  fi
  if [[ -f "${ROOT_DIR}/electron/build/icon.png" ]]; then
    cp "${ROOT_DIR}/electron/build/icon.png" "build/appicon.png"
    echo "==> 已复制图标文件: appicon.png"
  fi
  
  mkdir -p bin
  touch "bin/backend.bin"
  cp "${source_binary}" "bin/backend.bin"
  chmod +x "bin/backend.bin"
  
  export GOOS="$os"
  export GOARCH="$arch_flag"
  
  wails build -platform "${os}/${arch_flag}" -clean -ldflags "-s -w" -skipbindings -s
  
  cd "${WAILS_DIR}/build/bin"
  
  if [[ "$os" == "windows" ]]; then
    exe_file="0e7-desktop.exe"
    if [[ -f "$exe_file" ]]; then
      echo "==> Windows exe 文件位于: ${WAILS_DIR}/build/bin/${exe_file}"
      if zip -q "0e7-wails_windows_${arch_flag}.zip" "$exe_file" 2>/dev/null; then
        echo "==> 已创建 zip 包: 0e7-wails_windows_${arch_flag}.zip"
        rm -f "$exe_file"
        echo "==> 已删除原始 exe 文件"
      else
        echo "警告: zip 打包失败，保留原始文件"
      fi
    fi
  elif [[ "$os" == "darwin" ]]; then
    app_bundle="0E7 Desktop.app"
    if [[ -d "$app_bundle" ]]; then
      echo "==> macOS 应用包位于: ${WAILS_DIR}/build/bin/${app_bundle}"
      if tar -czf "0e7-wails_darwin_${arch_flag}.tar.gz" "$app_bundle"; then
        echo "==> 已创建 tar.gz 包: 0e7-wails_darwin_${arch_flag}.tar.gz"
      fi
      
      if command -v create-dmg &> /dev/null; then
        dmg_name="0e7-wails_darwin_${arch_flag}.dmg"
        if create-dmg --overwrite "$app_bundle" . --dmg-name "$dmg_name" 2>/dev/null; then
          echo "==> 已创建 dmg 包: ${dmg_name}"
          rm -rf "$app_bundle"
          echo "==> 已删除原始 app 包"
        else
          echo "警告: dmg 打包失败，保留原始文件"
        fi
      else
        echo "提示: 安装 create-dmg 可自动生成 dmg 包"
        rm -rf "$app_bundle"
        echo "==> 已删除原始 app 包（仅保留 tar.gz）"
      fi
    fi
  elif [[ "$os" == "linux" ]]; then
    exe_file="0e7-desktop"
    if [[ -f "$exe_file" ]]; then
      chmod +x "$exe_file"
      echo "==> Linux 可执行文件位于: ${WAILS_DIR}/build/bin/${exe_file}"
      if tar -czf "0e7-wails_linux_${arch_flag}.tar.gz" "$exe_file"; then
        echo "==> 已创建 tar.gz 包: 0e7-wails_linux_${arch_flag}.tar.gz"
      fi
      
      if command -v nfpm &> /dev/null; then
        cd "${WAILS_DIR}"
        mkdir -p deb-package/usr/local/bin
        mkdir -p deb-package/usr/share/applications
        
        cp build/bin/0e7-desktop deb-package/usr/local/bin/0e7-desktop
        chmod +x deb-package/usr/local/bin/0e7-desktop
        
        cat > deb-package/usr/share/applications/0e7-desktop.desktop <<'EOF'
[Desktop Entry]
Name=0E7 Desktop
Comment=0E7 For Security
Exec=/usr/local/bin/0e7-desktop
Icon=0e7-desktop
Terminal=false
Type=Application
Categories=Utility;
EOF
        
        VERSION="${GITHUB_REF:-refs/tags/v1.0.0}"
        VERSION="${VERSION#refs/tags/v}"
        if [[ -z "$VERSION" || "$VERSION" == "refs/tags/"* ]]; then
          VERSION="1.0.0"
        fi
        
        cat > nfpm.yaml <<EOF
name: "0e7-desktop"
arch: "${arch_flag}"
platform: "linux"
version: "${VERSION}"
section: "default"
priority: "extra"
maintainer: "HydrogenE7 <huangzhengdoc@gmail.com>"
description: "0E7 Desktop Application"
vendor: "HydrogenE7"
homepage: "https://github.com/huangzheng2016/0E7"
license: "MIT"
contents:
  - src: deb-package/usr/local/bin/0e7-desktop
    dst: /usr/local/bin/0e7-desktop
  - src: deb-package/usr/share/applications/0e7-desktop.desktop
    dst: /usr/share/applications/0e7-desktop.desktop
EOF
        
        if nfpm package --packager deb --target build/bin/ 2>/dev/null; then
          echo "==> 已创建 deb 包: build/bin/0e7-desktop_${VERSION}_${arch_flag}.deb"
          cd build/bin
          rm -f 0e7-desktop
          echo "==> 已删除原始可执行文件（保留 deb 和 tar.gz）"
        else
          echo "警告: deb 打包失败，保留原始文件"
        fi
        
        cd "${WAILS_DIR}"
        rm -rf deb-package nfpm.yaml
      else
        echo "提示: 安装 nfpm 可自动生成 deb 包"
        cd build/bin
        rm -f 0e7-desktop
        echo "==> 已删除原始可执行文件（仅保留 tar.gz）"
      fi
    fi
  fi
  
  cd "${WAILS_DIR}"
  rm -rf bin
  mkdir -p bin
  echo -n "PLACEHOLDER" > bin/backend.bin
  
  echo "==> Wails 应用构建完成: ${os}/${arch}"
  echo "==> 构建产物位于: ${WAILS_DIR}/build/bin/"
}

ensure_dependencies

for target in "${TARGETS[@]}"; do
  IFS='/' read -r os arch <<< "$target"
  echo "==> 打包 ${os}/${arch}"
  run_wails_build "$os" "$arch"
done

echo "Wails 打包完成，产物位于 ${WAILS_DIR}/build/bin"

