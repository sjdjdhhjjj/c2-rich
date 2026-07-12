# C2 控制中心（Go 版）

> 网络安全教学演示系统 · 参考 MSF / Cobalt Strike 设计理念
>
> 讲师课堂教学用 C2 控制中心，涵盖主机管理、命令终端、文件管理、屏幕监控、Payload 生成、内网穿透、WebShell 管理、配置管理等完整功能。

## ⚠️ 免责声明

本系统**仅供网络安全教学与授权渗透测试使用**，严禁用于任何非法用途。使用本系统进行未授权操作所产生的一切后果由使用者自行承担。

---

## 技术栈

| 组件 | 技术选型 | 说明 |
|------|----------|------|
| 服务端 | Go 1.21+ | 单二进制，无运行时依赖，跨平台 |
| 客户端 (Agent) | Go 1.21+ | 纯 Go (CGO_ENABLED=0)，支持 Windows/Linux 交叉编译 |
| WebShell | PHP / JSP / ASPX | 冰蝎被动模式，不回连，C2 主动发 HTTP 请求 |
| 数据库 | SQLite | 零配置，单文件 `server-go/c2.db` |
| 前端 | 原生 HTML/CSS/JS | 无框架依赖，jQuery 3.7 仅用于图表 |
| 加密 | AES-128/256-CBC, RC4, ChaCha20, XOR | 服务端/客户端/WebShell 三方对齐 |

---

## 目录结构

```
c2/
├── c2.bat                    # Windows 启动脚本（编译 + 启动）
├── c2.sh                     # Linux 启动脚本（start/stop/restart/status/log）
├── config.json               # Web 核心配置（host/port/protocol/ssl）
├── README.md                 # 本文档
│
├── server-go/                # 服务端（Go）
│   ├── main.go               # 主入口，路由注册，HTTP 服务启动
│   ├── agent.go              # Agent 通信端点 (/agent/heartbeat, /agent/pull, /agent/result)
│   ├── auth.go               # 认证中间件 + 登录/用户管理 API
│   ├── crypto.go             # 通信加密（AES/RC4/ChaCha20/XOR）
│   ├── db.go                 # SQLite 初始化 + 查询辅助 + 日志 + 北京时间工具
│   ├── handlers.go           # 仪表盘/客户端/任务/日志/配置/Payload 下载等 API
│   ├── settings.go           # 配置管理 + config.json 加载 + 网络工具
│   ├── payload_gen.go        # Payload 生成（EXE/PHP/JSP/ASPX/BAT/PS1/SH/Python/Shellcode）
│   ├── obfuscator.go         # 代码混淆引擎（Python/PHP/JSP/ASPX）
│   ├── webshell.go           # WebShell 管理 API（冰蝎被动模式）
│   ├── websocket.go          # WebSocket 实时推送（客户端上线/任务更新）
│   ├── helpers.go            # 通用辅助函数
│   ├── go.mod / go.sum       # Go 依赖
│   ├── c2_server.exe         # 编译产物（Windows）
│   ├── c2.db                 # SQLite 数据库（自动创建）
│   ├── static/               # Web 前端资源
│   │   ├── index.html        # 入口页面
│   │   ├── css/style.css     # 样式
│   │   └── js/               # 前端模块（15 个 JS 文件）
│   ├── payloads/             # 生成的 Payload 文件存放目录
│   └── tmp/                  # 临时文件（screenshots/recordings/audio/downloads/uploads）
│
└── client-go/                # 客户端 Agent（Go）
    ├── main.go               # Agent 主循环（心跳 + 任务轮询）
    ├── config.go             # 配置加载（ldflags 注入 / 环境变量 / 命令行）
    ├── crypto.go             # 客户端加密（与服务端算法对齐）
    ├── transport.go          # HTTP 通信层（心跳/拉取任务/回传结果）
    ├── tasks.go              # 23 种任务处理器注册表
    ├── task_cmd.go           # 命令执行 (cmd/powershell/bash)
    ├── task_file.go          # 文件管理（列表/上传/下载/查看/编辑/删除/重命名/新建）
    ├── task_media.go         # 媒体采集（截图/录屏/录音/摄像头）
    ├── task_media_windows.go # Windows 专属媒体实现（GDI+ 录屏）
    ├── task_media_unix.go    # Linux 专属媒体实现（X11 截图）
    ├── task_system.go        # 系统信息/进程列表/剪贴板/持久化/痕迹清理
    ├── task_tunnel.go        # 内网穿透（端口转发/SOCKS5/HTTP 代理）
    ├── sysinfo.go            # 系统信息收集
    ├── exec.go               # 命令执行封装
    ├── taskutil.go           # 任务辅助函数
    └── go.mod / go.sum       # Go 依赖
```

