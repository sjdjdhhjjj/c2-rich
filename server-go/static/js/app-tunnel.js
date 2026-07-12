// app-tunnel.js - 内网穿透管理（端口转发 / SOCKS5 / HTTP 代理）

app.renderTunnel = function() {
    if (!this.selectedClient) {
        return this.renderClientBar();
    }
    const c = this.selectedClient;
    // 端口默认值优先从配置管理读取（缓存到 this._tunnelPorts 避免每次渲染都请求）
    const tp = this._tunnelPorts || {};
    const pfPort = tp.tunnel_port_forward || '8888';
    const s5Port = tp.tunnel_socks5_port || '1080';
    const hpPort = tp.tunnel_http_proxy_port || '8080';
    // 异步加载配置端口（首次进入页面时）
    if (!this._tunnelPorts) {
        this._loadTunnelPorts();
    }
    return `
        ${this.renderClientBar()}
        <div class="card" style="padding:20px; margin-bottom:16px;">
            <h3 style="margin-bottom:16px; font-size:15px;">
                <i class="fas fa-network-wired" style="color:#bc8cff;"></i>
                内网穿透 - ${c.hostname} (${c.ip || 'unknown'})
            </h3>
            <div style="font-size:12px; color:#8b949e; margin-bottom:16px;">
                在目标主机上启动代理服务，将目标内网流量代理出来。参考 MSF route / CS socks proxy。<br>
                <span style="color:#d29922;">端口默认值来自配置管理（与 Web 后台/客户端监听端口分离，避免冲突）</span>
            </div>

            <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:16px;">
                <!-- 端口转发 -->
                <div style="border:1px solid #21262d; border-radius:8px; padding:16px; background:#0d1117;">
                    <h4 style="font-size:14px; color:#58a6ff; margin-bottom:12px;">
                        <i class="fas fa-arrow-right-arrow-left"></i> 端口转发
                    </h4>
                    <div style="font-size:11px; color:#8b949e; margin-bottom:10px;">
                        在目标机监听端口，转发到指定目标
                    </div>
                    <div style="margin-bottom:8px;">
                        <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">本地监听端口</label>
                        <input type="number" class="input" id="pfLocalPort" value="${pfPort}" style="font-size:12px; width:100%;">
                    </div>
                    <div style="margin-bottom:8px;">
                        <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">目标主机</label>
                        <input type="text" class="input" id="pfTargetHost" value="127.0.0.1" style="font-size:12px; width:100%;">
                    </div>
                    <div style="margin-bottom:10px;">
                        <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">目标端口</label>
                        <input type="number" class="input" id="pfTargetPort" value="3389" style="font-size:12px; width:100%;">
                    </div>
                    <button class="btn btn-primary" style="width:100%; justify-content:center; font-size:12px;" onclick="app.tunnelStart('port_forward')">
                        <i class="fas fa-play"></i> 启动转发
                    </button>
                </div>

                <!-- SOCKS5 代理 -->
                <div style="border:1px solid #238636; border-radius:8px; padding:16px; background:#0d1117;">
                    <h4 style="font-size:14px; color:#3fb950; margin-bottom:12px;">
                        <i class="fas fa-socks"></i> SOCKS5 代理
                    </h4>
                    <div style="font-size:11px; color:#8b949e; margin-bottom:10px;">
                        在目标机启动 SOCKS5 服务器
                    </div>
                    <div style="margin-bottom:8px;">
                        <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">监听端口</label>
                        <input type="number" class="input" id="s5Port" value="${s5Port}" style="font-size:12px; width:100%;">
                    </div>
                    <div style="margin-bottom:8px;">
                        <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">用户名 (可选)</label>
                        <input type="text" class="input" id="s5User" placeholder="留空则无认证" style="font-size:12px; width:100%;">
                    </div>
                    <div style="margin-bottom:10px;">
                        <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">密码 (可选)</label>
                        <input type="text" class="input" id="s5Pass" placeholder="留空则无认证" style="font-size:12px; width:100%;">
                    </div>
                    <button class="btn btn-primary" style="width:100%; justify-content:center; font-size:12px; background:#238636;" onclick="app.tunnelStart('socks5')">
                        <i class="fas fa-play"></i> 启动 SOCKS5
                    </button>
                </div>

                <!-- HTTP 代理 -->
                <div style="border:1px solid #d29922; border-radius:8px; padding:16px; background:#0d1117;">
                    <h4 style="font-size:14px; color:#d29922; margin-bottom:12px;">
                        <i class="fas fa-globe"></i> HTTP 代理
                    </h4>
                    <div style="font-size:11px; color:#8b949e; margin-bottom:10px;">
                        在目标机启动 HTTP/HTTPS 代理
                    </div>
                    <div style="margin-bottom:8px;">
                        <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">监听端口</label>
                        <input type="number" class="input" id="hpPort" value="${hpPort}" style="font-size:12px; width:100%;">
                    </div>
                    <div style="margin-bottom:10px; font-size:11px; color:#6e7681;">
                        支持 HTTP 和 HTTPS (CONNECT 方法)
                    </div>
                    <button class="btn btn-primary" style="width:100%; justify-content:center; font-size:12px; background:#d29922;" onclick="app.tunnelStart('http')">
                        <i class="fas fa-play"></i> 启动 HTTP 代理
                    </button>
                </div>
            </div>
        </div>

        <!-- 隧道列表 -->
        <div class="card" style="padding:20px;">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px;">
                <h3 style="font-size:15px;">
                    <i class="fas fa-list" style="color:#58a6ff;"></i> 活跃隧道
                </h3>
                <div style="display:flex; gap:8px;">
                    <button class="btn btn-secondary" style="font-size:12px;" onclick="app.tunnelList()">
                        <i class="fas fa-sync"></i> 刷新列表
                    </button>
                    <button class="btn btn-danger" style="font-size:12px;" onclick="app.tunnelStopAll()">
                        <i class="fas fa-stop"></i> 停止全部
                    </button>
                </div>
            </div>
            <div id="tunnelListContainer" style="min-height:100px;">
                <div style="text-align:center; padding:40px; color:#8b949e;">
                    <i class="fas fa-info-circle" style="font-size:24px; margin-bottom:8px;"></i>
                    <div style="font-size:12px;">点击"刷新列表"查看活跃隧道</div>
                </div>
            </div>
        </div>

        <!-- 使用说明 -->
        <div class="card" style="padding:20px; margin-top:16px; border:1px solid #21262d;">
            <h3 style="font-size:14px; margin-bottom:12px; color:#8b949e;">
                <i class="fas fa-book"></i> 使用说明
            </h3>
            <div style="font-size:12px; color:#8b949e; line-height:1.8;">
                <div><b style="color:#58a6ff;">端口转发：</b>在目标机监听 local_port，将流量转发到 target_host:target_port</div>
                <div style="margin-left:16px;">示例：监听 8888 转发到 10.0.0.1:3389，访问 目标IP:8888 即可连接内网 RDP</div>
                <div style="margin-top:8px;"><b style="color:#3fb950;">SOCKS5 代理：</b>在目标机启动 SOCKS5 服务器</div>
                <div style="margin-left:16px;">示例：启动后配置攻击机 proxychains: <code>socks5 目标IP 1080</code></div>
                <div style="margin-left:16px;">使用 proxychains: <code>proxychains nmap -sT 10.0.0.0/24</code></div>
                <div style="margin-top:8px;"><b style="color:#d29922;">HTTP 代理：</b>在目标机启动 HTTP 代理</div>
                <div style="margin-left:16px;">示例：浏览器配置 HTTP 代理 -> 目标IP:8080，可访问内网 Web 服务</div>
            </div>
        </div>
    `;
};

