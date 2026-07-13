// app-settings.js - 配置管理（参考 CS：监听器/加密/账户/系统参数/Webhook）
// 每个分类独立保存，避免误操作；监听配置修改后提示需重启服务

app._settingsTab = 'listen';   // 当前激活的子Tab
app._settingsData = null;      // 从服务端加载的完整配置
app._usersList = [];           // 用户列表

app.renderSettings = function() {
    if (!this._settingsData) {
        // 首次进入自动加载
        this.loadSettings();
        return `<div class="card" style="padding:30px; text-align:center; color:#8b949e;">
            <i class="fas fa-spinner fa-spin" style="font-size:24px; margin-bottom:10px;"></i>
            <div>正在加载配置...</div>
        </div>`;
    }
    const tabs = [
        { id: 'listen',    icon: 'fa-satellite-dish', label: '监听配置' },
        { id: 'crypto',    icon: 'fa-key',            label: '通信加密' },
        { id: 'account',   icon: 'fa-user-shield',    label: '账户管理' },
        { id: 'client',    icon: 'fa-sliders',        label: '客户端参数' },
        { id: 'limits',    icon: 'fa-gauge',          label: '任务限制' },
        { id: 'security',  icon: 'fa-lock',           label: '安全策略' },
        { id: 'webhook',   icon: 'fa-bell',           label: 'Webhook通知' },
    ];
    return `
        <div class="card" style="padding:16px; margin-bottom:16px;">
            <div style="display:flex; align-items:center; gap:8px; margin-bottom:14px; flex-wrap:wrap;">
                <div class="tabs" style="margin-bottom:0; border-bottom:none; flex:1;">
                    ${tabs.map(t => `
                        <div class="tab ${this._settingsTab === t.id ? 'active' : ''}" onclick="app._switchSettingsTab('${t.id}')">
                            <i class="fas ${t.icon}"></i> ${t.label}
                        </div>
                    `).join('')}
                </div>
            </div>
            <div id="settingsPanel">${this._renderSettingsPanel()}</div>
        </div>
    `;
};

app._renderSettingsPanel = function() {
    switch(this._settingsTab) {
        case 'listen':   return this._renderListenPanel();
        case 'crypto':   return this._renderCryptoPanel();
        case 'account':  return this._renderAccountPanel();
        case 'client':   return this._renderClientPanel();
        case 'limits':   return this._renderLimitsPanel();
        case 'security': return this._renderSecurityPanel();
        case 'webhook':  return this._renderWebhookPanel();
        default:         return '';
    }
};

app._switchSettingsTab = function(tab) {
    this._settingsTab = tab;
    if (tab === 'account') this.loadUsers();
    const panel = document.getElementById('settingsPanel');
    if (panel) panel.innerHTML = this._renderSettingsPanel();
    // 更新Tab高亮
    document.querySelectorAll('.card .tab').forEach(el => el.classList.remove('active'));
    event && event.currentTarget && event.currentTarget.classList.add('active');
};

