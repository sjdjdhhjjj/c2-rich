// app-render.js - App 渲染相关方法（Tab 标签页管理，参考 CS/浏览器多标签）

// ============== Tab 管理系统 ==============
// 每个 Tab 独立保持 page + selectedClient 状态，切换 Tab 不干扰其他 Tab
// tabs 数组结构: [{ id, page, title, icon, clientId, closable }]
// activeTabId 当前激活的 Tab

app.tabs = [];           // 已打开的 Tab 列表
app.activeTabId = null;  // 当前激活的 Tab
app._tabSeq = 0;         // Tab ID 自增序列
app._tabStates = {};     // 每个 Tab 的独立状态: { tabId: { selectedClient, fileCurrentPath, fileList, ... } }

app.renderMain = function() {
    return `
        <div style="display:flex; min-height:100vh;">
            ${this.renderSidebar()}
            <div style="flex:1; overflow:hidden; display:flex; flex-direction:column;">
                ${this.renderTabBar()}
                <div style="flex:1; overflow-y:auto; padding:20px;" id="tabContentArea">
                    ${this.renderPage()}
                </div>
            </div>
        </div>
    `;
};

app.renderSidebar = function() {
    const menuItems = [
        { id: 'dashboard', icon: 'fa-gauge-high', label: '仪表盘' },
        { id: 'clients', icon: 'fa-desktop', label: '主机管理' },
        { id: 'webshells', icon: 'fa-globe', label: 'WebShell 管理' },
        { id: 'terminal', icon: 'fa-terminal', label: '命令终端' },
        { id: 'files', icon: 'fa-folder-open', label: '文件管理' },
        { id: 'screenshot', icon: 'fa-camera', label: '屏幕监控' },
        { id: 'payloads', icon: 'fa-bomb', label: 'Payload生成' },
        { id: 'cmdgen', icon: 'fa-code', label: '命令生成器' },
        { id: 'tunnel', icon: 'fa-network-wired', label: '内网穿透' },
        { id: 'tasks', icon: 'fa-tasks', label: '任务列表' },
        { id: 'logs', icon: 'fa-file-alt', label: '日志审计' },
        { id: 'settings', icon: 'fa-cog', label: '配置管理' },
    ];

    return `
        <div style="width:220px; background:#0d1117; border-right:1px solid #21262d; display:flex; flex-direction:column;">
            <div style="padding:20px; border-bottom:1px solid #21262d;">
                <div style="display:flex; align-items:center; gap:10px;">
                    <div style="font-size:28px; color:#3fb950;" class="glow-green">
                        <i class="fas fa-crosshairs"></i>
                    </div>
                    <div>
                        <div style="font-weight:700; font-size:15px;">C2 控制台</div>
                        <div style="font-size:10px; color:#8b949e;">v2.0 Tab版</div>
                    </div>
                </div>
            </div>
            <div style="flex:1; padding:12px 0;">
                ${menuItems.map(item => `
                    <div class="sidebar-item ${this.page === item.id ? 'active' : ''}" onclick="app.openTab('${item.id}')">
                        <i class="fas ${item.icon}"></i>
                        <span>${item.label}</span>
                    </div>
                `).join('')}
            </div>
            <div style="padding:12px; border-top:1px solid #21262d;">
                <div style="display:flex; align-items:center; gap:10px; padding:8px;">
                    <div style="width:32px; height:32px; border-radius:50%; background:#238636; display:flex; align-items:center; justify-content:center;">
                        <i class="fas fa-user" style="font-size:14px;"></i>
                    </div>
                    <div style="flex:1;">
                        <div style="font-size:13px; font-weight:600;">admin</div>
                        <div style="font-size:11px; color:#8b949e;">超级管理员</div>
                    </div>
                    <i class="fas fa-sign-out-alt" style="color:#8b949e; cursor:pointer;" onclick="app.logout()"></i>
                </div>
            </div>
        </div>
    `;
};

