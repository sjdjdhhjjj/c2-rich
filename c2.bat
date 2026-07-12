@echo off
setlocal enabledelayedexpansion
title C2 Server (Go Edition)
color 0A

echo ==================================================
echo    C2 Control Center - Go Edition
echo    Windows Startup Script
echo ==================================================
echo.

cd /d "%~dp0"

REM Check Go environment
where go >nul 2>nul
if errorlevel 1 (
    echo [!] Go not found. Please install Go 1.21+ and add to PATH
    echo [!] Download: https://go.dev/dl/
    pause
    exit /b 1
)

echo [*] Go version:
go version
echo.

REM Build server (always rebuild to ensure latest)
echo [*] Building C2 server...
pushd server-go
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -o c2_server.exe .
if errorlevel 1 (
    echo [!] Build failed, please check code
    popd
    pause
    exit /b 1
)
popd
echo [+] Server built successfully: server-go\c2_server.exe
echo.

REM Check config.json
if not exist "config.json" (
    echo [!] config.json not found, creating default...
    (echo {) > config.json
    (echo   "web": {) >> config.json
    (echo     "host": "0.0.0.0",) >> config.json
    (echo     "port": 5000,) >> config.json
    (echo     "protocol": "http",) >> config.json
    (echo     "ssl_cert": "",) >> config.json
    (echo     "ssl_key": "") >> config.json
    (echo   }) >> config.json
    (echo }) >> config.json
    echo [+] Default config.json created
    echo.
)

REM Kill old server process
echo [*] Checking for existing server process...
taskkill /F /IM c2_server.exe >nul 2>nul
timeout /t 1 /nobreak >nul

REM Read port from config.json for display
for /f "delims=" %%i in ('powershell -NoProfile -Command "(Get-Content config.json | ConvertFrom-Json).web.port"') do set CFG_PORT=%%i
for /f "delims=" %%i in ('powershell -NoProfile -Command "(Get-Content config.json | ConvertFrom-Json).web.host"') do set CFG_HOST=%%i
if "!CFG_PORT!"=="" set CFG_PORT=5000
if "!CFG_HOST!"=="" set CFG_HOST=0.0.0.0

REM Start service
echo [*] Starting C2 server (Go)...
echo [*] Web UI: http://127.0.0.1:!CFG_PORT!
if "!CFG_HOST!"=="0.0.0.0" (
    echo [*] Listen: 0.0.0.0:!CFG_PORT! (all interfaces)
) else (
    echo [*] Listen: !CFG_HOST!:!CFG_PORT!
)
echo [*] Config: config.json (root directory)
echo [*] User: admin / admin123
echo [*] WebSocket: ws://127.0.0.1:!CFG_PORT!/ws
echo [*] Press Ctrl+C to stop
echo.

server-go\c2_server.exe

echo.
echo [*] Service stopped
pause