// ============== 监听配置 ==============
// 三类独立服务: HTTP/HTTPS (Web控制台+Agent回连) / TCP (Shell回连) / 内网穿透
// 注意: web host/port/protocol/ssl_cert/ssl_key 来自根目录 config.json 文件，此处只读
// 其余字段（callback_host / client_listen_port / agent_protocol / shell_listen_port / tunnel_*）存在数据库，可在线编辑
app._renderListenPanel = function() {
    const l = this._settingsData.listen || {};
    return `
        <div style="max-width:680px;">
            <div style="background:#0d1117; border:1px solid #238636; border-radius:8px; padding:12px; margin-bottom:16px;">
                <div style="color:#3fb950; font-weight:600; margin-bottom:6px;"><i class="fas fa-info-circle"></i> 监听器配置 (三类独立服务)</div>
                <div style="color:#8b949e; font-size:12px; line-height:1.6;">
                    服务端运行三类独立监听服务，端口/协议完全分离，职责隔离:<br>
                    <span style="color:#58a6ff;">HTTP/HTTPS</span> Web 控制台 + Agent 回连 ｜
                    <span style="color:#f0883e;">TCP</span> Shellcode 回连 ｜
                    <span style="color:#bc8cff;">内网穿透</span> 端口转发/SOCKS5/HTTP代理
                </div>
            </div>

            <!-- ========== Payload 回连地址（共用） ========== -->
            <div style="background:#161b22; border:1px solid #30363d; border-radius:8px; padding:12px; margin-bottom:16px;">
                <div style="color:#8b949e; font-weight:600; margin-bottom:8px;"><i class="fas fa-globe"></i> Payload 回连地址</div>
                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">回连 IP（共用）</label>
                <input type="text" class="input" id="setCallbackHost" value="${l.callback_host || ''}" placeholder="留空=自动检测 (${l.detected_local_ip || '127.0.0.1'})">
                <div style="font-size:11px; color:#6e7681; margin-top:4px;">受害者需能访问到此 IP，留空自动用本机IP: <span style="color:#3fb950;">${l.detected_local_ip || '127.0.0.1'}</span></div>
            </div>

            <!-- ========== HTTP/HTTPS 服务 ========== -->
            <div style="background:#0d1117; border:1px solid #58a6ff; border-radius:8px; padding:12px; margin-bottom:16px;">
                <div style="color:#58a6ff; font-weight:600; margin-bottom:6px;"><i class="fas fa-globe"></i> HTTP/HTTPS 服务</div>
                <div style="color:#8b949e; font-size:12px; line-height:1.6; margin-bottom:10px;">
                    Web 控制台（管理后台）和 Agent 回连走 HTTP/HTTPS 协议，独立端口，路由完全隔离。<br>
                    Agent 端口不暴露 <code>/api/*</code> 管理接口，Web 端口不暴露 <code>/agent/*</code> 回连端点。
                </div>

                <!-- Web 控制台 (config.json 只读) -->
                <div style="background:#161b22; border:1px solid #30363d; border-radius:6px; padding:10px; margin-bottom:10px;">
                    <div style="color:#bc8cff; font-size:12px; font-weight:600; margin-bottom:6px;">
                        <i class="fas fa-window-maximize"></i> Web 控制台 (管理后台, config.json 只读)
                    </div>
                    <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:8px;">
                        <div>
                            <label style="font-size:11px; color:#6e7681; display:block; margin-bottom:4px;">监听 IP</label>
                            <input type="text" class="input" value="${l.host || '0.0.0.0'}" readonly style="opacity:0.6; cursor:not-allowed; font-size:12px;">
                        </div>
                        <div>
                            <label style="font-size:11px; color:#6e7681; display:block; margin-bottom:4px;">端口</label>
                            <input type="text" class="input" value="${l.port || '5000'}" readonly style="opacity:0.6; cursor:not-allowed; font-size:12px;">
                        </div>
                        <div>
                            <label style="font-size:11px; color:#6e7681; display:block; margin-bottom:4px;">协议</label>
                            <input type="text" class="input" value="${(l.protocol || 'http').toUpperCase()}" readonly style="opacity:0.6; cursor:not-allowed; font-size:12px;">
                        </div>
                    </div>
                    <div style="margin-top:6px;">
                        <label style="font-size:11px; color:#6e7681; display:block; margin-bottom:4px;">SSL 证书路径</label>
                        <input type="text" class="input" value="${l.ssl_cert || '(未配置)'}" readonly style="opacity:0.6; cursor:not-allowed; font-size:12px;">
                    </div>
                    <div style="margin-top:8px;">
                        <button class="btn btn-secondary" onclick="app.reloadFileConfig()" style="font-size:12px;">
                            <i class="fas fa-sync-alt"></i> 重新加载 config.json
                        </button>
                    </div>
                </div>

                <!-- Agent 回连 (可编辑) -->
                <div style="background:#161b22; border:1px solid #238636; border-radius:6px; padding:10px;">
                    <div style="color:#3fb950; font-size:12px; font-weight:600; margin-bottom:6px;">
                        <i class="fas fa-network-wired"></i> Agent 回连 (agent/deliver, 可在线编辑)
                    </div>
                    <div style="display:grid; grid-template-columns:1fr 1fr; gap:8px;">
                        <div>
                            <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">端口</label>
                            <input type="number" class="input" id="setClientListenPort" value="${l.client_listen_port || '8443'}" min="1" max="65535" style="font-size:12px;">
                        </div>
                        <div>
                            <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">协议</label>
                            <select class="input" id="setAgentProtocol" style="font-size:12px;">
                                <option value="http" ${(l.agent_protocol || 'http') === 'http' ? 'selected' : ''}>HTTP</option>
                                <option value="https" ${l.agent_protocol === 'https' ? 'selected' : ''}>HTTPS (共用 web SSL 证书)</option>
                            </select>
                        </div>
                    </div>
                    <div style="font-size:11px; color:#6e7681; margin-top:4px;">EXE/BAT/PS1 等 payload 回连此端口，/agent/* /deliver/* 端点，/ws/agent/* WebSocket 回连共用此端口</div>
                </div>
            </div>

            <!-- ========== TCP Agent 服务 (TCP 协议 Agent 回连) ========== -->
            <div style="background:#0d1117; border:1px solid #238636; border-radius:8px; padding:12px; margin-bottom:16px;">
                <div style="color:#3fb950; font-weight:600; margin-bottom:6px;"><i class="fas fa-plug"></i> TCP Agent 服务</div>
                <div style="color:#8b949e; font-size:12px; line-height:1.6; margin-bottom:10px;">
                    TCP 协议 Agent 回连端口，独立于 HTTP/WS。<br>
                    帧格式: <code>[4字节大端长度][base64密文]</code>，长连接，客户端轮询 pull。
                </div>
                <div>
                    <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">TCP Agent 端口</label>
                    <input type="number" class="input" id="setAgentTcpPort" value="${l.agent_tcp_port || '28443'}" min="1" max="65535" style="font-size:12px;">
                    <div style="font-size:11px; color:#6e7681; margin-top:4px;">TCP 协议 Agent 回连端口，独立于 HTTP/WS，默认 28443</div>
                </div>
            </div>

            <!-- ========== TCP 服务 (Shellcode 回连) ========== -->
            <div style="background:#0d1117; border:1px solid #f0883e; border-radius:8px; padding:12px; margin-bottom:16px;">
                <div style="color:#f0883e; font-weight:600; margin-bottom:6px;"><i class="fas fa-terminal"></i> TCP 服务 (Shellcode 回连)</div>
                <div style="color:#8b949e; font-size:12px; line-height:1.6; margin-bottom:10px;">
                    Raw TCP 监听器，接受 reverse_tcp/bind_tcp shellcode 回连。<br>
                    非 HTTP 协议，独立于 Web/Agent 的 HTTP 服务。参考 MSF <code>multi/handler</code>。
                </div>
                <div>
                    <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">Shell TCP 端口</label>
                    <input type="number" class="input" id="setShellListenPort" value="${l.shell_listen_port || '4444'}" min="1" max="65535" style="font-size:12px;">
                    <div style="font-size:11px; color:#6e7681; margin-top:4px;">shellcode 的 LPORT 填此端口，上线后在主机管理可见 (type=shell)</div>
                </div>
            </div>

            <!-- ========== 内网穿透 ========== -->
            <div style="background:#0d1117; border:1px solid #bc8cff; border-radius:8px; padding:12px; margin-bottom:16px;">
                <div style="color:#bc8cff; font-weight:600; margin-bottom:6px;"><i class="fas fa-route"></i> 内网穿透</div>
                <div style="color:#8b949e; font-size:12px; line-height:1.6; margin-bottom:10px;">
                    在目标机上监听本地端口，转发流量到指定目标。
                </div>
                <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:8px;">
                    <div>
                        <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">端口转发</label>
                        <input type="number" class="input" id="setTunnelPortForward" value="${l.tunnel_port_forward || '8888'}" min="1" max="65535" style="font-size:12px;">
                    </div>
                    <div>
                        <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">SOCKS5 代理</label>
                        <input type="number" class="input" id="setTunnelSocks5" value="${l.tunnel_socks5_port || '1080'}" min="1" max="65535" style="font-size:12px;">
                    </div>
                    <div>
                        <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">HTTP 代理</label>
                        <input type="number" class="input" id="setTunnelHttpProxy" value="${l.tunnel_http_proxy_port || '8080'}" min="1" max="65535" style="font-size:12px;">
                    </div>
                </div>
            </div>

            <div style="background:#0d1117; border:1px solid #d29922; border-radius:6px; padding:10px; margin-bottom:14px; font-size:11px; color:#d29922;">
                <i class="fas fa-exclamation-triangle"></i> 端口冲突校验: Web控制台 / Agent回连 / Agent TCP / Shell TCP / 内网穿透端口必须互不相同。修改端口/协议后需重启服务生效。
            </div>
            <div style="display:flex; gap:10px;">
                <button class="btn btn-primary" onclick="app.saveListenSettings()">
                    <i class="fas fa-save"></i> 保存监听配置
                </button>
                <button class="btn btn-secondary" onclick="app.testListenConnection()">
                    <i class="fas fa-plug"></i> 测试当前监听
                </button>
            </div>
        </div>
    `;
};

