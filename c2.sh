#!/bin/bash
# ============================================================
# C2 演示系统 - Go 版 Linux 服务管理脚本
# 用法: ./c2.sh {start|stop|restart|status|log|build|agent}
# ============================================================

# 项目根目录（脚本所在目录）
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVER_GO_DIR="$PROJECT_DIR/server-go"
CLIENT_GO_DIR="$PROJECT_DIR/client-go"
PID_FILE="$PROJECT_DIR/c2.pid"
LOG_FILE="$PROJECT_DIR/c2.log"
SERVER_BIN="$SERVER_GO_DIR/c2_server"

# ============ 检查 Go 环境 ============
check_go() {
    if ! command -v go &>/dev/null; then
        echo "[-] 未找到 Go，请安装 Go 1.21+"
        echo "    下载: https://go.dev/dl/"
        echo "    Ubuntu/Debian: sudo apt install golang-go"
        echo "    或从官方下载: https://go.dev/dl/"
        return 1
    fi
    return 0
}

# ============ 编译服务端 ============
build_server() {
    echo "[*] 编译 C2 服务端 (Go, Linux ELF)..."
    cd "$SERVER_GO_DIR"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o c2_server .
    local ret=$?
    cd "$PROJECT_DIR"
    if [ $ret -ne 0 ]; then
        echo "[-] 服务端编译失败"
        return 1
    fi
    chmod +x "$SERVER_BIN"
    echo "[+] 服务端编译成功: $SERVER_BIN"
    return 0
}

# ============ 编译 Agent ============
build_agent() {
    echo "[*] 编译 Go Agent..."
    cd "$CLIENT_GO_DIR"
    # Windows EXE
    echo "    - Windows EXE (amd64)..."
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o c2_agent.exe .
    # Linux ELF
    echo "    - Linux ELF (amd64)..."
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o c2_agent_linux_amd64 .
    local ret=$?
    cd "$PROJECT_DIR"
    if [ $ret -ne 0 ]; then
        echo "[-] Agent 编译失败"
        return 1
    fi
    echo "[+] Agent 编译成功:"
    echo "    $CLIENT_GO_DIR/c2_agent.exe (Windows PE)"
    echo "    $CLIENT_GO_DIR/c2_agent_linux_amd64 (Linux ELF)"
    return 0
}

# ============ 编译全部 ============
build_all() {
    if ! check_go; then
        return 1
    fi
    build_server || return 1
    build_agent || return 1
    echo "[+] 全部编译完成"
    return 0
}

# ============ 启动服务 ============
start() {
    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
        echo "[!] C2 服务已在运行 (PID: $(cat "$PID_FILE"))"
        exit 1
    fi

    if ! check_go; then
        exit 1
    fi

    # 自动编译（如果二进制不存在）
    if [ ! -f "$SERVER_BIN" ]; then
        if ! build_server; then
            exit 1
        fi
    else
        echo "[*] 服务端已编译: $SERVER_BIN"
    fi

    # 检查 config.json
    if [ ! -f "$PROJECT_DIR/config.json" ]; then
        echo "[!] config.json 不存在，创建默认配置..."
        cat > "$PROJECT_DIR/config.json" <<'EOF'
{
  "web": {
    "host": "0.0.0.0",
    "port": 5000,
    "protocol": "http",
    "ssl_cert": "",
    "ssl_key": ""
  }
}
EOF
        echo "[+] 默认 config.json 已创建"
    fi

    # 从 config.json 读取端口（用于显示）
    CFG_PORT=$(grep -o '"port"[[:space:]]*:[[:space:]]*[0-9]*' "$PROJECT_DIR/config.json" | grep -o '[0-9]*' | head -1)
    CFG_HOST=$(grep -o '"host"[[:space:]]*:[[:space:]]*"[^"]*"' "$PROJECT_DIR/config.json" | grep -o '"[^"]*"$' | tr -d '"' | head -1)
    [ -z "$CFG_PORT" ] && CFG_PORT=5000
    [ -z "$CFG_HOST" ] && CFG_HOST=0.0.0.0

    echo "[*] 启动 C2 演示系统 (Go)..."
    echo "    Web UI: http://127.0.0.1:$CFG_PORT"
    echo "    Listen: $CFG_HOST:$CFG_PORT"
    echo "    Config: config.json (root directory)"
    echo "    User: admin / admin123"
    cd "$PROJECT_DIR"
    nohup "$SERVER_BIN" > "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
    sleep 2

    if kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
        echo "[+] C2 服务已启动 (PID: $(cat "$PID_FILE"))"
        echo "    日志: $LOG_FILE"
        # 显示启动信息
        head -15 "$LOG_FILE" 2>/dev/null | while read line; do
            echo "    $line"
        done
    else
        echo "[-] 启动失败，请查看日志: $LOG_FILE"
        tail -20 "$LOG_FILE"
        rm -f "$PID_FILE"
        exit 1
    fi
}