// Tab 栏渲染
app.renderTabBar = function() {
    const online = this.clients.filter(c => c.status === 'online').length;
    const total = this.clients.length;

    return `
        <div style="background:#0d1117; border-bottom:1px solid #21262d; display:flex; flex-direction:column;">
            <!-- 顶部信息栏 -->
            <div style="padding:8px 24px; display:flex; align-items:center; justify-content:space-between; border-bottom:1px solid #161b22;">
                <div style="display:flex; align-items:center; gap:16px; font-size:12px;">
                    <span class="status-online"><span class="status-dot online"></span>在线 ${online}</span>
                    <span class="status-offline"><span class="status-dot offline"></span>离线 ${total - online}</span>
                    <span style="color:#8b949e;">总数 ${total}</span>
                    <span style="color:#8b949e;">|</span>
                    <span style="color:#58a6ff;">Tab: ${this.tabs.length}</span>
                </div>
                <div style="display:flex; gap:10px; align-items:center;">
                    <button class="btn btn-secondary" style="font-size:11px; padding:4px 10px;" onclick="app.closeAllTabs()" title="关闭所有Tab">
                        <i class="fas fa-times-circle"></i> 关闭全部
                    </button>
                    <span style="font-size:12px; color:#8b949e;">
                        <i class="far fa-clock"></i> ${new Date().toLocaleString('zh-CN')}
                    </span>
                </div>
            </div>
            <!-- Tab 标签栏 -->
            <div style="display:flex; align-items:center; padding:0 8px; overflow-x:auto; background:#161b22; min-height:36px;">
                ${this.tabs.map(tab => `
                    <div class="tab-bar-item ${this.activeTabId === tab.id ? 'active' : ''}" onclick="app.switchTab(${tab.id})">
                        <i class="fas ${tab.icon}" style="font-size:11px; margin-right:6px;"></i>
                        <span style="font-size:12px;">${tab.title}</span>
                        ${tab.closable ? `<i class="fas fa-times" style="font-size:10px; margin-left:8px; opacity:0.6;" onclick="event.stopPropagation(); app.closeTab(${tab.id})"></i>` : ''}
                    </div>
                `).join('')}
            </div>
        </div>
    `;
};

// 打开新 Tab（或激活已存在的同名 Tab）
app.openTab = function(page, clientId) {
    // 如果指定了 clientId，每次都开新 Tab（允许对多个主机同时操作）
    // 如果没指定 clientId，同名 page 只开一个 Tab
    if (!clientId) {
        const existing = this.tabs.find(t => t.page === page && !t.clientId);
        if (existing) {
            this.switchTab(existing.id);
            return;
        }
    }

    const titles = {
        dashboard: '仪表盘', clients: '主机管理', webshells: 'WebShell 管理', terminal: '命令终端',
        files: '文件管理', screenshot: '屏幕监控', payloads: 'Payload生成',
        cmdgen: '命令生成器', tunnel: '内网穿透', tasks: '任务列表', logs: '日志审计',
        settings: '配置管理'
    };
    const icons = {
        dashboard: 'fa-gauge-high', clients: 'fa-desktop', webshells: 'fa-globe', terminal: 'fa-terminal',
        files: 'fa-folder-open', screenshot: 'fa-camera', payloads: 'fa-bomb',
        cmdgen: 'fa-code', tunnel: 'fa-network-wired', tasks: 'fa-tasks', logs: 'fa-file-alt',
        settings: 'fa-cog'
    };

    this._tabSeq++;
    const tabId = this._tabSeq;
    let title = titles[page] || page;
    let client = null;

    if (clientId) {
        // 用 _allClients（含 WebShell）查找，否则从 WebShell 管理进入操作页面时找不到
        client = (this._allClients || this.clients || []).find(c => c.client_id === clientId);
        if (client) {
            title = `${titles[page] || page} - ${client.hostname}`;
        }
    } else {
        // 从侧边栏打开操作类页面时，继承当前已选客户端（提升交互流畅度）
        if (['terminal', 'files', 'screenshot'].includes(page) && this.selectedClient) {
            client = this.selectedClient;
        }
    }

    const tab = {
        id: tabId,
        page: page,
        title: title,
        icon: icons[page] || 'fa-window-maximize',
        clientId: clientId,
        closable: page !== 'dashboard'  // 仪表盘不可关闭
    };

    // 保存 Tab 独立状态
    this._tabStates[tabId] = {
        selectedClient: client,
        fileCurrentPath: '.',
        fileList: [],
        terminalHistory: [],
        terminalOutput: ''
    };

    this.tabs.push(tab);
    this.activeTabId = tabId;
    // 关键修复：必须同步设置 this.page，否则 renderPage() 渲染的是旧页面
    this.page = page;
    this._applyTabState(tabId);

    // 加载数据
    this._loadPageData(page);
    this.render();
};

