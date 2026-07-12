// app-logs.js - App 日志与 Webhook 相关方法

app.renderLogs = function() {
    if (this.logSubPage === 'webhook') {
        return this.renderWebhookSettings();
    }
    let filtered = this.logs;
    if (this.logFilterType) filtered = filtered.filter(l => l.type === this.logFilterType);
    const kw = this.logKeyword || '';
    if (kw) filtered = filtered.filter(l => (l.content||'').includes(kw) || (l.client_id||'').includes(kw));
    // 分页计算
    const total = filtered.length;
    const totalPages = Math.max(1, Math.ceil(total / this.logPageSize));
    if (this.logPage > totalPages) this.logPage = totalPages;
    if (this.logPage < 1) this.logPage = 1;
    const startIdx = (this.logPage - 1) * this.logPageSize;
    const pageData = filtered.slice(startIdx, startIdx + this.logPageSize);
    return `
                <div style="margin-bottom:12px; display:flex; gap:8px; border-bottom:1px solid #21262d;">
                    <div class="tab-item ${!this.logSubPage?'active':''}" onclick="app.logSubPage=''; app.render()">日志列表</div>
                    <div class="tab-item ${this.logSubPage==='webhook'?'active':''}" onclick="app.logSubPage='webhook'; app.loadWebhookConfig(); app.render()">Webhook 通知</div>
                </div>
                <div style="margin-bottom:12px; display:flex; gap:10px; align-items:center; flex-wrap:wrap;">
                    <select class="select" style="max-width:150px;" onchange="app.logFilterType=this.value; app.logPage=1; app.render()">
                        <option value="">全部类型</option>
                        <option value="login" ${this.logFilterType==='login'?'selected':''}>登录</option>
                        <option value="client" ${this.logFilterType==='client'?'selected':''}>客户端</option>
                        <option value="task" ${this.logFilterType==='task'?'selected':''}>任务</option>
                        <option value="file" ${this.logFilterType==='file'?'selected':''}>文件</option>
                        <option value="payload" ${this.logFilterType==='payload'?'selected':''}>Payload</option>
                        <option value="group" ${this.logFilterType==='group'?'selected':''}>分组</option>
                        <option value="system" ${this.logFilterType==='system'?'selected':''}>系统</option>
                    </select>
                    <input type="text" class="input" placeholder="搜索内容..." style="max-width:300px;" value="${this.logKeyword||''}" oninput="app.logKeyword=this.value; clearTimeout(app._logTimer); app._logTimer=setTimeout(()=>{app.logPage=1; app.render()}, 300)">
                    <span style="font-size:11px; color:#8b949e;">共 ${total} 条</span>
                    <div style="flex:1;"></div>
                    <button class="btn btn-secondary" onclick="app.loadLogs()"><i class="fas fa-sync-alt"></i> 刷新</button>
                    <button class="btn btn-blue" onclick="app.exportLogs()"><i class="fas fa-download"></i> 导出日志</button>
                    <button class="btn btn-danger" onclick="app.clearLogs()"><i class="fas fa-trash"></i> 清空日志</button>
                </div>
                <div class="card" style="overflow:hidden;">
                    <table style="width:100%; border-collapse:collapse;">
                        <thead>
                            <tr style="background:#161b22; font-size:12px; color:#8b949e;">
                                <th style="padding:10px 12px; text-align:left;">时间</th>
                                <th style="padding:10px 12px; text-align:left;">类型</th>
                                <th style="padding:10px 12px; text-align:left;">内容</th>
                                <th style="padding:10px 12px; text-align:left;">客户端</th>
                                <th style="padding:10px 12px; text-align:left;">IP</th>
                            </tr>
                        </thead>
                        <tbody>
                            ${pageData.map(log => `
                                <tr style="border-top:1px solid #21262d; font-size:12px;">
                                    <td style="padding:8px 12px; color:#8b949e; font-size:11px;">${log.created_at?.slice(0,19).replace('T',' ') || ''}</td>
                                    <td style="padding:8px 12px;"><span class="badge badge-${log.type==='login'?'green':log.type==='system'?'purple':'blue'}">${log.type}</span></td>
                                    <td style="padding:8px 12px;">${log.content || '-'}</td>
                                    <td style="padding:8px 12px; font-family:monospace; font-size:11px; color:#8b949e;">${log.client_id || '-'}</td>
                                    <td style="padding:8px 12px; font-family:monospace; font-size:11px; color:#8b949e;">${log.ip || '-'}</td>
                                </tr>
                            `).join('')}
                        </tbody>
                    </table>
                    ${total === 0 ? `
                        <div style="text-align:center; padding:60px; color:#8b949e;">暂无日志记录</div>
                    ` : ''}
                    ${this.renderPagination(total, this.logPage, this.logPageSize, 'app.goToLogPage')}
                </div>
            `;
};

// 日志列表翻页
app.goToLogPage = function(p) {
    this.logPage = parseInt(p) || 1;
    this.render();
};