// reloadFileConfig 重新加载 config.json（修改 web ip/port 后刷新内存配置）
app.reloadFileConfig = async function() {
    try {
        const res = await API.post('/api/settings/reload_config', {});
        if (res.success) {
            this._notify(res.message || 'config.json 已重新加载', 'success', 6000);
            // 刷新配置数据
            await this.loadSettings();
        } else {
            this._notify('加载失败', 'error');
        }
    } catch(e) {
        this._notify('加载失败: ' + e.message, 'error');
    }
};

// ============== 通信加密 ==============
app._renderCryptoPanel = function() {
    const e = this._settingsData.encryption || {};
    const aesKeyLen = (e.aes_key || '').length;
    return `
        <div style="max-width:680px;">
            <div style="background:#0d1117; border:1px solid #bc8cff; border-radius:8px; padding:12px; margin-bottom:16px;">
                <div style="color:#bc8cff; font-weight:600; margin-bottom:6px;"><i class="fas fa-shield-halved"></i> 流量通信加密 (参考 CS Cryptographic Specifier)</div>
                <div style="color:#8b949e; font-size:12px; line-height:1.6;">
                    配置客户端与服务端之间通信流量的加密方式。<br>
                    生成的 Payload 会使用此处配置的算法和密码对回传数据进行加密。<br>
                    <span style="color:#d29922;">⚠ 修改加密配置后，之前生成的 Payload 将无法正常通信，需重新生成。</span>
                </div>
            </div>
            <div style="margin-bottom:14px;">
                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">加密算法</label>
                <select class="select" id="setEncAlgo" onchange="app._onEncAlgoChange()">
                    <option value="aes-128-cbc" ${(e.algorithm === 'aes-128-cbc' || e.algorithm === 'aes') ? 'selected' : ''}>AES-128-CBC (推荐)</option>
                    <option value="aes-256-cbc" ${e.algorithm === 'aes-256-cbc' ? 'selected' : ''}>AES-256-CBC</option>
                    <option value="rc4" ${e.algorithm === 'rc4' ? 'selected' : ''}>RC4 流加密</option>
                    <option value="chacha20" ${e.algorithm === 'chacha20' ? 'selected' : ''}>ChaCha20</option>
                    <option value="xor" ${e.algorithm === 'xor' ? 'selected' : ''}>XOR 异或</option>
                    <option value="none" ${e.algorithm === 'none' ? 'selected' : ''}>不加密 (仅演示用)</option>
                </select>
            </div>
            <div style="margin-bottom:14px;">
                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">
                    加密密码 <span style="color:#6e7681;">(Payload 生成时的默认密码)</span>
                </label>
                <div style="display:flex; gap:8px;">
                    <input type="text" class="input" id="setEncPwd" value="${e.password || ''}" placeholder="加密密码">
                    <button class="btn btn-secondary" onclick="app._genRandomPwd()" title="随机生成"><i class="fas fa-dice"></i></button>
                </div>
                <div style="font-size:11px; color:#6e7681; margin-top:4px;">用于 Payload 内嵌的密钥派生（SHA256 截取作为 AES key）</div>
            </div>
            <div style="margin-bottom:14px;">
                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">
                    AES Key (16/24/32 字节)
                    <span style="color:${aesKeyLen === 16 || aesKeyLen === 24 || aesKeyLen === 32 ? '#3fb950' : '#f85149'};">[${aesKeyLen} 字节]</span>
                </label>
                <input type="text" class="input" id="setAesKey" value="${e.aes_key || ''}" placeholder="如: C2DemoKey2024!!!">
                <div style="font-size:11px; color:#6e7681; margin-top:4px;">服务端使用的 AES Key，必须为 16/24/32 字节</div>
            </div>
            <div style="margin-bottom:20px;">
                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">XOR Key</label>
                <input type="text" class="input" id="setXorKey" value="${e.xor_key || ''}" placeholder="如: c2demo">
                <div style="font-size:11px; color:#6e7681; margin-top:4px;">XOR 加密模式使用的密钥，任意长度字符串</div>
            </div>
            <button class="btn btn-primary" onclick="app.saveCryptoSettings()">
                <i class="fas fa-save"></i> 保存加密配置
            </button>
        </div>
    `;
};