---

## 快速开始

### 环境要求

- **Go 1.21+**（[下载地址](https://go.dev/dl/)）
- 浏览器（Chrome / Edge / Firefox）

### Windows 启动

```cmd
双击 c2.bat
```

`c2.bat` 会自动完成：
1. 检查 Go 环境
2. 编译 `server-go/c2_server.exe`
3. 检查/创建 `config.json`
4. 杀掉旧进程
5. 启动服务并显示监听地址

### Linux 启动

```bash
chmod +x c2.sh
./c2.sh start     # 启动（后台运行）
./c2.sh stop      # 停止
./c2.sh restart   # 重启
./c2.sh status    # 查看状态
./c2.sh log       # 查看日志
```

### 手动编译

```bash
# Windows
cd server-go
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -o c2_server.exe .

# Linux
cd server-go
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o c2_server .
```

### 访问 Web 后台

- 地址：`http://127.0.0.1:5000`（端口来自 `config.json`）
- 默认账号：`admin`
- 默认密码：`admin123`

---

## 配置文件 (config.json)

根目录 `config.json` 用于配置 Web 服务核心参数，**修改后需重启服务生效**：

```json
{
  "web": {
    "host": "0.0.0.0",
    "port": 5000,
    "protocol": "http",
    "ssl_cert": "",
    "ssl_key": ""
  }
}
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `host` | 监听地址，`0.0.0.0` 表示所有网卡 | `0.0.0.0` |
| `port` | Web 后台端口 | `5000` |
| `protocol` | 通信协议（`http` / `https`） | `http` |
| `ssl_cert` | HTTPS 证书路径（protocol=https 时必填） | 空 |
| `ssl_key` | HTTPS 私钥路径（protocol=https 时必填） | 空 |

> 其余配置（Payload 回连地址、加密设置、客户端行为、安全策略等）在 Web 后台「配置管理」页面在线编辑，存储在 SQLite 数据库中。

---

## 功能特性

### 核心功能（参考 MSF/CS）

| 模块 | 说明 | 参考工具 |
|------|------|----------|
| 主机管理 | 客户端上线、Session 生命周期管理、分组、批量操作 | MSF `sessions` / CS Beacon 列表 |
| 命令终端 | CMD / PowerShell / Bash 远程执行，↑↓ 浏览历史 | CS Beacon Console |
| 文件管理 | 目录浏览、上传下载、在线编辑任意文件、重命名删除 | CS 文件浏览器 |
| 屏幕监控 | 实时屏幕查看、可调刷新间隔、独立录屏、摄像头拍照录像 | CS 屏幕捕获 |
| Payload 生成 | EXE/PHP/JSP/ASPX/BAT/PS1/SH/Python 等多格式，CS 风格投递 URL | MSF `msfvenom` / CS Payload Generator |
| Shellcode 生成 | 支持 windows/x64、linux/x64，输出 C/Python/Hex/Raw/exe_loader | MSF `msfvenom -f c` |
| 命令生成器 | Windows/Linux 80+ 模板（信息收集/提权/横向/持久化） | MSF 命令速查 |
| 内网穿透 | 端口转发、SOCKS5 代理、HTTP 代理 | MSF `portfwd` / CS socks proxy |
| WebShell 管理 | PHP/JSP/ASPX 冰蝎被动模式，终端/文件管理/系统信息 | 冰蝎 / 哥斯拉 |
| 配置管理 | 监听配置、加密设置、用户管理、客户端参数、任务限制、安全策略、Webhook | CS Listener 配置 |
| 任务列表 | 任务下发记录查询，支持状态/类型筛选、翻页 | - |
| 日志审计 | 操作日志、事件搜索、CSV 导出、Webhook 通知 | - |

### Agent 任务类型（23 种）

| 类别 | 任务类型 | 说明 |
|------|----------|------|
| 命令执行 | `cmd` | 执行系统命令（CMD/PowerShell/Bash） |
| 系统信息 | `sysinfo` | 主机名/OS/架构/用户名/IP |
| 系统信息 | `process_list` | 进程列表 |
| 文件管理 | `file_list` | 目录浏览 |
| 文件管理 | `file_download` | 从 Agent 下载文件 |
| 文件管理 | `file_view` | 查看文件内容 |
| 文件管理 | `file_save` | 保存文件到目标 |
| 文件管理 | `file_mkdir` | 新建目录 |
| 文件管理 | `file_delete` | 删除文件/目录 |
| 文件管理 | `file_rename` | 重命名 |
| 文件管理 | `file_upload` | 上传文件到目标 |
| 媒体采集 | `screenshot` | 屏幕截图 |
| 媒体采集 | `record_screen` | 录屏（XVID/avi） |
| 媒体采集 | `record_audio` | 录音 |
| 媒体采集 | `camera_photo` | 摄像头拍照 |
| 媒体采集 | `camera_record` | 摄像头录像 |
| 键盘记录 | `keylogger_start` | 键盘记录 |
| 剪贴板 | `clipboard` | 读取剪贴板 |
| 持久化 | `persistence` | 持久化（注册表/计划任务） |
| 痕迹清理 | `clean_trace` | 清理日志/痕迹 |
| 内网穿透 | `port_forward` | 端口转发 |
| 内网穿透 | `socks5_proxy` | SOCKS5 代理 |
| 内网穿透 | `http_proxy` | HTTP 代理 |

### Payload 生成格式

| 格式 | 扩展名 | 平台 | 说明 |
|------|--------|------|------|
| EXE | `.exe` / ELF | Windows / Linux | Go 交叉编译，Windows 加 `-H windowsgui` 隐藏控制台 |
| PHP WebShell | `.php` | 跨平台 | 冰蝎被动模式，支持 AES/XOR/RC4/ChaCha20 加密 |
| JSP WebShell | `.jsp` | Tomcat | 冰蝎被动模式，Java 实现，9 种操作 |
| ASPX WebShell | `.aspx` | IIS | 冰蝎被动模式，C# 实现，含 ChaCha20 类 |
| BAT | `.bat` | Windows | 批处理木马 |
| PS1 | `.ps1` | Windows | PowerShell 脚本 |
| SH | `.sh` | Linux | Bash 脚本 |
| Python | `.py` | 跨平台 | 混淆 Python agent |
| Shellcode | `.c` / `.py` / `.hex` / `.raw` / `.exe_loader` | Windows/Linux | 5 种输出格式 |

### Shellcode Payload 类型

| Payload 类型 | 说明 |
|--------------|------|
| `windows/x64/shell_reverse_tcp` | Windows x64 反弹 Shell |
| `windows/x64/shell_bind_tcp` | Windows x64 正向 Shell |
| `windows/x64/meterpreter_reverse_tcp` | Meterpreter 反弹（兼容） |
| `windows/x64/meterpreter_reverse_http` | Meterpreter HTTP（兼容） |
| `linux/x64/shell_reverse_tcp` | Linux x64 反弹 Shell |
| `linux/x64/shell_bind_tcp` | Linux x64 正向 Shell |
| `custom/cmd` | 自定义命令 Shellcode |

### 加密算法

服务端、客户端、WebShell 三方对齐，通过 `X-Enc-Algo` 请求头协商：

| 算法 | 密钥长度 | 说明 |
|------|----------|------|
| `aes-128-cbc` | 16 字节 | AES-128-CBC，IV 随机生成 |
| `aes-256-cbc` | 32 字节 | AES-256-CBC，IV 随机生成 |
| `rc4` | 32 字节 | RC4 流加密 |
| `chacha20` | 32 字节 | ChaCha20 (IETF, 12 字节 nonce) |
| `xor` | 变长 | XOR 异或 |
| `none` | - | 不加密（仅演示） |

密钥派生：`SHA-256(password)[:16|32]`
加密格式：`base64(iv/nonce + ciphertext)`

---

## WebShell 使用（冰蝎被动模式）

### 特点

- **不回连**：WebShell 部署后不主动连接 C2，C2 直接发 HTTP POST 请求
- **不自动注册**：WebShell 不会在 C2 自动注册，需在后台手动添加
- **伪装 404**：无请求时返回伪装的 404 页面
- **加密通信**：支持 AES-128/256-CBC、XOR、RC4、ChaCha20

### 支持的操作

| 操作 | 说明 |
|------|------|
| `cmd` | 命令执行 |
| `sysinfo` | 系统信息（主机名/OS/架构/用户名/IP） |
| `file_list` | 目录浏览 |
| `file_view` | 查看文件内容 |
| `file_save` | 保存文件 |
| `file_delete` | 删除文件 |
| `file_mkdir` | 新建目录 |
| `file_rename` | 重命名 |
| `file_download` | 下载文件 |

### 使用步骤

1. 在「Payload 生成」页面生成 PHP/JSP/ASPX WebShell 文件
2. 将文件上传到目标 Web 目录（如 `/var/www/html/` 或 Tomcat `webapps/`）
3. 在「WebShell 管理」页面手动添加 WebShell URL
4. 添加时**必须使用与生成时完全一致的加密方式和密码**
5. C2 自动验证连通性，验证成功后即可在终端/文件管理中操作

> ⚠️ 加密不一致会导致 500 错误！生成时用的算法/密码 = 添加时必须填的算法/密码

---

## 通信协议

### Agent 通信

```
Agent → C2:  POST /agent/heartbeat    （心跳，含客户端信息）
C2 → Agent:  POST /agent/pull         （拉取任务）
Agent → C2:  POST /agent/result       （回传任务结果）
```

### WebShell 通信

```
C2 → WebShell: POST http://target/shell.php
  Body: base64(iv + encrypt(JSON({action, param})))
  Header: Content-Type: application/octet-stream

WebShell → C2: HTTP Response
  Body: base64(iv + encrypt(JSON({status, result})))
```

### WebSocket 实时推送

```
浏览器 → C2: ws://host:port/ws
C2 → 浏览器: {"event":"client_update"}     （客户端上线/离线）
C2 → 浏览器: {"event":"task_update"}       （任务状态变更）
```

---

## API 路由

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/login` | 登录，返回 token |
| POST | `/api/user/password` | 修改密码 |
| GET | `/api/users` | 用户列表（admin） |
| POST | `/api/users` | 创建用户（admin） |
| DELETE | `/api/users/{uid}` | 删除用户（admin） |

### 仪表盘 & 主机

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/dashboard/stats` | 仪表盘统计 |
| GET | `/api/clients` | 客户端列表 |
| DELETE | `/api/clients/{id}` | 删除客户端 |
| GET | `/api/sessions` | Session 列表 |
| POST | `/api/sessions/{id}/kill` | 终止 Session |
| GET/POST/DELETE | `/api/groups` | 分组管理 |

### 任务 & 日志

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/task/send` | 下发任务 |
| GET | `/api/tasks` | 任务列表（分页） |
| GET | `/api/logs` | 日志列表（分页） |
| GET | `/api/logs/export` | 导出 CSV |

### Payload

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/payload/generate` | 生成 Payload |
| POST | `/api/payload/shellcode/generate` | 生成 Shellcode |
| GET | `/api/payloads` | Payload 列表 |
| DELETE | `/api/payloads/{id}` | 删除 Payload |
| GET | `/api/payload/download/{filename}?token=xxx` | 下载 Payload |
| GET | `/deliver/{token}` | 投递 URL（无需认证） |

### WebShell

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/webshell` | WebShell 列表 |
| POST | `/api/webshell` | 添加 WebShell |
| DELETE | `/api/webshell/{id}` | 删除 WebShell |
| POST | `/api/webshell/{id}/exec` | 执行操作 |
| GET | `/api/webshell/{id}/test` | 测试连通性 |

### 配置

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/settings` | 获取配置 |
| PUT | `/api/settings` | 更新配置 |
| POST | `/api/settings/test` | 测试监听 |
| POST | `/api/settings/reload_config` | 重新加载 config.json |
| GET/POST | `/api/settings/webhook` | Webhook 配置 |

### Agent 端点（无需 Web 认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/agent/heartbeat` | 心跳 |
| POST | `/agent/pull` | 拉取任务 |
| POST | `/agent/result` | 回传结果 |
| GET | `/api/resource/{subdir}/{filename}?token=xxx` | 资源文件访问 |

---

## 配置管理说明

### config.json（文件配置，只读）

以下配置从根目录 `config.json` 读取，**不支持在线修改**，需编辑文件后重启服务：

- Web 监听地址（host）
- Web 端口（port）
- 通信协议（protocol）
- SSL 证书路径（ssl_cert / ssl_key）

### 数据库配置（在线编辑）

以下配置在 Web 后台「配置管理」页面在线编辑，存储在 SQLite：

- **监听配置**：Payload 回连地址、客户端监听端口、内网穿透端口
- **加密设置**：加密算法、密码、AES Key、XOR Key
- **客户端行为**：心跳间隔、任务轮询、离线超时、最大重连
- **任务限制**：文件上传大小限制
- **安全策略**：Session 超时、最大登录失败、锁定时长
- **Webhook**：企业微信/钉钉/飞书通知

> ⚠️ 修改监听参数后需重启服务才能生效

---

## 部署指南

### Windows 部署

1. 安装 Go 1.21+
2. 双击 `c2.bat`
3. 浏览器访问 `http://127.0.0.1:5000`

### Linux 部署

```bash
# 1. 安装 Go
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# 2. 启动
chmod +x c2.sh
./c2.sh start

# 3. 验证
curl http://127.0.0.1:5000/
```

### 生成 Agent Payload

1. 进入 Web 后台「Payload 生成」页面
2. 选择操作系统、架构、输出格式
3. 配置加密方式和密码（已自动同步全局配置）
4. 点击「生成 Payload」
5. 下载生成的文件，投递到目标机器运行

### 生成 WebShell

1. 选择输出格式为 PHP / JSP / ASPX
2. 配置加密方式和密码
3. 生成并下载
4. 上传到目标 Web 目录
5. 在「WebShell 管理」页面添加 URL

---

## 常见问题

### Q: 双击 c2_server.exe 无反应？

A: 服务端是控制台程序，请通过 `c2.bat` 启动，或在命令行中运行 `server-go\c2_server.exe`。

### Q: 生成的 EXE 双击运行有黑框？

A: 已通过 `-H windowsgui` 设置 PE Subsystem 为 Windows GUI，运行不弹控制台。如果仍有黑框，请重新生成 Payload。

### Q: WebShell 添加后显示 500 错误？

A: 添加 WebShell 时的加密方式和密码必须与生成时完全一致。请在「配置管理」页面确认加密设置，或重新生成 WebShell。

### Q: 修改了 config.json 端口但不生效？

A: 修改 `config.json` 后需要重启服务（`c2.bat` 或 `./c2.sh restart`）。在「配置管理」页面可点「重新加载 config.json」按钮刷新内存配置，但监听地址在启动时绑定，仍需重启。

### Q: Payload 下载 URL 不可用？

A: Payload 列表中的 `download_filename` 字段为实际文件名，下载链接格式为 `/api/payload/download/{filename}?token={token}`。投递 URL（`/deliver/{token}`）无需认证。

### Q: Shellcode 生成失败？

A: 支持的 Payload 类型：`windows/x64/shell_reverse_tcp`、`windows/x64/shell_bind_tcp`、`linux/x64/shell_reverse_tcp`、`linux/x64/shell_bind_tcp`、`custom/cmd` 等。输出格式：`c`、`python`、`hex`、`raw`、`exe_loader`。

---

## 开发说明

### 编译服务端

```bash
cd server-go
go build -o c2_server.exe .
```

### 编译客户端 Agent

```bash
cd client-go
# Windows
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o agent.exe .
# Linux
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o agent .
```

### 前端文件结构

```
server-go/static/
├── index.html              # 入口页面
├── css/style.css           # 全局样式
└── js/
    ├── api.js              # API 封装（fetch + token）
    ├── app-core.js         # 核心逻辑（路由、状态、通知）
    ├── app-render.js       # 页面渲染框架
    ├── app-dashboard.js    # 仪表盘
    ├── app-clients.js      # 主机管理
    ├── app-webshell.js     # WebShell 管理
    ├── app-terminal.js     # 命令终端
    ├── app-files.js        # 文件管理
    ├── app-media.js        # 屏幕监控
    ├── app-payloads.js     # Payload 生成
    ├── app-cmdgen.js       # 命令生成器
    ├── app-tunnel.js       # 内网穿透
    ├── app-tasks.js        # 任务列表
    ├── app-logs.js         # 日志审计
    └── app-settings.js     # 配置管理
```

### 数据库表结构

| 表 | 说明 |
|------|------|
| `users` | 用户管理 |
| `clients` | 上线客户端 |
| `tasks` | 任务记录 |
| `payloads` | 生成的 Payload |
| `logs` | 操作日志 |
| `settings` | 配置项（键值对） |
| `groups` | 客户端分组 |
| `webshells` | WebShell 列表 |

### 时间处理

所有时间使用北京时间（UTC+8），通过 `nowLocal()` 函数生成，不依赖 SQLite 的 `CURRENT_TIMESTAMP`（返回 UTC）。

### 版本更新

修改前端代码后，更新 `index.html` 中的 `?v=` 版本号参数以强制浏览器刷新缓存。