// 切换 Tab
app.switchTab = function(tabId) {
    // 先保存当前 Tab 状态（包括终端历史）
    if (this.activeTabId && this._tabStates[this.activeTabId]) {
        this._tabStates[this.activeTabId].selectedClient = this.selectedClient;
        this._tabStates[this.activeTabId].fileCurrentPath = this.fileCurrentPath;
        this._tabStates[this.activeTabId].fileList = this.fileList;
        this._tabStates[this.activeTabId].terminalHistory = this.terminalHistory || [];
    }

    this.activeTabId = tabId;
    this._applyTabState(tabId);

    const tab = this.tabs.find(t => t.id === tabId);
    if (tab) {
        this.page = tab.page;
    }
    this.render();
};

// 关闭 Tab
app.closeTab = function(tabId) {
    const idx = this.tabs.findIndex(t => t.id === tabId);
    if (idx === -1) return;
    if (this.tabs[idx].page === 'dashboard') return; // 仪表盘不可关闭

    this.tabs.splice(idx, 1);
    delete this._tabStates[tabId];

    // 如果关闭的是当前激活的 Tab，切换到最后一个
    if (this.activeTabId === tabId) {
        if (this.tabs.length > 0) {
            this.switchTab(this.tabs[this.tabs.length - 1].id);
        } else {
            // 没有 Tab 了，回到仪表盘
            this.openTab('dashboard');
        }
    } else {
        this.render();
    }
};

// 关闭所有 Tab（保留仪表盘）
app.closeAllTabs = function() {
    if (!confirm('确认关闭所有 Tab（保留仪表盘）？')) return;
    this.tabs = this.tabs.filter(t => t.page === 'dashboard');
    this._tabStates = {};
    if (this.tabs.length > 0) {
        this.activeTabId = this.tabs[0].id;
        this.page = 'dashboard';
    }
    this.render();
};

// 应用 Tab 状态到 app 对象
app._applyTabState = function(tabId) {
    const state = this._tabStates[tabId];
    if (!state) return;
    this.selectedClient = state.selectedClient;
    this.fileCurrentPath = state.fileCurrentPath || '.';
    this.fileList = state.fileList || [];
    this.terminalHistory = state.terminalHistory || [];
};

// 加载页面数据
app._loadPageData = function(page) {
    if (page === 'dashboard') this.loadStats();
    if (page === 'clients') this.loadClients();
    if (page === 'webshells') this.loadWebshells();
    if (page === 'tasks') this.loadTasks();
    if (page === 'logs') this.loadLogs();
    if (page === 'payloads') {
        // 刷新配置缓存，确保渲染时用的是最新的配置管理值
        this.loadSettingsCache().then(() => {
            this.loadPayloads();
            this.render();
        });
    }
    if (page === 'settings') this.loadSettings();
    // 操作类页面需要客户端列表用于选择器下拉框
    if (['terminal', 'files', 'screenshot'].includes(page) && !this._allClients) {
        this.loadClients();
    }
};

// 兼容旧接口：goToPage 改为 openTab
app.goToPage = function(page) {
    this.openTab(page);
};

// 兼容旧接口：goToClientPage 改为 openTab
app.goToClientPage = function(page, clientId) {
    this.openTab(page, clientId);
};

app.getPageTitle = function() {
    const titles = {
        dashboard: '仪表盘', clients: '主机管理', terminal: '命令终端',
        files: '文件管理', screenshot: '屏幕监控', payloads: 'Payload生成器',
        cmdgen: '命令生成器', tunnel: '内网穿透', tasks: '任务列表', logs: '日志审计'
    };
    return titles[this.page] || '';
};