app._onEncAlgoChange = function() {
    const algo = document.getElementById('setEncAlgo').value;
    const aesInput = document.getElementById('setAesKey');
    const xorInput = document.getElementById('setXorKey');
    const pwdInput = document.getElementById('setEncPwd');
    const isAES = algo.startsWith('aes-');
    const isXOR = algo === 'xor';
    if (aesInput) aesInput.disabled = !isAES;
    if (xorInput) xorInput.disabled = !isXOR;
    if (pwdInput) pwdInput.disabled = (algo === 'none');
};

app._genRandomPwd = function() {
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*';
    let pwd = '';
    for (let i = 0; i < 24; i++) pwd += chars[Math.floor(Math.random() * chars.length)];
    document.getElementById('setEncPwd').value = pwd;
    this._notify('已生成随机密码: ' + pwd, 'success');
};

// ============== 账户管理 ==============
app._renderAccountPanel = function() {
    const users = this._usersList || [];
    return `
        <div style="max-width:760px;">
            <div style="background:#0d1117; border:1px solid #238636; border-radius:8px; padding:12px; margin-bottom:16px;">
                <div style="color:#3fb950; font-weight:600; margin-bottom:6px;"><i class="fas fa-user-shield"></i> 中台账户管理</div>
                <div style="color:#8b949e; font-size:12px; line-height:1.6;">
                    修改当前登录用户密码，或创建/删除操作员账户（仅管理员可见）。
                </div>
            </div>
            <div class="card" style="padding:16px; margin-bottom:20px; border:1px solid #21262d;">
                <h4 style="font-size:14px; margin-bottom:14px; color:#58a6ff;"><i class="fas fa-key"></i> 修改我的密码</h4>
                <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:12px; margin-bottom:14px;">
                    <div>
                        <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">原密码</label>
                        <input type="password" class="input" id="setOldPwd" placeholder="当前密码">
                    </div>
                    <div>
                        <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">新密码</label>
                        <input type="password" class="input" id="setNewPwd" placeholder="至少6位">
                    </div>
                    <div>
                        <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">确认新密码</label>
                        <input type="password" class="input" id="setConfirmPwd" placeholder="再次输入">
                    </div>
                </div>
                <button class="btn btn-primary" onclick="app.savePassword()">
                    <i class="fas fa-save"></i> 修改密码
                </button>
            </div>
            <div class="card" style="padding:16px; border:1px solid #21262d;">
                <div style="display:flex; align-items:center; margin-bottom:14px;">
                    <h4 style="font-size:14px; color:#58a6ff; flex:1;"><i class="fas fa-users"></i> 操作员列表</h4>
                    <button class="btn btn-blue" onclick="app._showCreateUserForm()" style="padding:6px 12px; font-size:12px;">
                        <i class="fas fa-user-plus"></i> 新建用户
                    </button>
                </div>
                <div id="createUserForm" style="display:none; background:#0d1117; padding:14px; border-radius:8px; margin-bottom:14px;">
                    <div style="display:grid; grid-template-columns:2fr 2fr 1fr; gap:12px; margin-bottom:10px;">
                        <input type="text" class="input" id="newUserName" placeholder="用户名">
                        <input type="password" class="input" id="newUserPwd" placeholder="密码 (至少6位)">
                        <select class="select" id="newUserRole">
                            <option value="user">普通用户</option>
                            <option value="admin">管理员</option>
                        </select>
                    </div>
                    <div style="display:flex; gap:8px;">
                        <button class="btn btn-primary" onclick="app.createUser()" style="padding:6px 14px; font-size:12px;"><i class="fas fa-check"></i> 创建</button>
                        <button class="btn btn-secondary" onclick="document.getElementById('createUserForm').style.display='none'" style="padding:6px 14px; font-size:12px;">取消</button>
                    </div>
                </div>
                <table style="width:100%; font-size:13px;">
                    <thead>
                        <tr style="border-bottom:1px solid #21262d;">
                            <th style="text-align:left; padding:8px; color:#8b949e;">ID</th>
                            <th style="text-align:left; padding:8px; color:#8b949e;">用户名</th>
                            <th style="text-align:left; padding:8px; color:#8b949e;">角色</th>
                            <th style="text-align:left; padding:8px; color:#8b949e;">创建时间</th>
                            <th style="text-align:right; padding:8px; color:#8b949e;">操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${users.length === 0 ? `<tr><td colspan="5" style="text-align:center; padding:20px; color:#6e7681;">加载中...</td></tr>` :
                          users.map(u => `
                            <tr style="border-bottom:1px solid #161b22;">
                                <td style="padding:8px;">${u.id}</td>
                                <td style="padding:8px;">
                                    <i class="fas fa-user-circle" style="color:#58a6ff; margin-right:6px;"></i>${u.username}
                                </td>
                                <td style="padding:8px;">
                                    <span class="badge ${u.role === 'admin' ? 'badge-green' : 'badge-blue'}">${u.role === 'admin' ? '管理员' : '普通用户'}</span>
                                </td>
                                <td style="padding:8px; color:#8b949e; font-size:12px;">${u.created_at || '-'}</td>
                                <td style="padding:8px; text-align:right;">
                                    ${u.username === 'admin' ? '<span style="color:#6e7681; font-size:11px;">内置账户</span>' :
                                      `<button class="btn btn-danger" style="padding:4px 10px; font-size:11px;" onclick="app.deleteUser(${u.id}, '${u.username}')"><i class="fas fa-trash"></i></button>`}
                                </td>
                            </tr>
                          `).join('')}
                    </tbody>
                </table>
            </div>
        </div>
    `;
};