// 从配置管理加载内网穿透端口默认值
app._loadTunnelPorts = async function() {
    try {
        const settings = await API.get('/api/settings');
        const listen = settings.listen || {};
        this._tunnelPorts = {
            tunnel_port_forward: listen.tunnel_port_forward || '8888',
            tunnel_socks5_port: listen.tunnel_socks5_port || '1080',
            tunnel_http_proxy_port: listen.tunnel_http_proxy_port || '8080',
        };
        // 填充到表单（如果当前在隧道页面）
        const pf = document.getElementById('pfLocalPort');
        if (pf) pf.value = this._tunnelPorts.tunnel_port_forward;
        const s5 = document.getElementById('s5Port');
        if (s5) s5.value = this._tunnelPorts.tunnel_socks5_port;
        const hp = document.getElementById('hpPort');
        if (hp) hp.value = this._tunnelPorts.tunnel_http_proxy_port;
    } catch (e) {
        // 静默失败
    }
};

// 启动隧道
app.tunnelStart = async function(type) {
    if (!this.selectedClient) { this._notify('请先选择主机', 'error'); return; }
    let taskType, taskData, desc;
    if (type === 'port_forward') {
        const lp = parseInt(document.getElementById('pfLocalPort').value);
        const th = document.getElementById('pfTargetHost').value;
        const tp = parseInt(document.getElementById('pfTargetPort').value);
        if (!lp || !th || !tp) { this._notify('参数不完整', 'error'); return; }
        taskType = 'port_forward';
        taskData = { action: 'start', local_port: lp, target_host: th, target_port: tp };
        desc = `端口转发 ${lp} -> ${th}:${tp}`;
    } else if (type === 'socks5') {
        const port = parseInt(document.getElementById('s5Port').value);
        taskType = 'socks5_proxy';
        taskData = {
            action: 'start',
            port: port,
            username: document.getElementById('s5User').value,
            password: document.getElementById('s5Pass').value
        };
        desc = `SOCKS5 代理 :${port}`;
    } else if (type === 'http') {
        const port = parseInt(document.getElementById('hpPort').value);
        taskType = 'http_proxy';
        taskData = { action: 'start', port: port };
        desc = `HTTP 代理 :${port}`;
    } else return;

    const taskIds = await this.sendTask(taskType, taskData);
    if (taskIds && taskIds.length > 0) {
        this._notify(`${desc} 任务已下发，等待结果...`, 'info');
        // 等待任务结果并展示
        this._waitForTunnelTask(taskIds[0]);
    }
};