app.renderWebhookSettings = function() {
    const cfg = this.webhookConfig || { enabled: false, url: '', events: 'login,client_online,payload,task' };
    const events = (cfg.events || '').split(',');
    return `
                <div style="margin-bottom:12px; display:flex; gap:8px; border-bottom:1px solid #21262d;">
                    <div class="tab-item" onclick="app.logSubPage=''; app.render()">日志列表</div>
                    <div class="tab-item active" onclick="app.logSubPage='webhook'; app.render()">Webhook 通知</div>
                </div>
                <div class="card" style="padding:24px; max-width:600px;">
                    <h3 style="margin-bottom:20px; font-size:16px;"><i class="fas fa-bell" style="color:#d29922;"></i> Webhook 通知配置</h3>
                    <div style="margin-bottom:16px; display:flex; align-items:center; gap:10px;">
                        <label style="font-size:13px; color:#c9d1d9;">启用 Webhook</label>
                        <label class="switch">
                            <input type="checkbox" id="webhookEnabled" ${cfg.enabled?'checked':''} onchange="app.toggleWebhook(this.checked)">
                            <span class="slider"></span>
                        </label>
                    </div>
                    <div style="margin-bottom:16px;">
                        <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">Webhook URL</label>
                        <input type="text" class="input" id="webhookUrl" placeholder="https://hooks.example.com/xxx" value="${cfg.url || ''}">
                    </div>
                    <div style="margin-bottom:16px;">
                        <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">通知事件</label>
                        <div style="display:grid; grid-template-columns:1fr 1fr; gap:8px;">
                            ${['login','client','task','payload','file','system'].map(ev => `
                                <label style="display:flex; align-items:center; gap:6px; font-size:12px; color:#c9d1d9; cursor:pointer;">
                                    <input type="checkbox" value="${ev}" ${events.includes(ev)?'checked':''} class="webhook-event"> ${ev}
                                </label>
                            `).join('')}
                        </div>
                    </div>
                    <div style="display:flex; gap:10px;">
                        <button class="btn btn-primary" onclick="app.saveWebhookConfig()"><i class="fas fa-save"></i> 保存配置</button>
                        <button class="btn btn-secondary" onclick="app.testWebhook()"><i class="fas fa-paper-plane"></i> 发送测试</button>
                    </div>
                    <div style="margin-top:20px; padding:12px; background:#0d1117; border-radius:6px; border:1px solid #21262d; font-size:11px; color:#6e7681; line-height:1.8;">
                        <div><i class="fas fa-info-circle" style="color:#58a6ff;"></i> Webhook会以POST方式发送JSON数据到指定URL</div>
                        <div>数据格式: { type, content, timestamp, extra }</div>
                        <div>支持的通知类型: 登录、客户端上线、任务执行、Payload生成等</div>
                    </div>
                </div>
            `;
};

app.loadWebhookConfig = function() {
    API.get('/api/settings/webhook').then(res => {
        this.webhookConfig = res;
        this.render();
    });
};

app.saveWebhookConfig = function() {
    const events = Array.from(document.querySelectorAll('.webhook-event:checked')).map(e => e.value).join(',');
    const url = document.getElementById('webhookUrl').value;
    const enabled = document.getElementById('webhookEnabled').checked;
    
    if (enabled && !url) {
        this._notify('启用Webhook时必须填写URL', 'error');
        return;
    }
    
    const data = {
        enabled: enabled,
        url: url,
        events: events
    };
    
    const loading = this._notify('正在保存Webhook配置...', 'loading', 0);
    API.post('/api/settings/webhook', data).then(res => {
        loading.remove();
        if (res.success) {
            this._notify('Webhook配置已保存', 'success');
            this.webhookConfig = data;
        } else {
            this._notify('保存失败: ' + (res.error || '未知错误'), 'error');
        }
    }).catch(e => {
        loading.remove();
        this._notify('保存失败: ' + e.message, 'error');
    });
};

app.testWebhook = function() {
    const url = document.getElementById('webhookUrl').value;
    if (!url) {
        this._notify('请先填写Webhook URL', 'error');
        return;
    }
    const loading = this._notify('正在发送测试消息...', 'loading', 0);
    API.post('/api/settings/webhook/test', { url: url }).then(res => {
        loading.remove();
        if (res.success) {
            this._notify('测试消息发送成功！状态码: ' + res.status_code, 'success');
        } else {
            this._notify('测试失败: ' + (res.error || '未知错误'), 'error');
        }
    }).catch(e => {
        loading.remove();
        this._notify('测试失败: ' + e.message, 'error');
    });
};

app.toggleWebhook = function(enabled) {
    // 即时更新状态，但保存时一起存
};

app.exportLogs = function() {
    const loading = this._notify('正在导出日志...', 'loading', 0);
    const url = '/api/logs/export?type=' + (this.logFilterType||'') ;
    fetch(url, { headers: { 'Authorization': 'Bearer ' + API.token } })
        .then(r => {
            if (!r.ok) throw new Error('HTTP ' + r.status);
            return r.blob();
        })
        .then(blob => {
            const a = document.createElement('a');
            a.href = URL.createObjectURL(blob);
            a.download = 'c2_logs.csv';
            a.click();
            URL.revokeObjectURL(a.href);
            loading.remove();
            this._notify('日志已导出为CSV', 'success');
        })
        .catch(e => {
            loading.remove();
            this._notify('导出失败: ' + e.message, 'error');
        });
};

app.clearLogs = async function() {
    if (!confirm('确认清空' + (this.logFilterType ? '「'+this.logFilterType+'」类型' : '全部') + '日志？此操作不可恢复！')) return;
    const loading = this._notify('正在清空日志...', 'loading', 0);
    try {
        await API.post('/api/logs/clear', { type: this.logFilterType || '' });
        await this.loadLogs();
        loading.remove();
        this._notify('日志已清空', 'success');
    } catch(e) {
        loading.remove();
        this._notify('清空失败: ' + e.message, 'error');
    }
};

app.filterLogs = function(type) {
    this.logFilterType = type;
    this.loadLogs();
};
