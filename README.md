# C2-Rich

基于 Go 语言的命令与控制（C2）框架，纯 Go 实现，支持 Windows/Linux 交叉编译，集成多协议通信、流量加密、代码混淆、WebShell 管理、内网穿透等功能。

> 仅供安全研究与授权渗透测试使用，禁止用于非法用途。

---

## 功能概览

### 通信协议（三种）

| 协议 | 模式 | 端口 | 特点 |
|------|------|------|------|
| HTTP/HTTPS | 轮询 | 18443 | 定时 pull 任务（10s+抖动），兼容性好 |
| WebSocket | 长连接推送 | 18443 | 服务端主动推送任务，秒级响应 |
| TCP | 长连接推送 | 28443 | 独立端口，`[4字节长度][密文]` 帧格式 |

### 加密通信（6 种算法）

| 算法 | 密钥派生 | 特点 |
|------|---------|------|
| AES-128-CBC | SHA256(password)[:16] | IV 随机，PKCS7 填充 |
| AES-256-CBC | SHA256(password)[:32] | IV 随机，PKCS7 填充 |
| ChaCha20 | SHA256(password)[:32] | IETF 标准，12 字节 nonce |
| RC4 | SHA256(password)[:32] | 流加密 |
| XOR | password 直接循环 | 轻量级 |
| none | - | base64 编码，不加密 |

所有加密后数据经 base64 编码传输，body 为纯密文（无 JSON 外壳、无字段名、无自定义 HTTP 头），服务端自动遍历算法解密。

### Payload 生成

| 格式 | 平台 | 生成方式 |
|------|------|---------|
| EXE | Windows/Linux | Go 交叉编译 + 源码混淆（每次 MD5 不同） |
| DLL | Windows | C 源码（DLL 劫持） |
| BAT/PS1 | Windows | 脚本 |
| SH | Linux | Bash 脚本 |
| Python | 跨平台 | 混淆脚本 |
| PHP/JSP/ASPX/ASP | Web | 冰蝎被动模式 WebShell |
| Shellcode | Windows/Linux | C/Python/Hex/Raw/EXE Loader，字节级混淆 |

### 客户端功能

- **命令执行**：支持 cmd/powershell/bash，跨平台
- **文件管理**：列目录/查看/编辑/上传/下载/删除/重命名/建目录
- **屏幕监控**：截图/录屏
- **媒体采集**：录音/摄像头拍照/摄像头录像
- **系统信息**：进程列表/系统信息
- **内网穿透**：端口转发/SOCKS5 代理/HTTP 代理
- **WebShell 管理**：冰蝎被动模式，支持 PHP/JSP/ASPX/ASP

### 流量伪装

- URL 路径随机化：`/api/v1/<16位随机hex>`，每次请求不同
- 操作类型 `_op` 隐入加密 body，无明文特征
- 多浏览器 UA 池：Chrome/Edge/Firefox/Safari 随机切换
- 随机 Accept-Language（8 语言池）
- 心跳节奏 45s/10s + 0-50% 随机抖动
- 纯密文 body，Content-Type 伪装为 `application/json`
- Shellcode 字节级混淆（NOP 滑动 + 垃圾指令），每次生成 MD5 不同

---

## 架构设计

### 端口隔离体系

```
┌─────────────────────────────────────────────────┐
│                 C2 Server (Go)                  │
├─────────────────────────────────────────────────┤
│  Web 控制台 :5000    │  Agent 回连 :18443       │
│  - 管理 API          │  - HTTP POST (纯密文)    │
│  - WebSocket /ws     │  - WebSocket /ws/agent/  │
│  - 静态资源          │  - /deliver/{token}      │
├─────────────────────┼──────────────────────────┤
│  Agent TCP :28443   │  Shell TCP :44330        │
│  - [4B长度][密文]    │  - reverse_tcp 回连      │
└─────────────────────┴──────────────────────────┘
```

- Web 控制台（5000）不暴露 `/agent/*` 和 `/deliver/*` 路由
- Agent 回连（18443）不暴露管理界面和 Web 界面
- 三种端口完全隔离，路由不交叉

### 目录结构