app._showCreateUserForm = function() {
    const f = document.getElementById('createUserForm');
    if (f) f.style.display = 'block';
};

// ============== 客户端参数 ==============
app._renderClientPanel = function() {
    const c = this._settingsData.client || {};
    return `
        <div style="max-width:680px;">
            <div style="background:#0d1117; border:1px solid #238636; border-radius:8px; padding:12px; margin-bottom:16px;">
                <div style="color:#3fb950; font-weight:600; margin-bottom:6px;"><i class="fas fa-sliders"></i> 客户端行为参数</div>
                <div style="color:#8b949e; font-size:12px; line-height:1.6;">
                    配置被控端（agent.py）的心跳和轮询行为。修改后对新上线的客户端生效。
                </div>
            </div>
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:16px; margin-bottom:14px;">
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">心跳间隔 (秒)</label>
                    <input type="number" class="input" id="setHeartbeat" value="${c.heartbeat_interval || 5}" min="1" max="300">
                    <div style="font-size:11px; color:#6e7681; margin-top:4px;">客户端向服务端报告存活状态的频率</div>
                </div>
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">任务轮询间隔 (秒)</label>
                    <input type="number" class="input" id="setTaskPoll" value="${c.task_poll_interval || 3}" min="1" max="120">
                    <div style="font-size:11px; color:#6e7681; margin-top:4px;">客户端检查待执行任务的频率</div>
                </div>
            </div>
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:16px; margin-bottom:20px;">
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">离线判定超时 (秒)</label>
                    <input type="number" class="input" id="setOfflineTimeout" value="${c.offline_timeout || 60}" min="10" max="3600">
                    <div style="font-size:11px; color:#6e7681; margin-top:4px;">超过此时间未收到心跳则标记为离线</div>
                </div>
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">最大重连次数</label>
                    <input type="number" class="input" id="setReconnectMax" value="${c.reconnect_max || 30}" min="1" max="9999">
                    <div style="font-size:11px; color:#6e7681; margin-top:4px;">连接失败后客户端最大重试次数</div>
                </div>
            </div>
            <button class="btn btn-primary" onclick="app.saveClientSettings()">
                <i class="fas fa-save"></i> 保存客户端参数
            </button>
        </div>
    `;
};