# ============ 停止服务 ============
stop() {
    if [ ! -f "$PID_FILE" ]; then
        echo "[!] C2 服务未运行"
        exit 1
    fi
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        echo "[*] 停止 C2 服务 (PID: $PID)..."
        kill "$PID"
        sleep 2
        if kill -0 "$PID" 2>/dev/null; then
            echo "[!] 进程未退出，强制终止..."
            kill -9 "$PID"
        fi
        echo "[+] C2 服务已停止"
    else
        echo "[!] 进程不存在 (PID: $PID)，清理 PID 文件"
    fi
    rm -f "$PID_FILE"
}

# ============ 重启服务 ============
restart() {
    stop 2>/dev/null
    sleep 1
    start
}

# ============ 查看状态 ============
status() {
    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
        PID=$(cat "$PID_FILE")
        echo "[+] C2 服务运行中 (PID: $PID)"
        echo "    二进制: $SERVER_BIN"
        echo "    日志: $LOG_FILE"
        # 显示端口信息
        if command -v ss &>/dev/null; then
            ss -tlnp 2>/dev/null | grep "$PID" | head -5
        elif command -v netstat &>/dev/null; then
            netstat -tlnp 2>/dev/null | grep "$PID" | head -5
        fi
    else
        echo "[-] C2 服务未运行"
        if [ -f "$SERVER_BIN" ]; then
            echo "[*] 二进制存在: $SERVER_BIN"
            echo "    执行 ./c2.sh start 启动"
        else
            echo "[*] 二进制未编译"
            echo "    执行 ./c2.sh build 编译"
        fi
        exit 1
    fi
}

# ============ 查看日志 ============
log() {
    if [ -f "$LOG_FILE" ]; then
        echo "[*] 实时日志 (Ctrl+C 退出):"
        tail -f "$LOG_FILE"
    else
        echo "[-] 日志文件不存在: $LOG_FILE"
        exit 1
    fi
}

# ============ 主入口 ============
case "${1:-}" in
    start)   start ;;
    stop)    stop ;;
    restart) restart ;;
    status)  status ;;
    log)     log ;;
    build)   check_go && build_server ;;
    build_agent) check_go && build_agent ;;
    build_all)  build_all ;;
    *)
        echo "C2 演示系统管理脚本 (Go 版)"
        echo "用法: $0 {start|stop|restart|status|log|build|build_agent|build_all}"
        echo ""
        echo "  start       启动服务（自动编译 if 未编译）"
        echo "  stop        停止服务"
        echo "  restart     重启服务"
        echo "  status      查看运行状态"
        echo "  log         查看实时日志"
        echo "  build       编译服务端 (Linux ELF)"
        echo "  build_agent 编译 Go Agent (Windows EXE + Linux ELF)"
        echo "  build_all   编译全部（服务端 + Agent）"
        exit 1
        ;;
esac