// 通用客户端选择器（终端/文件/屏幕监控页面共用）
// 当未选客户端时显示选择入口；已选时显示客户端信息条
// 数据源 _allClients 包含 Agent + WebShell 两类，通过标签区分
app.renderClientBar = function() {
    const allClients = this._allClients || this.clients || [];
    // 按类型分组：Agent 在前，WebShell 在后
    const agents = allClients.filter(c => c.client_type !== 'webshell');
    const webshells = allClients.filter(c => c.client_type === 'webshell');

    // 生成选项列表（带类型标识），当前选中的主机标记 selected
    const selCid = this.selectedClient ? this.selectedClient.client_id : '';
    const buildOptions = (list) => list.map(cl => {
        const isWs = cl.client_type === 'webshell';
        const tag = isWs ? '[WS]' : '[Agent]';
        const status = isWs ? '●被动' : (cl.status === 'online' ? '●在线' : '○离线');
        const sel = cl.client_id === selCid ? ' selected' : '';
        return `<option value="${cl.client_id}"${sel}>${tag} ${cl.hostname} (${cl.ip || '-'}) ${status}</option>`;
    }).join('');

    if (this.selectedClient) {
        const c = this.selectedClient;
        const isWs = c.client_type === 'webshell';
        const typeBadge = isWs
            ? `<span class="badge badge-blue"><i class="fas fa-globe"></i> WebShell</span>`
            : `<span class="badge" style="background:#21262d; color:#8b949e;">Agent</span>`;
        const statusBadge = isWs
            ? `<span class="badge badge-blue">被动</span>`
            : `<span class="badge badge-${c.status === 'online' ? 'green' : 'red'}">${c.status === 'online' ? '在线' : '离线'}</span>`;
        return `
            <div class="card" style="padding:10px 16px; margin-bottom:16px; display:flex; align-items:center; gap:12px;">
                <span class="status-dot ${c.status}"></span>
                <span style="font-size:14px; font-weight:600;">${c.hostname || '未知'}</span>
                ${typeBadge}
                <span style="color:#8b949e; font-size:12px;">${c.os || ''} ${c.arch || ''} | ${c.ip || '-'} | ${c.username || '-'}</span>
                ${statusBadge}
                <div style="flex:1;"></div>
                <select class="select" style="max-width:240px; font-size:12px;" onchange="app.changeSelectedClient(this.value)">
                    <option value="">切换主机...</option>
                    ${agents.length > 0 ? `<optgroup label="--- Agent ---">${buildOptions(agents)}</optgroup>` : ''}
                    ${webshells.length > 0 ? `<optgroup label="--- WebShell ---">${buildOptions(webshells)}</optgroup>` : ''}
                </select>
            </div>
        `;
    }
    // 未选客户端：显示选择提示
    return `
        <div class="card" style="padding:20px; margin-bottom:16px;">
            <div style="display:flex; align-items:center; gap:16px;">
                <i class="fas fa-hand-pointer" style="font-size:32px; color:#d29922;"></i>
                <div style="flex:1;">
                    <div style="font-size:14px; font-weight:600; margin-bottom:6px;">选择目标主机</div>
                    <div style="font-size:12px; color:#8b949e;">从下方下拉框选择一台主机开始操作（支持 Agent 和 WebShell）</div>
                </div>
                <select class="select" style="max-width:300px;" onchange="app.changeSelectedClient(this.value)">
                    <option value="">-- 选择主机 --</option>
                    ${agents.length > 0 ? `<optgroup label="--- Agent ---">${buildOptions(agents)}</optgroup>` : ''}
                    ${webshells.length > 0 ? `<optgroup label="--- WebShell ---">${buildOptions(webshells)}</optgroup>` : ''}
                </select>
            </div>
        </div>
    `;
};

// 切换当前选中的客户端（不影响其他Tab状态）
app.changeSelectedClient = function(clientId) {
    if (!clientId) return;
    const c = (this._allClients || this.clients || []).find(x => x.client_id === clientId);
    if (!c) return;
    this.selectedClient = c;
    // 同步到当前Tab状态
    if (this.activeTabId && this._tabStates[this.activeTabId]) {
        this._tabStates[this.activeTabId].selectedClient = c;
        // 重置文件路径和终端历史
        this._tabStates[this.activeTabId].fileCurrentPath = '.';
        this.fileCurrentPath = '.';
        this.fileList = [];
        this.terminalHistory = [];
        this._tabStates[this.activeTabId].terminalHistory = [];
        this._cmdHistoryIdx = -1;
    }
    this.render();
};

app.renderPage = function() {
    switch(this.page) {
        case 'dashboard': return this.renderDashboard();
        case 'clients': return this.renderClients();
        case 'webshells': return this.renderWebshells();
        case 'terminal': return this.renderTerminal();
        case 'files': return this.renderFiles();
        case 'screenshot': return this.renderScreenshot();
        case 'payloads': return this.renderPayloads();
        case 'cmdgen': return this.renderCmdGen();
        case 'tunnel': return this.renderTunnel();
        case 'tasks': return this.renderTasks();
        case 'logs': return this.renderLogs();
        case 'settings': return this.renderSettings();
        default: return '';
    }
};