// ============== 任务限制 ==============
app._renderLimitsPanel = function() {
    const l = this._settingsData.limits || {};
    return `
        <div style="max-width:680px;">
            <div style="background:#0d1117; border:1px solid #238636; border-radius:8px; padding:12px; margin-bottom:16px;">
                <div style="color:#3fb950; font-weight:600; margin-bottom:6px;"><i class="fas fa-gauge"></i> 任务执行限制</div>
                <div style="color:#8b949e; font-size:12px; line-height:1.6;">
                    配置各类任务的大小/时长上限，避免资源耗尽。
                </div>
            </div>
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:16px; margin-bottom:14px;">
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">截图最大分辨率</label>
                    <select class="select" id="setShotRes">
                        <option value="360p" ${l.screenshot_max_resolution === '360p' ? 'selected' : ''}>360p (最小)</option>
                        <option value="720p" ${l.screenshot_max_resolution === '720p' ? 'selected' : ''}>720p</option>
                        <option value="1080p" ${l.screenshot_max_resolution === '1080p' ? 'selected' : ''}>1080p (最大)</option>
                    </select>
                </div>
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">录屏/录音最大时长 (秒)</label>
                    <input type="number" class="input" id="setRecMax" value="${l.record_max_duration || 60}" min="3" max="300">
                </div>
            </div>
            <div style="margin-bottom:20px;">
                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">文件上传大小限制 (MB)</label>
                <input type="number" class="input" id="setFileMax" value="${l.file_upload_max_mb || 50}" min="1" max="500">
                <div style="font-size:11px; color:#6e7681; margin-top:4px;">从服务端向目标主机上传文件的最大大小</div>
            </div>
            <button class="btn btn-primary" onclick="app.saveLimitsSettings()">
                <i class="fas fa-save"></i> 保存任务限制
            </button>
        </div>
    `;
};

// ============== 安全策略 ==============
app._renderSecurityPanel = function() {
    const s = this._settingsData.security || {};
    return `
        <div style="max-width:680px;">
            <div style="background:#0d1117; border:1px solid #f85149; border-radius:8px; padding:12px; margin-bottom:16px;">
                <div style="color:#f85149; font-weight:600; margin-bottom:6px;"><i class="fas fa-lock"></i> 安全策略</div>
                <div style="color:#8b949e; font-size:12px; line-height:1.6;">
                    配置登录会话超时、登录失败锁定等安全策略。
                </div>
            </div>
            <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:16px; margin-bottom:20px;">
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">会话超时 (秒)</label>
                    <input type="number" class="input" id="setSessionTimeout" value="${s.session_timeout || 86400}" min="300" max="604800">
                    <div style="font-size:11px; color:#6e7681; margin-top:4px;">默认86400(24h)</div>
                </div>
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">最大登录失败次数</label>
                    <input type="number" class="input" id="setMaxLoginAttempts" value="${s.max_login_attempts || 5}" min="3" max="20">
                    <div style="font-size:11px; color:#6e7681; margin-top:4px;">超过此次数将锁定IP</div>
                </div>
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">登录锁定时长 (分钟)</label>
                    <input type="number" class="input" id="setLockMinutes" value="${s.login_lock_minutes || 15}" min="1" max="1440">
                    <div style="font-size:11px; color:#6e7681; margin-top:4px;">达到最大失败次数后的锁定时间</div>
                </div>
            </div>
            <button class="btn btn-primary" onclick="app.saveSecuritySettings()">
                <i class="fas fa-save"></i> 保存安全策略
            </button>
        </div>
    `;
};

// ============== Webhook ==============
app._renderWebhookPanel = function() {
    const w = this._settingsData.webhook || {};
    return `
        <div style="max-width:680px;">
            <div style="background:#0d1117; border:1px solid #238636; border-radius:8px; padding:12px; margin-bottom:16px;">
                <div style="color:#3fb950; font-weight:600; margin-bottom:6px;"><i class="fas fa-bell"></i> Webhook 通知</div>
                <div style="color:#8b949e; font-size:12px; line-height:1.6;">
                    配置事件通知 Webhook，支持企业微信/钉钉/飞书/自定义 HTTP 接收端。
                </div>
            </div>
            <div style="margin-bottom:14px;">
                <label style="display:flex; align-items:center; gap:8px; cursor:pointer;">
                    <input type="checkbox" id="setWebhookEnabled" ${w.enabled ? 'checked' : ''} style="width:16px; height:16px;">
                    <span style="font-size:13px;">启用 Webhook 通知</span>
                </label>
            </div>
            <div style="margin-bottom:14px;">
                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">Webhook URL</label>
                <input type="text" class="input" id="setWebhookUrl" value="${w.url || ''}" placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx">
            </div>
            <div style="margin-bottom:20px;">
                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">订阅事件 (逗号分隔)</label>
                <input type="text" class="input" id="setWebhookEvents" value="${w.events || ''}" placeholder="login,client_online,payload,task">
                <div style="font-size:11px; color:#6e7681; margin-top:4px;">可选: login, client_online, payload, task</div>
            </div>
            <div style="display:flex; gap:10px;">
                <button class="btn btn-primary" onclick="app.saveWebhookSettings()">
                    <i class="fas fa-save"></i> 保存 Webhook
                </button>
                <button class="btn btn-secondary" onclick="app.testWebhookFromSettings()">
                    <i class="fas fa-paper-plane"></i> 发送测试消息
                </button>
            </div>
        </div>
    `;
};

