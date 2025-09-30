# 0E7

## AWD攻防演练工具箱

专为AWD（Attack With Defense）攻防演练比赛设计的综合性工具箱，集成漏洞利用、流量监控、自动化攻击等功能

## 功能特性

### 1. 漏洞利用管理
- **Exploit管理**: 支持多种编程语言的漏洞利用脚本
- **多语言执行**: 支持Python、Go等脚本的执行
- **定时任务**: 支持定时执行和周期性任务
- **参数化配置**: 支持环境变量、命令行参数等灵活配置
- **参数注入**: 支持将BUCKET值批量注入利用
- **结果收集**: 自动收集执行结果和输出信息
- **团队协作**: 支持多用户并行使用

### 2. 流量监控分析
- **PCAP解析**: 支持多种网络协议的数据包分析
- **实时监控**: 实时捕获和分析网络流量
- **流量可视化**: 提供直观的流量数据展示
- **协议识别**: 自动识别并解析HTTP、TCP协议
- **全文检索**：支持Bleve和Elasticsearch两种全文检索引擎

### 3. 客户端管理
- **多平台支持**: 支持Windows、Linux、macOS等主流操作系统
- **自动注册**: 客户端自动向服务器注册和心跳保持
- **任务分发**: 服务器自动向客户端分发执行任务
- **状态监控**: 实时监控客户端状态和执行进度

## 快速开始

### 环境要求

确保您的系统已安装以下环境：

- **Go 1.19+**: 后端开发环境
- **Node.js 16+**: 前端开发环境
- **npm**: 包管理工具

### 构建方式

#### 方式一：基础构建（推荐）
```bash
# 构建当前系统版本
chmod +x build.sh
./build.sh
```

#### 方式二：高级构建（请自行准备跨平台编译工具链）
```bash
# 查看所有选项
./build-advanced.sh -h

# 构建当前系统版本
./build-advanced.sh

# 构建所有支持的平台
./build-advanced.sh -a

# 发布模式构建（优化文件大小）
./build-advanced.sh -r

# 构建指定平台
./build-advanced.sh -p windows
./build-advanced.sh -p linux
./build-advanced.sh -p darwin
```

### 运行方式

#### 服务器模式
```bash
# 启动服务器
./0e7_<platform>_<arch>

# 或指定配置文件
./0e7_<platform>_<arch> -config config.ini
```

#### 客户端模式
```bash
# 启动客户端
./0e7_<platform>_<arch> -client

# 或指定服务器地址
./0e7_<platform>_<arch> -client -server https://server:6102
```

## 配置文件

### 默认配置 (config.ini)
```ini
[global]
timeout_http     = 5
timeout_download = 60
debug            = false

[server]
enable     = true
port       = 6102
db_engine  = sqlite3
server_url = https://localhost:6102
flag       = flag{.*}
tls        = 
pcap_zip   = 

[client]
enable     = true
id         = 
name       = 0e7
server_url = https://remotehost:6102
pypi       = https://pypi.tuna.tsinghua.edu.cn/simple
update     = true
worker     = 
monitor    = 
```

## 项目结构
```
0E7/
├── frontend/              # 前端代码
├── service/               # 后端服务
│   ├── client/            # 客户端代码
│   ├── config/            # 配置管理
│   ├── database/          # 数据库模型
│   ├── route/             # API路由
│   ├── webui/             # Web界面
│   └── pcap/              # PCAP处理
├── build.sh               # 基础构建脚本
├── build-advanced.sh      # 高级构建脚本
├── config.ini             # 配置文件
└── 0e7.go                 # 主程序入口
```

## 许可证

本项目采用 AGPL-3.0 许可证，详情请查看 [LICENSE](LICENSE) 文件。

---