```
c2-rich/
├── server-go/               # 服务端源码
│   ├── main.go              # 主入口，路由注册
│   ├── agent.go             # Agent 通信（heartbeat/pull/result）
│   ├── agent_ws.go          # WebSocket Agent 端点
│   ├── agent_tcp.go         # TCP Agent 监听器
│   ├── agent_conn_registry.go  # 长连接注册表（推送）
│   ├── handlers.go          # Web API handlers
│   ├── auth.go              # 认证（JWT/Session）
│   ├── crypto.go            # 加密层
│   ├── payload_gen.go       # Payload 生成
│   ├── shellcode_obf.go     # Shellcode 混淆
│   ├── shell_handler.go     # Raw TCP Shell Handler
│   ├── webshell.go          # WebShell 管理
│   ├── obfuscator.go        # 代码混淆
│   ├── obfuscator_go.go     # Go Agent 源码混淆
│   ├── settings.go          # 配置管理
│   ├── db.go                # 数据库
│   ├── websocket.go         # Web 控制台实时推送
│   ├── static/              # 前端资源
│   │   ├── index.html
│   │   ├── css/style.css
│   │   └── js/              # 前端模块（16 个 JS 文件）
│   └── c2.db                # SQLite 数据库
├── client-go/               # 客户端 Agent 源码
│   ├── main.go              # 主入口（协议模式分发）
│   ├── config.go            # 配置（ldflags 注入）
│   ├── crypto.go            # 加密层（与服务端对齐）
│   ├── transport.go         # HTTP 传输
│   ├── transport_ws.go      # WebSocket 传输（reader 分发）
│   ├── transport_tcp.go     # TCP 传输（reader 分发）
│   ├── tasks.go             # 任务调度
│   ├── task_cmd.go          # 命令执行
│   ├── task_file.go         # 文件管理
│   ├── task_media.go        # 媒体采集
│   ├── task_system.go       # 系统信息
│   └── task_tunnel.go       # 内网穿透
├── config.json              # 配置文件
├── c2.bat                   # Windows 启动脚本
└── c2.sh                    # Linux 启动脚本
```

---

## 快速开始

### 环境要求

- Go 1.21+
- 操作系统：Windows / Linux

### Windows 启动

```cmd
c2.bat
```

脚本自动编译服务端，启动后显示：
```
Web 控制台:   http://127.0.0.1:5000
Agent 回连:   http://127.0.0.1:18443
Agent TCP:    tcp://127.0.0.1:28443
Shell TCP:    tcp://127.0.0.1:44330
默认账号:     admin / admin123
```

### Linux 启动

```bash
chmod +x c2.sh
./c2.sh start      # 启动（后台运行）
./c2.sh status     # 查看状态
./c2.sh log        # 实时日志
./c2.sh restart    # 重启
./c2.sh stop       # 停止
./c2.sh build_all  # 编译全部（服务端 + Agent）
```

### 生成 Payload

1. 浏览器访问 `http://127.0.0.1:5000`，使用 `admin/admin123` 登录
2. 进入「Payload 生成」页面
3. 选择通信协议（HTTP/WebSocket/TCP）、加密算法、输出格式
4. 点击「生成 Payload」
5. 下载生成的 EXE/脚本/WebShell，在目标机器执行

### 添加 WebShell

1. 在「Payload 生成」页面生成 PHP/JSP/ASPX WebShell 文件
2. 将文件上传到目标 Web 服务器
3. 进入「WebShell 管理」页面，点击「添加 WebShell」
4. 填写 WebShell URL、加密算法、密码（必须与生成时一致）
5. 验证成功后即可在终端/文件管理中操作

---

## 配置说明

### config.json（文件配置）

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

### 数据库配置（通过 Web 界面修改）

进入「配置管理」页面可调整：

- **通信监听**：回连地址、Agent 端口、TCP 端口、Shell 端口
- **通信加密**：加密算法、加密密码
- **客户端行为**：心跳间隔、任务拉取间隔、离线超时
- **任务限制**：截图分辨率、录屏时长、上传大小限制
- **安全策略**：Session 超时、登录尝试次数、锁定时间
- **Webhook**：事件通知 URL、触发事件

---

## 技术栈

| 组件 | 技术 |
|------|------|
| 后端 | Go 1.21+，纯 Go（CGO_ENABLED=0） |
| 前端 | 原生 JavaScript + jQuery 3.7.1 |
| 数据库 | SQLite（modernc.org/sqlite 纯 Go 驱动） |
| WebSocket | gorilla/websocket |
| 加密 | crypto/aes, crypto/rc4, golang.org/x/crypto/chacha20 |
| 交叉编译 | GOOS/GOARCH 环境变量 |
| 配置注入 | go build -ldflags -X |

---

## 安全声明

本项目仅供以下用途：
- 安全研究学习
- 授权的渗透测试
- 防御技术研究

使用本工具进行未授权测试是违法行为，使用者需自行承担法律责任。