// 等待隧道任务结果
app._waitForTunnelTask = function(taskId) {
    let attempts = 0;
    const check = () => {
        attempts++;
        API.get(`/api/task/${taskId}`).then(task => {
            if (!task) { if (attempts < 10) setTimeout(check, 2000); return; }
            if (task.status === 'completed' && task.result) {
                this._notify(task.result, 'success', 8000);
                // 自动刷新隧道列表
                setTimeout(() => this.tunnelList(), 1000);
            } else if (task.status === 'failed') {
                this._notify('隧道启动失败: ' + (task.result || ''), 'error', 8000);
            } else {
                if (attempts < 15) setTimeout(check, 2000);
                else this._notify('等待结果超时', 'error');
            }
        }).catch(() => { if (attempts < 10) setTimeout(check, 2000); });
    };
    setTimeout(check, 1500);
};

// 列出所有隧道
app.tunnelList = async function() {
    if (!this.selectedClient) { this._notify('请先选择主机', 'error'); return; }
    const container = document.getElementById('tunnelListContainer');
    if (container) {
        container.innerHTML = '<div style="text-align:center; padding:20px; color:#8b949e;"><i class="fas fa-spinner fa-spin"></i> 查询中...</div>';
    }
    // 并行查询三种隧道
    const types = [
        { taskType: 'port_forward', label: '端口转发', icon: 'fa-arrow-right-arrow-left', color: '#58a6ff' },
        { taskType: 'socks5_proxy', label: 'SOCKS5', icon: 'fa-socks', color: '#3fb950' },
        { taskType: 'http_proxy', label: 'HTTP代理', icon: 'fa-globe', color: '#d29922' }
    ];
    const results = [];
    for (const t of types) {
        const taskIds = await this.sendTask(t.taskType, { action: 'list' });
        if (taskIds && taskIds[0]) {
            try {
                const task = await API.get(`/api/task/${taskIds[0]}`);
                if (task.status === 'completed' && task.result) {
                    results.push({ ...t, text: task.result });
                }
            } catch(e) {}
        }
    }

    if (container) {
        if (results.every(r => !r.text || r.text.includes('无'))) {
            container.innerHTML = '<div style="text-align:center; padding:40px; color:#8b949e;"><i class="fas fa-info-circle" style="font-size:24px; margin-bottom:8px;"></i><div style="font-size:12px;">暂无活跃隧道</div></div>';
            return;
        }
        container.innerHTML = results.map(r => {
            if (!r.text || r.text.includes('无')) return '';
            return `
                <div style="border:1px solid #21262d; border-radius:8px; padding:12px; margin-bottom:10px; background:#161b22;">
                    <div style="display:flex; align-items:center; gap:8px; margin-bottom:8px;">
                        <i class="fas ${r.icon}" style="color:${r.color}; font-size:14px;"></i>
                        <b style="color:${r.color}; font-size:13px;">${r.label}</b>
                    </div>
                    <pre style="font-family:monospace; font-size:11px; color:#7ee787; white-space:pre-wrap; margin:0;">${r.text.replace(/</g,'&lt;')}</pre>
                </div>
            `;
        }).join('');
    }
};

// 停止单个隧道
app.tunnelStop = async function(tunnelId, taskType) {
    if (!this.selectedClient) return;
    if (!confirm(`确认停止隧道 ${tunnelId} ？`)) return;
    const taskIds = await this.sendTask(taskType, { action: 'stop', tunnel_id: tunnelId });
    if (taskIds && taskIds[0]) {
        this._notify(`停止任务已下发: ${tunnelId}`, 'info');
        setTimeout(() => this.tunnelList(), 2000);
    }
};

// 停止所有隧道
app.tunnelStopAll = async function() {
    if (!this.selectedClient) return;
    if (!confirm('确认停止所有隧道？')) return;
    const types = ['port_forward', 'socks5_proxy', 'http_proxy'];
    for (const t of types) {
        await this.sendTask(t, { action: 'stop', tunnel_id: '' });
    }
    this._notify('停止全部任务已下发', 'info');
    setTimeout(() => this.tunnelList(), 2000);
};
