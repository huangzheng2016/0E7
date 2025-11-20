#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "==> 启动 Wails 开发模式"

# 检查 Wails CLI
if ! command -v wails &> /dev/null; then
  echo "错误: 未找到 wails 命令"
  echo "请先安装 Wails CLI:"
  echo "  go install github.com/wailsapp/wails/v2/cmd/wails@latest"
  exit 1
fi

# 确保前端依赖已安装
if [ ! -d "${ROOT_DIR}/frontend/node_modules" ]; then
  echo "==> 安装前端依赖"
  npm --prefix "${ROOT_DIR}/frontend" install --loglevel=error --fund=false
fi

# 检查 Wails Go 依赖
cd "${ROOT_DIR}"
if ! go list -m github.com/wailsapp/wails/v2 &> /dev/null; then
  echo "==> 安装 Wails Go 依赖"
  go get github.com/wailsapp/wails/v2
fi

# 切换到 wails 目录执行开发模式
cd "${WAILS_DIR}"

# 启动 Wails 开发模式
wails dev -skipbindings

