#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ELECTRON_DIR="${ROOT_DIR}/electron"
RESOURCES_DIR="${ELECTRON_DIR}/resources"
BIN_DIR="${RESOURCES_DIR}/bin"

rename_archives() {
  if [ ! -d "${ELECTRON_DIR}/release" ]; then
    return
  fi
  while IFS= read -r -d '' file; do
    new_file="${file/_x64./_amd64.}"
    if [ "$file" != "$new_file" ]; then
      mv "$file" "$new_file"
    fi
  done < <(find "${ELECTRON_DIR}/release" -type f -name '0e7_electron_*_x64.*' -print0)
}

usage() {
  cat <<'EOF'
用法: ./build-electron.sh [选项]

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
  echo "==> 安装 Electron 依赖"
  npm --prefix "${ELECTRON_DIR}" install --loglevel=error --fund=false
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

prepare_binary() {
  local os="$1"
  local arch="$2"
  local binary_name
  binary_name=$(backend_filename "$os" "$arch")
  local source_path="${ROOT_DIR}/${binary_name}"
  if [[ ! -f "$source_path" ]]; then
    echo "未找到后端二进制：${source_path}"
    exit 1
  fi
  rm -rf "${BIN_DIR}"
  mkdir -p "${BIN_DIR}"
  cp "$source_path" "${BIN_DIR}/"
  chmod +x "${BIN_DIR}/${binary_name}"
}

map_arch_flag() {
  case "$1" in
    amd64) echo "x64" ;;
    arm64) echo "arm64" ;;
    *) echo "$1" ;;
  esac
}

run_electron_build() {
  local os="$1"
  local arch="$2"
  local arch_flag
  arch_flag=$(map_arch_flag "$arch")

  case "$os" in
    darwin)
      npm --prefix "${ELECTRON_DIR}" run build:mac -- "--${arch_flag}"
      ;;
    linux)
      npm --prefix "${ELECTRON_DIR}" run build:linux -- "--${arch_flag}"
      ;;
    windows)
      npm --prefix "${ELECTRON_DIR}" run build:win -- "--${arch_flag}"
      ;;
    *)
      echo "未知的构建目标: ${os}"
      exit 1
      ;;
  esac
  
  # 删除生成的 blockmap 文件
  find "${ELECTRON_DIR}/release" -name "*.blockmap" -type f -delete 2>/dev/null || true
}

ensure_dependencies

for target in "${TARGETS[@]}"; do
  IFS='/' read -r os arch <<< "$target"
  echo "==> 打包 ${os}/${arch}"
  prepare_binary "$os" "$arch"
  run_electron_build "$os" "$arch"
done

rename_archives

echo "Electron 打包完成，产物位于 ${ELECTRON_DIR}/release"