// ============== 数据加载与保存方法 ==============
app.loadSettings = function() {
    API.get('/api/settings').then(data => {
        this._settingsData = data;
        this.render();
    }).catch(e => {
        this._notify('加载配置失败: ' + e.message, 'error');
    });
};

app.loadUsers = function() {
    API.get('/api/users').then(data => {
        this._usersList = Array.isArray(data) ? data : [];
        if (this._settingsTab === 'account') {
            const panel = document.getElementById('settingsPanel');
            if (panel) panel.innerHTML = this._renderSettingsPanel();
        }
    }).catch(e => {
        this._usersList = [];
        // 非管理员或失败时静默
        if (this._settingsTab === 'account') {
            const panel = document.getElementById('settingsPanel');
            if (panel) panel.innerHTML = this._renderSettingsPanel();
        }
    });
};

app.saveListenSettings = function() {
    const data = {
        listen: {
            callback_host: document.getElementById('setCallbackHost').value.trim(),
            client_listen_port: document.getElementById('setClientListenPort').value.trim(),
            agent_protocol: document.getElementById('setAgentProtocol').value.trim(),
            agent_tcp_port: document.getElementById('setAgentTcpPort').value.trim(),
            shell_listen_port: document.getElementById('setShellListenPort').value.trim(),
            tunnel_port_forward: document.getElementById('setTunnelPortForward').value.trim(),
            tunnel_socks5_port: document.getElementById('setTunnelSocks5').value.trim(),
            tunnel_http_proxy_port: document.getElementById('setTunnelHttpProxy').value.trim(),
        }
    };
    const loading = this._notify('正在保存监听配置...', 'loading', 0);
    API.put('/api/settings', data).then(res => {
        const msg = res.restart_required
            ? '监听配置已保存，需重启服务生效（端口/协议已更改）'
            : '监听配置已保存';
        this._notify(msg, 'success', 6000);
        this._settingsData.listen = {...this._settingsData.listen, ...data.listen};
    }).catch(e => {
        this._notify('保存失败: ' + e.message, 'error');
    }).finally(() => { if (loading) loading.remove(); });
};

app.saveCryptoSettings = function() {
    const algo = document.getElementById('setEncAlgo').value;
    const pwd = document.getElementById('setEncPwd').value;
    const aesKey = document.getElementById('setAesKey').value;
    const xorKey = document.getElementById('setXorKey').value;
    if (algo === 'none') {
        if (!pwd) { this._notify('即使不加密也建议设置密码（用于后续切换）', 'warning'); }
    } else {
        if (!pwd) { this._notify('加密密码不能为空', 'error'); return; }
        if (pwd.length < 8) { this._notify('加密密码建议至少 8 个字符', 'warning'); }
    }
    const data = {
        encryption: { algorithm: algo, password: pwd, aes_key: aesKey, xor_key: xorKey }
    };
    const loading = this._notify('正在保存加密配置...', 'loading', 0);
    API.put('/api/settings', data).then(res => {
        this._notify('加密配置已保存。之前生成的 Payload 需重新生成', 'success', 6000);
        this._settingsData.encryption = {...this._settingsData.encryption, ...data.encryption};
    }).catch(e => {
        this._notify('保存失败: ' + e.message, 'error');
    }).finally(() => { if (loading) loading.remove(); });
};

app.savePassword = function() {
    const oldPwd = document.getElementById('setOldPwd').value;
    const newPwd = document.getElementById('setNewPwd').value;
    const confirm = document.getElementById('setConfirmPwd').value;
    if (!oldPwd || !newPwd) { this._notify('请填写原密码和新密码', 'error'); return; }
    if (newPwd !== confirm) { this._notify('两次输入的新密码不一致', 'error'); return; }
    if (newPwd.length < 6) { this._notify('新密码长度至少6位', 'error'); return; }
    const loading = this._notify('正在修改密码...', 'loading', 0);
    API.post('/api/user/password', { old_password: oldPwd, new_password: newPwd, confirm_password: confirm })
    .then(res => {
        this._notify('密码修改成功，下次登录请使用新密码', 'success', 6000);
        document.getElementById('setOldPwd').value = '';
        document.getElementById('setNewPwd').value = '';
        document.getElementById('setConfirmPwd').value = '';
    }).catch(e => {
        this._notify('修改失败: ' + e.message, 'error');
    }).finally(() => { if (loading) loading.remove(); });
};

app.createUser = function() {
    const uname = document.getElementById('newUserName').value.trim();
    const pwd = document.getElementById('newUserPwd').value;
    const role = document.getElementById('newUserRole').value;
    if (!uname || !pwd) { this._notify('用户名和密码不能为空', 'error'); return; }
    if (pwd.length < 6) { this._notify('密码长度至少6位', 'error'); return; }
    const loading = this._notify('正在创建用户...', 'loading', 0);
    API.post('/api/users', { username: uname, password: pwd, role: role })
    .then(res => {
        this._notify('用户 ' + uname + ' 创建成功', 'success');
        document.getElementById('newUserName').value = '';
        document.getElementById('newUserPwd').value = '';
        document.getElementById('createUserForm').style.display = 'none';
        this.loadUsers();
    }).catch(e => {
        this._notify('创建失败: ' + e.message, 'error');
    }).finally(() => { if (loading) loading.remove(); });
};

app.deleteUser = function(uid, uname) {
    if (!confirm('确定要删除用户 ' + uname + ' 吗？此操作不可恢复。')) return;
    const loading = this._notify('正在删除用户...', 'loading', 0);
    API.del('/api/users/' + uid)
    .then(res => {
        this._notify('用户 ' + uname + ' 已删除', 'success');
        this.loadUsers();
    }).catch(e => {
        this._notify('删除失败: ' + e.message, 'error');
    }).finally(() => { if (loading) loading.remove(); });
};

app.saveClientSettings = function() {
    const data = {
        client: {
            heartbeat_interval: parseInt(document.getElementById('setHeartbeat').value) || 5,
            task_poll_interval: parseInt(document.getElementById('setTaskPoll').value) || 3,
            offline_timeout: parseInt(document.getElementById('setOfflineTimeout').value) || 60,
            reconnect_max: parseInt(document.getElementById('setReconnectMax').value) || 30,
        }
    };
    const loading = this._notify('正在保存客户端参数...', 'loading', 0);
    API.put('/api/settings', data).then(res => {
        this._notify('客户端参数已保存', 'success');
        this._settingsData.client = {...this._settingsData.client, ...data.client};
    }).catch(e => {
        this._notify('保存失败: ' + e.message, 'error');
    }).finally(() => { if (loading) loading.remove(); });
};

app.saveLimitsSettings = function() {
    const data = {
        limits: {
            screenshot_max_resolution: document.getElementById('setShotRes').value,
            record_max_duration: parseInt(document.getElementById('setRecMax').value) || 60,
            file_upload_max_mb: parseInt(document.getElementById('setFileMax').value) || 50,
        }
    };
    const loading = this._notify('正在保存任务限制...', 'loading', 0);
    API.put('/api/settings', data).then(res => {
        this._notify('任务限制已保存', 'success');
        this._settingsData.limits = {...this._settingsData.limits, ...data.limits};
    }).catch(e => {
        this._notify('保存失败: ' + e.message, 'error');
    }).finally(() => { if (loading) loading.remove(); });
};

app.saveSecuritySettings = function() {
    const data = {
        security: {
            session_timeout: parseInt(document.getElementById('setSessionTimeout').value) || 86400,
            max_login_attempts: parseInt(document.getElementById('setMaxLoginAttempts').value) || 5,
            login_lock_minutes: parseInt(document.getElementById('setLockMinutes').value) || 15,
        }
    };
    const loading = this._notify('正在保存安全策略...', 'loading', 0);
    API.put('/api/settings', data).then(res => {
        this._notify('安全策略已保存', 'success');
        this._settingsData.security = {...this._settingsData.security, ...data.security};
    }).catch(e => {
        this._notify('保存失败: ' + e.message, 'error');
    }).finally(() => { if (loading) loading.remove(); });
};

app.saveWebhookSettings = function() {
    const data = {
        webhook: {
            enabled: document.getElementById('setWebhookEnabled').checked,
            url: document.getElementById('setWebhookUrl').value.trim(),
            events: document.getElementById('setWebhookEvents').value.trim(),
        }
    };
    const loading = this._notify('正在保存 Webhook 配置...', 'loading', 0);
    API.put('/api/settings', data).then(res => {
        this._notify('Webhook 配置已保存', 'success');
        this._settingsData.webhook = {...this._settingsData.webhook, ...data.webhook};
    }).catch(e => {
        this._notify('保存失败: ' + e.message, 'error');
    }).finally(() => { if (loading) loading.remove(); });
};

app.testWebhookFromSettings = function() {
    const url = document.getElementById('setWebhookUrl').value.trim();
    if (!url) { this._notify('请先填写 Webhook URL', 'error'); return; }
    const loading = this._notify('正在发送测试消息...', 'loading', 0);
    API.post('/api/settings/webhook/test', { url: url })
    .then(res => {
        this._notify('测试消息发送成功！状态码: ' + res.status_code, 'success');
    }).catch(e => {
        this._notify('测试失败: ' + e.message, 'error');
    }).finally(() => { if (loading) loading.remove(); });
};

app.testListenConnection = function() {
    const loading = this._notify('正在测试监听...', 'loading', 0);
    API.get('/api/settings/test').then(res => {
        this._notify(res.message || '监听正常', 'success', 6000);
    }).catch(e => {
        this._notify('测试失败: ' + e.message, 'error');
    }).finally(() => { if (loading) loading.remove(); });
};
