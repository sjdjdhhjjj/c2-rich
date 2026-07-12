app.renderScreenshot = function() {
    if (!this.selectedClient) {
        return this.renderClientBar();
    }
    return `
        ${this.renderClientBar()}
        <div class="card" style="padding:16px; margin-bottom:16px;">
            <div style="display:flex; align-items:center; gap:8px; margin-bottom:14px; flex-wrap:wrap;">
                <div class="tabs" style="margin-bottom:0; border-bottom:none; flex:1;">
                    <div class="tab active" onclick="app.switchMediaTab('screenshot', this)"><i class="fas fa-eye"></i> 屏幕监控</div>
                    <div class="tab" onclick="app.switchMediaTab('record', this)"><i class="fas fa-video"></i> 录屏</div>
                    <div class="tab" onclick="app.switchMediaTab('audio', this)"><i class="fas fa-microphone"></i> 录音</div>
                    <div class="tab" onclick="app.switchMediaTab('camera', this)"><i class="fas fa-camera-retro"></i> 摄像头</div>
                </div>
            </div>

            <!-- 屏幕监控面板（持续观察屏幕 + 截图 + 录制按钮） -->
            <div id="screenshotPanel">
                <div style="display:flex; gap:12px; margin-bottom:14px; align-items:center; flex-wrap:wrap;">
                    <label style="font-size:12px; color:#8b949e;">分辨率:</label>
                    <select class="select" id="shotResolution" style="max-width:110px; font-size:12px;">
                        <option value="360p">360p</option>
                        <option value="720p" selected>720p</option>
                        <option value="1080p">1080p</option>
                    </select>
                    <label style="font-size:12px; color:#8b949e;">刷新间隔:</label>
                    <select class="select" id="shotInterval" style="max-width:100px; font-size:12px;" onchange="app._onIntervalChange()">
                        <option value="0">手动</option>
                        <option value="2">2秒</option>
                        <option value="3" selected>3秒</option>
                        <option value="5">5秒</option>
                        <option value="10">10秒</option>
                    </select>
                    <button class="btn btn-primary" onclick="app.takeScreenshot()" id="btnSingleShot"><i class="fas fa-camera"></i> 立即截图</button>
                    <button class="btn btn-blue" onclick="app.toggleLiveMonitor()" id="btnLiveMonitor"><i class="fas fa-play"></i> 开始监控</button>
                    <button class="btn btn-danger" onclick="app.recordScreen()" id="btnRecord"><i class="fas fa-video"></i> 录制</button>
                    <span id="liveStatus" style="font-size:11px; color:#6e7681;">JPEG质量随分辨率自适应(360p→55, 720p→65, 1080p→75)</span>
                </div>
                <div style="min-height:380px; display:flex; align-items:center; justify-content:center; background:#0d1117; border-radius:8px; border:1px solid #21262d; position:relative;">
                    <div id="screenshotContainer" style="width:100%;">
                        <div style="color:#8b949e; text-align:center;">
                            <i class="fas fa-eye" style="font-size:48px; margin-bottom:12px;"></i>
                            <div style="font-size:13px;">点击"开始监控"持续观察目标屏幕</div>
                            <div style="font-size:11px; margin-top:6px; color:#6e7681;">"立即截图"单次获取 | "录制"开始录屏 | 资源存储: server-go/tmp/screenshots/</div>
                        </div>
                    </div>
                    <div id="liveBadge" style="position:absolute; top:8px; left:8px; display:none; background:rgba(248,81,73,0.9); color:white; padding:2px 10px; border-radius:4px; font-size:11px; font-weight:600;">
                        <span class="status-dot online" style="background:#fff; width:6px; height:6px;"></span> LIVE
                    </div>
                </div>
            </div>

            <!-- 录屏面板 -->
            <div id="recordPanel" style="display:none;">
                <div style="display:flex; gap:12px; margin-bottom:14px; align-items:center; flex-wrap:wrap;">
                    <label style="font-size:12px; color:#8b949e;">时长(秒):</label>
                    <input type="number" class="input" id="recDuration" value="10" min="3" max="60" style="max-width:80px; font-size:12px;">
                    <button class="btn btn-blue" onclick="app.recordScreen()"><i class="fas fa-video"></i> 开始录屏</button>
                    <span style="font-size:11px; color:#6e7681;">需 ffmpeg（可选，无则回退帧截图）</span>
                </div>
                <div style="min-height:380px; display:flex; align-items:center; justify-content:center; background:#0d1117; border-radius:8px; border:1px solid #21262d;">
                    <div id="recordContainer">
                        <div style="color:#8b949e; text-align:center;">
                            <i class="fas fa-video" style="font-size:48px; margin-bottom:12px;"></i>
                            <div style="font-size:13px;">录制完成后在此播放</div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- 录音面板 -->
            <div id="audioPanel" style="display:none;">
                <div style="display:flex; gap:12px; margin-bottom:14px; align-items:center; flex-wrap:wrap;">
                    <label style="font-size:12px; color:#8b949e;">时长(秒):</label>
                    <input type="number" class="input" id="audioDuration" value="10" min="3" max="60" style="max-width:80px; font-size:12px;">
                    <button class="btn btn-secondary" style="background:linear-gradient(135deg,#bc8cff,#a371f7); color:white;" onclick="app.recordAudio()"><i class="fas fa-microphone"></i> 开始录音</button>
                    <span style="font-size:11px; color:#6e7681;">需客户端安装 pyaudio</span>
                </div>
                <div style="min-height:380px; display:flex; align-items:center; justify-content:center; background:#0d1117; border-radius:8px; border:1px solid #21262d;">
                    <div id="audioContainer">
                        <div style="color:#8b949e; text-align:center;">
                            <i class="fas fa-microphone" style="font-size:48px; margin-bottom:12px;"></i>
                            <div style="font-size:13px;">录音完成后在此播放</div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- 摄像头面板 -->
            <div id="cameraPanel" style="display:none;">
                <div style="display:flex; gap:12px; margin-bottom:14px; align-items:center; flex-wrap:wrap;">
                    <label style="font-size:12px; color:#8b949e;">录像时长(秒):</label>
                    <input type="number" class="input" id="camDuration" value="10" min="3" max="60" style="max-width:80px; font-size:12px;">
                    <button class="btn btn-primary" style="background:linear-gradient(135deg,#d29922,#b8860b);" onclick="app.cameraPhoto()"><i class="fas fa-camera"></i> 拍照</button>
                    <button class="btn btn-danger" onclick="app.cameraRecord()"><i class="fas fa-video"></i> 录像</button>
                    <span style="font-size:11px; color:#6e7681;">无需额外依赖（系统原生）</span>
                </div>
                <div style="min-height:380px; display:flex; align-items:center; justify-content:center; background:#0d1117; border-radius:8px; border:1px solid #21262d;">
                    <div id="cameraContainer">
                        <div style="color:#8b949e; text-align:center;">
                            <i class="fas fa-video" style="font-size:48px; margin-bottom:12px;"></i>
                            <div style="font-size:13px;">调用目标主机摄像头进行拍照/录像</div>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        ${this.renderMediaHistory()}
    `;
};

app.renderMediaHistory = function() {
    return `
        <div class="card" style="padding:16px; margin-top:16px;">
            <div style="display:flex; align-items:center; margin-bottom:12px;">
                <i class="fas fa-history" style="color:#58a6ff; margin-right:8px;"></i>
                <span style="font-size:14px; font-weight:600;">历史记录</span>
                <span style="font-size:11px; color:#8b949e; margin-left:10px;">已截图 / 视频 / 录音</span>
                <div style="flex:1;"></div>
                <button class="btn btn-secondary" style="padding:4px 10px; font-size:12px;" onclick="app.loadTasksSilent().then(()=>{document.getElementById('mediaHistoryList').innerHTML=app.renderMediaHistoryItems();})">
                    <i class="fas fa-sync"></i> 刷新
                </button>
                <button class="btn btn-danger" style="padding:4px 10px; font-size:12px; margin-left:8px;" onclick="app.clearMediaHistory()">
                    <i class="fas fa-trash-alt"></i> 清空全部
                </button>
            </div>
            <div id="mediaHistoryList">${this.renderMediaHistoryItems()}</div>
        </div>
    `;
};

// 清空所有媒体历史记录（截图/录屏/录音/摄像头）
app.clearMediaHistory = function() {
    if (!confirm('确认清空所有媒体历史记录？\n将删除所有截图、录屏、录音文件及任务记录（不可恢复）')) return;
    const loading = this._notify('正在清空媒体记录...', 'loading', 0);
    API.post('/api/media/clear').then(r => {
        loading.remove();
        if (r.success) {
            this._notify(`已清空 ${r.deleted_files || 0} 个媒体文件`, 'success');
            this.loadTasksSilent().then(() => {
                const list = document.getElementById('mediaHistoryList');
                if (list) list.innerHTML = this.renderMediaHistoryItems();
            });
        } else {
            this._notify('清空失败: ' + (r.error || '未知错误'), 'error');
        }
    }).catch(e => {
        loading.remove();
        this._notify('清空失败: ' + (e.message || e), 'error');
    });
};

app.renderMediaHistoryItems = function() {
    const MEDIA_TYPES = {
        'screenshot': { label: '截图', icon: 'fa-camera', color: '#3fb950', subdir: 'screenshots' },
        'record_screen': { label: '录屏', icon: 'fa-video', color: '#58a6ff', subdir: 'recordings' },
        'record_audio': { label: '录音', icon: 'fa-microphone', color: '#bc8cff', subdir: 'audio' },
        'camera_photo': { label: '摄像头', icon: 'fa-camera-retro', color: '#d29922', subdir: 'screenshots' },
        'camera_record': { label: '摄像录像', icon: 'fa-video', color: '#d29922', subdir: 'recordings' }
    };
    const items = (this.tasks || [])
        .filter(t => t.task_type in MEDIA_TYPES && t.status === 'completed' && t.result)
        .sort((a, b) => (b.id || 0) - (a.id || 0));

    if (items.length === 0) {
        return `<div style="text-align:center; padding:30px; color:#6e7681; font-size:13px;">
            <i class="fas fa-inbox" style="font-size:32px; margin-bottom:10px; display:block;"></i>
            暂无历史记录
        </div>`;
    }

    return `<div style="display:grid; grid-template-columns:repeat(auto-fill, minmax(220px, 1fr)); gap:12px;">` + items.map(t => {
        const meta = MEDIA_TYPES[t.task_type];
        let resourceUrl = null, fileSize = 0, parseErr = false;
        try {
            const parsed = JSON.parse(t.result);
            if (parsed.type === 'file' && parsed.path) {
                resourceUrl = parsed.path;
                fileSize = parsed.size || 0;
            } else if (parsed.data && parsed.data.length > 100) {
                // file_download 格式兜底
            }
        } catch(e) {
            if (t.result.length < 100) parseErr = true;
        }

        const client = (this.clients || []).find(c => c.client_id === t.client_id);
        const host = client ? client.hostname : (t.client_id || '').substring(0, 8);
        const ts = t.completed_at || t.created_at || '';
        const sizeStr = fileSize > 0 ? (fileSize / 1024).toFixed(1) + ' KB' : '';

        // 缩略图：截图显示img，录屏显示视频首帧poster，录音显示图标
        let thumb = '';
        if (parseErr) {
            thumb = `<div style="height:120px; display:flex; align-items:center; justify-content:center; color:#f85149; font-size:11px;">解析失败</div>`;
        } else if (t.result.startsWith('[ERROR]')) {
            thumb = `<div style="height:120px; display:flex; align-items:center; justify-content:center; color:#f85149; font-size:11px; padding:8px; text-align:center;">${t.result.substring(0, 80)}</div>`;
        } else if (t.task_type === 'screenshot' && resourceUrl) {
            thumb = `<img src="${resourceUrl}?token=${API.token}" style="width:100%; height:120px; object-fit:cover; border-radius:6px; cursor:pointer;" onclick="app.previewMedia('${resourceUrl}','screenshot')">`;
        } else if (t.task_type === 'record_screen' && resourceUrl) {
            thumb = `<div style="position:relative; height:120px; background:#000; border-radius:6px; cursor:pointer; display:flex; align-items:center; justify-content:center;" onclick="app.previewMedia('${resourceUrl}','record_screen')">
                <i class="fas fa-play-circle" style="font-size:36px; color:#58a6ff;"></i>
            </div>`;
        } else if (t.task_type === 'record_audio' && resourceUrl) {
            thumb = `<div style="height:120px; background:linear-gradient(135deg,#1a1f2e,#0d1117); border-radius:6px; cursor:pointer; display:flex; align-items:center; justify-content:center;" onclick="app.previewMedia('${resourceUrl}','record_audio')">
                <i class="fas fa-microphone" style="font-size:36px; color:#bc8cff;"></i>
            </div>`;
        } else if (t.task_type === 'camera_photo' && resourceUrl) {
            thumb = `<img src="${resourceUrl}?token=${API.token}" style="width:100%; height:120px; object-fit:cover; border-radius:6px; cursor:pointer;" onclick="app.previewMedia('${resourceUrl}','screenshot')">`;
        } else if (t.task_type === 'camera_record' && resourceUrl) {
            thumb = `<div style="position:relative; height:120px; background:#000; border-radius:6px; cursor:pointer; display:flex; align-items:center; justify-content:center;" onclick="app.previewMedia('${resourceUrl}','record_screen')">
                <i class="fas fa-play-circle" style="font-size:36px; color:#d29922;"></i>
            </div>`;
        } else if (t.result.length > 100 && (t.task_type === 'screenshot' || t.task_type === 'camera_photo')) {
            // 兜底：直接base64内联
            thumb = `<img src="data:image/jpeg;base64,${t.result.substring(0, 2000)}" style="width:100%; height:120px; object-fit:cover; border-radius:6px; opacity:0.5;">`;
        } else {
            thumb = `<div style="height:120px; display:flex; align-items:center; justify-content:center; color:#6e7681; font-size:11px;">无预览</div>`;
        }

        return `
            <div class="card" style="padding:8px; overflow:hidden; position:relative;">
                <button title="删除" onclick="app.deleteMediaTask(${t.id})" style="position:absolute; top:6px; right:6px; z-index:5; background:rgba(248,81,73,0.85); color:#fff; border:none; border-radius:50%; width:24px; height:24px; cursor:pointer; display:flex; align-items:center; justify-content:center; font-size:11px;"><i class="fas fa-times"></i></button>
                ${thumb}
                <div style="padding:8px 4px 4px;">
                    <div style="display:flex; align-items:center; gap:6px; margin-bottom:4px;">
                        <span class="badge" style="background:${meta.color}22; color:${meta.color};"><i class="fas ${meta.icon}"></i> ${meta.label}</span>
                        <span style="font-size:11px; color:#8b949e; margin-left:auto;">${sizeStr}</span>
                    </div>
                    <div style="font-size:12px; color:#c9d1d9; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;"><i class="fas fa-desktop" style="color:#8b949e;"></i> ${host}</div>
                    <div style="font-size:10px; color:#6e7681; margin-top:2px;">${ts}</div>
                </div>
            </div>
        `;
    }).join('') + `</div>`;
};

app.deleteMediaTask = async function(taskId) {
    if (!confirm('确认删除该记录？资源文件将一并删除。')) return;
    const loading = this._notify('正在删除...', 'loading', 0);
    try {
        await API.post('/api/task/' + taskId + '/delete', {});
        await this.loadTasksSilent();
        const hist = document.getElementById('mediaHistoryList');
        if (hist) hist.innerHTML = this.renderMediaHistoryItems();
        loading.remove();
        this._notify('记录已删除', 'success');
    } catch(e) {
        loading.remove();
        this._notify('删除失败: ' + e.message, 'error');
    }
};

app.previewMedia = function(url, type) {
    // 移除已有预览框
    const old = document.getElementById('mediaPreviewOverlay');
    if (old) old.remove();

    const overlay = document.createElement('div');
    overlay.id = 'mediaPreviewOverlay';
    overlay.className = 'modal-overlay';
    overlay.style.alignItems = 'flex-start';

    let mediaHtml = '';
    let toolbar = '';
    if (type === 'screenshot') {
        mediaHtml = `<img id="previewImg" src="${url}?token=${API.token}&t=${Date.now()}" style="max-width:100%; max-height:72vh; border-radius:8px; transition:transform 0.1s; cursor:grab; user-select:none;">`;
        toolbar = `
            <button class="btn btn-secondary" style="padding:4px 10px; font-size:12px;" onclick="app.zoomPreview(1.2)"><i class="fas fa-search-plus"></i></button>
            <button class="btn btn-secondary" style="padding:4px 10px; font-size:12px;" onclick="app.zoomPreview(0.8)"><i class="fas fa-search-minus"></i></button>
            <button class="btn btn-secondary" style="padding:4px 10px; font-size:12px;" onclick="app.resetPreviewZoom()"><i class="fas fa-expand"></i> 适配</button>
            <a class="btn btn-secondary" style="padding:4px 10px; font-size:12px; text-decoration:none;" href="${url}?token=${API.token}&download=1" download><i class="fas fa-download"></i></a>
        `;
    } else if (type === 'record_screen') {
        mediaHtml = `<video controls autoplay style="max-width:100%; max-height:72vh; border-radius:8px;"><source src="${url}?token=${API.token}&t=${Date.now()}" type="video/x-msvideo"></video>`;
        toolbar = `<a class="btn btn-secondary" style="padding:4px 10px; font-size:12px; text-decoration:none;" href="${url}?token=${API.token}&download=1" download><i class="fas fa-download"></i></a>`;
    } else if (type === 'record_audio') {
        mediaHtml = `<div style="padding:40px 20px;"><i class="fas fa-microphone" style="font-size:64px; color:#bc8cff; margin-bottom:16px;"></i><audio controls autoplay style="width:100%;"><source src="${url}?token=${API.token}&t=${Date.now()}" type="audio/wav"></audio></div>`;
        toolbar = `<a class="btn btn-secondary" style="padding:4px 10px; font-size:12px; text-decoration:none;" href="${url}?token=${API.token}&download=1" download><i class="fas fa-download"></i></a>`;
    }

    overlay.innerHTML = `
        <div class="modal" style="text-align:center; max-width:95vw; width:95vw; padding:16px;">
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px; gap:8px;">
                <span style="font-size:14px; font-weight:600;"><i class="fas fa-eye"></i> 预览 ${type==='screenshot'?'<span id="zoomLabel" style="font-size:11px; color:#8b949e; margin-left:8px;">100%</span>':''}</span>
                <div style="display:flex; gap:6px; align-items:center;">
                    ${toolbar}
                    <button class="btn btn-secondary" style="padding:4px 10px;" onclick="this.closest('.modal-overlay').remove()"><i class="fas fa-times"></i> 关闭</button>
                </div>
            </div>
            <div id="previewStage" style="overflow:auto; max-height:78vh; display:flex; align-items:center; justify-content:center; border-radius:8px; background:#0d1117;">
                ${mediaHtml}
            </div>
        </div>
    `;
    // 点击空白处关闭
    overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove(); });
    document.body.appendChild(overlay);

    // 图片缩放/拖动逻辑
    if (type === 'screenshot') {
        const img = document.getElementById('previewImg');
        const stage = document.getElementById('previewStage');
        let scale = 1, tx = 0, ty = 0;
        window._previewState = { scale, tx, ty, img, stage };

        const apply = () => {
            img.style.transform = `translate(${window._previewState.tx}px, ${window._previewState.ty}px) scale(${window._previewState.scale})`;
            const lbl = document.getElementById('zoomLabel');
            if (lbl) lbl.textContent = Math.round(window._previewState.scale * 100) + '%';
        };

        // 滚轮缩放
        stage.addEventListener('wheel', (e) => {
            e.preventDefault();
            const delta = e.deltaY < 0 ? 1.1 : 0.9;
            window._previewState.scale = Math.max(0.2, Math.min(8, window._previewState.scale * delta));
            apply();
        }, { passive: false });

        // 拖动
        let dragging = false, sx = 0, sy = 0, startTx = 0, startTy = 0;
        img.addEventListener('mousedown', (e) => {
            dragging = true;
            sx = e.clientX; sy = e.clientY;
            startTx = window._previewState.tx; startTy = window._previewState.ty;
            img.style.cursor = 'grabbing';
            e.preventDefault();
        });
        document.addEventListener('mousemove', (e) => {
            if (!dragging) return;
            window._previewState.tx = startTx + (e.clientX - sx);
            window._previewState.ty = startTy + (e.clientY - sy);
            apply();
        });
        document.addEventListener('mouseup', () => {
            if (dragging) { dragging = false; img.style.cursor = 'grab'; }
        });

        // 双击重置
        img.addEventListener('dblclick', () => app.resetPreviewZoom());
    }
};

app.zoomPreview = function(factor) {
    if (!window._previewState) return;
    window._previewState.scale = Math.max(0.2, Math.min(8, window._previewState.scale * factor));
    const img = window._previewState.img;
    img.style.transform = `translate(${window._previewState.tx}px, ${window._previewState.ty}px) scale(${window._previewState.scale})`;
    const lbl = document.getElementById('zoomLabel');
    if (lbl) lbl.textContent = Math.round(window._previewState.scale * 100) + '%';
};

app.resetPreviewZoom = function() {
    if (!window._previewState) return;
    window._previewState.scale = 1;
    window._previewState.tx = 0;
    window._previewState.ty = 0;
    window._previewState.img.style.transform = 'translate(0,0) scale(1)';
    const lbl = document.getElementById('zoomLabel');
    if (lbl) lbl.textContent = '100%';
};

app.switchMediaTab = function(tab, el) {
    // 只切换当前卡片内的 .tab（避免影响其他页面的 .tab 元素）
    const card = el ? el.closest('.card') : null;
    if (card) {
        card.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    } else {
        document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    }
    if (el) el.classList.add('active');
    const sp = document.getElementById('screenshotPanel');
    const rp = document.getElementById('recordPanel');
    const ap = document.getElementById('audioPanel');
    const cp = document.getElementById('cameraPanel');
    if (sp) sp.style.display = tab === 'screenshot' ? '' : 'none';
    if (rp) rp.style.display = tab === 'record' ? '' : 'none';
    if (ap) ap.style.display = tab === 'audio' ? '' : 'none';
    if (cp) cp.style.display = tab === 'camera' ? '' : 'none';
};

app.cameraPhoto = async function() {
    if (!this.selectedClient) return;
    const c0 = document.getElementById('cameraContainer');
    if (c0) c0.innerHTML = `<div style="color:#8b949e;"><i class="fas fa-spinner fa-spin" style="font-size:48px;"></i><div style="margin-top:12px;">正在调用摄像头拍照...</div></div>`;
    const taskIds = await this.sendTask('camera_photo', {});
    if (!taskIds || taskIds.length === 0) return;
    const taskId = taskIds[0];
    let attempts = 0;
    const maxAttempts = 10;
    const checkResult = () => {
        attempts++;
        API.get(`/api/task/${taskId}`).then(task => {
            const c = document.getElementById('cameraContainer');
            if (!c) return;
            if (task && task.status === 'completed' && task.result) {
                if (task.result.startsWith('[ERROR]')) {
                    c.innerHTML = `<div style="color:#f85149;"><i class="fas fa-exclamation-triangle" style="font-size:36px;"></i><div style="margin-top:12px;">${task.result}</div></div>`;
                } else {
                    let resourceUrl = null, fileSize = 0;
                    try {
                        const parsed = JSON.parse(task.result);
                        if (parsed.type === 'file' && parsed.path) { resourceUrl = parsed.path; fileSize = parsed.size || 0; }
                    } catch(e) {}
                    if (resourceUrl) {
                        c.innerHTML = `<div><img src="${resourceUrl}?token=${API.token}&t=${Date.now()}" style="max-width:100%; border-radius:8px; box-shadow:0 4px 20px rgba(0,0,0,0.5);"><div style="margin-top:12px; font-size:11px; color:#8b949e;"><i class="fas fa-camera"></i> 摄像头拍照 | ${(fileSize/1024).toFixed(1)} KB | ${new Date().toLocaleString('zh-CN')}</div></div>`;
                    } else if (task.result.length > 100) {
                        c.innerHTML = `<img src="data:image/jpeg;base64,${task.result}" style="max-width:100%; border-radius:8px;">`;
                    }
                }
            } else if (attempts < maxAttempts) {
                setTimeout(checkResult, 2000);
            } else {
                c.innerHTML = '<div style="color:#f85149;"><i class="fas fa-clock" style="font-size:36px;"></i><div style="margin-top:12px;">超时，请确认客户端在线</div></div>';
            }
        });
    };
    setTimeout(checkResult, 2000);
};

app.cameraRecord = async function() {
    if (!this.selectedClient) return;
    const duration = parseInt(document.getElementById('camDuration')?.value || 10);
    const c0 = document.getElementById('cameraContainer');
    if (c0) c0.innerHTML = `<div style="color:#8b949e;"><i class="fas fa-spinner fa-spin" style="font-size:48px;"></i><div style="margin-top:12px;">正在录像 (${duration}秒)...</div></div>`;
    const taskIds = await this.sendTask('camera_record', { duration: duration });
    if (!taskIds || taskIds.length === 0) return;
    const taskId = taskIds[0];
    let attempts = 0;
    const maxAttempts = Math.ceil(duration / 2) + 8;
    const checkResult = () => {
        attempts++;
        API.get(`/api/task/${taskId}`).then(task => {
            const c = document.getElementById('cameraContainer');
            if (!c) return;
            if (task && task.status === 'completed' && task.result) {
                if (task.result.startsWith('[ERROR]')) {
                    c.innerHTML = `<div style="color:#f85149;">${task.result}</div>`;
                } else {
                    try {
                        const parsed = JSON.parse(task.result);
                        if (parsed.type === 'file' && parsed.path) {
                            c.innerHTML = `<video controls style="max-width:100%; border-radius:8px; box-shadow:0 4px 20px rgba(0,0,0,0.5);"><source src="${parsed.path}?token=${API.token}&t=${Date.now()}" type="video/x-msvideo"></video><div style="margin-top:12px; font-size:11px; color:#8b949e;">录像大小: ${(parsed.size/1024).toFixed(1)} KB | server-go/tmp/recordings/</div>`;
                        }
                    } catch(e) { c.innerHTML = `<div style="color:#f85149;">解析失败</div>`; }
                }
            } else if (attempts < maxAttempts) {
                setTimeout(checkResult, 2000);
            } else {
                c.innerHTML = '<div style="color:#f85149;">录像超时，请确认客户端在线</div>';
            }
        });
    };
    setTimeout(checkResult, (duration + 3) * 1000);
};

// 实时屏幕监控状态
app._liveMonitorTimer = null;
app._liveMonitorRunning = false;

// 开始/停止实时监控
app.toggleLiveMonitor = function() {
    if (this._liveMonitorRunning) {
        this.stopLiveMonitor();
    } else {
        this.startLiveMonitor();
    }
};

// 开始实时监控：按设定间隔自动截图刷新
app.startLiveMonitor = function() {
    const intervalSel = document.getElementById('shotInterval');
    let interval = parseInt(intervalSel ? intervalSel.value : 3);
    if (!interval || interval < 1) {
        this._notify('请选择刷新间隔（手动模式无法持续监控）', 'error');
        return;
    }
    this._liveMonitorRunning = true;
    const btn = document.getElementById('btnLiveMonitor');
    if (btn) btn.innerHTML = '<i class="fas fa-stop"></i> 停止监控';
    const badge = document.getElementById('liveBadge');
    if (badge) badge.style.display = 'block';
    const status = document.getElementById('liveStatus');
    if (status) status.innerHTML = '<span style="color:#3fb950;">● 监控中</span> 每 ' + interval + ' 秒刷新';

    // 立即截一次，然后按间隔刷新
    this._takeScreenshotSilent();
    this._liveMonitorTimer = setInterval(() => this._takeScreenshotSilent(), interval * 1000);
};

// 停止实时监控
app.stopLiveMonitor = function() {
    this._liveMonitorRunning = false;
    if (this._liveMonitorTimer) {
        clearInterval(this._liveMonitorTimer);
        this._liveMonitorTimer = null;
    }
    const btn = document.getElementById('btnLiveMonitor');
    if (btn) btn.innerHTML = '<i class="fas fa-play"></i> 开始监控';
    const badge = document.getElementById('liveBadge');
    if (badge) badge.style.display = 'none';
    const status = document.getElementById('liveStatus');
    if (status) status.textContent = 'JPEG质量随分辨率自适应(360p→55, 720p→65, 1080p→75)';
};

// 间隔改变时，如果正在监控则重启定时器
app._onIntervalChange = function() {
    if (this._liveMonitorRunning) {
        this.stopLiveMonitor();
        this.startLiveMonitor();
    }
};

// 静默截图（不显示loading spinner，直接刷新图片，避免监控时画面闪烁）
app._takeScreenshotSilent = async function() {
    if (!this.selectedClient) return;
    const resolution = document.getElementById('shotResolution')?.value || '720p';
    try {
        const taskIds = await this.sendTask('screenshot', { resolution: resolution });
        if (!taskIds || taskIds.length === 0) return;
        const taskId = taskIds[0];
        let attempts = 0;
        const check = () => {
            attempts++;
            API.get(`/api/task/${taskId}`).then(task => {
                if (!task) return;
                if (task.status === 'completed' && task.result) {
                    if (task.result.startsWith('[ERROR]')) return;
                    let resourceUrl = null, fileSize = 0;
                    try {
                        const parsed = JSON.parse(task.result);
                        if (parsed.type === 'file' && parsed.path) {
                            resourceUrl = parsed.path;
                            fileSize = parsed.size || 0;
                        }
                    } catch(e) {}
                    const c = document.getElementById('screenshotContainer');
                    if (!c) return;
                    if (resourceUrl) {
                        // 直接更新img src，不重建DOM避免闪烁
                        const existingImg = c.querySelector('img.live-monitor-img');
                        if (existingImg) {
                            existingImg.src = `${resourceUrl}?token=${API.token}&t=${Date.now()}`;
                            // 更新尺寸和时间戳
                            const sizeEl = c.querySelector('.live-size');
                            const tsEl = c.querySelector('.live-ts');
                            if (sizeEl) sizeEl.textContent = (fileSize/1024).toFixed(1) + ' KB';
                            if (tsEl) tsEl.textContent = new Date().toLocaleTimeString('zh-CN');
                        } else {
                            c.innerHTML = `
                                <div style="cursor:zoom-in;" onclick="app.previewMedia('${resourceUrl}','screenshot')">
                                    <img class="live-monitor-img" src="${resourceUrl}?token=${API.token}&t=${Date.now()}" style="max-width:100%; max-height:500px; object-fit:contain; border-radius:6px;">
                                    <div style="margin-top:8px; font-size:11px; color:#8b949e; text-align:center;">
                                        <i class="fas fa-search-plus"></i> 点击放大 | ${resolution} | <span class="live-size">${(fileSize/1024).toFixed(1)} KB</span> | <span class="live-ts">${new Date().toLocaleTimeString('zh-CN')}</span>
                                    </div>
                                </div>
                            `;
                        }
                    }
                } else if (task.status === 'failed') {
                    // 失败时停止监控
                    if (this._liveMonitorRunning) this.stopLiveMonitor();
                    this._notify('截图失败: ' + (task.result || ''), 'error');
                } else if (attempts < 10) {
                    setTimeout(check, 1000);
                }
            }).catch(() => {});
        };
        setTimeout(check, 1500);
    } catch(e) {}
};

app.takeScreenshot = async function() {
    if (!this.selectedClient) return;
    const resolution = document.getElementById('shotResolution')?.value || '720p';
    const showLoading = () => {
        const c = document.getElementById('screenshotContainer');
        if (c) c.innerHTML = `<div style="color:#8b949e;"><i class="fas fa-spinner fa-spin" style="font-size:48px;"></i><div style="margin-top:12px;">正在截取屏幕 (${resolution})...</div></div>`;
    };
    showLoading();
    const taskIds = await this.sendTask('screenshot', { resolution: resolution });
    if (!taskIds || taskIds.length === 0) return;
    const taskId = taskIds[0];

    let attempts = 0;
    const maxAttempts = 10;
    const checkResult = () => {
        attempts++;
        API.get(`/api/task/${taskId}`).then(task => {
            const c = document.getElementById('screenshotContainer');
            if (!c) return;

            if (task && task.status === 'completed' && task.result) {
                if (task.result.startsWith('[ERROR]')) {
                    c.innerHTML = `<div style="color:#f85149;"><i class="fas fa-exclamation-triangle" style="font-size:36px;"></i><div style="margin-top:12px;">${task.result}</div></div>`;
                } else {
                    let resourceUrl = null;
                    let fileSize = 0;
                    try {
                        const parsed = JSON.parse(task.result);
                        if (parsed.type === 'file' && parsed.path) {
                            resourceUrl = parsed.path;
                            fileSize = parsed.size || 0;
                        }
                    } catch(e) {
                        if (task.result.length > 100) {
                            resourceUrl = null;
                        }
                    }

                    if (resourceUrl) {
                        c.innerHTML = `
                            <div style="cursor:zoom-in;" onclick="app.previewMedia('${resourceUrl}','screenshot')">
                                <img src="${resourceUrl}?token=${API.token}&t=${Date.now()}" style="max-width:100%; max-height:500px; object-fit:contain; border-radius:6px;">
                                <div style="margin-top:8px; font-size:11px; color:#8b949e; text-align:center;">
                                    <i class="fas fa-search-plus"></i> 点击放大 | ${resolution} | ${(fileSize / 1024).toFixed(1)} KB
                                </div>
                            </div>
                        `;
                    } else if (task.result.length > 100) {
                        c.innerHTML = `<img src="data:image/jpeg;base64,${task.result}" style="max-width:100%; border-radius:8px;">`;
                    } else {
                        c.innerHTML = `<div style="color:#f85149;">截图数据异常</div>`;
                    }
                }
            } else if (attempts < maxAttempts) {
                setTimeout(checkResult, 2000);
            } else {
                c.innerHTML = '<div style="color:#f85149;"><i class="fas fa-clock" style="font-size:36px;"></i><div style="margin-top:12px;">截图超时，请确认客户端在线</div></div>';
            }
        });
    };
    setTimeout(checkResult, 2000);
};

app.recordScreen = async function() {
    if (!this.selectedClient) return;
    const duration = parseInt(document.getElementById('recDuration')?.value || 10);
    const c0 = document.getElementById('recordContainer');
    if (c0) c0.innerHTML = `<div style="color:#8b949e;"><i class="fas fa-spinner fa-spin" style="font-size:48px;"></i><div style="margin-top:12px;">正在录制屏幕 (${duration}秒)...</div></div>`;
    const taskIds = await this.sendTask('record_screen', { duration: duration });
    if (!taskIds || taskIds.length === 0) return;
    const taskId = taskIds[0];

    let attempts = 0;
    const maxAttempts = Math.ceil(duration / 2) + 8;
    const checkResult = () => {
        attempts++;
        API.get(`/api/task/${taskId}`).then(task => {
            const c = document.getElementById('recordContainer');
            if (!c) return;
            if (task && task.status === 'completed' && task.result) {
                if (task.result.startsWith('[ERROR]')) {
                    c.innerHTML = `<div style="color:#f85149;">${task.result}</div>`;
                } else {
                    try {
                        const parsed = JSON.parse(task.result);
                        if (parsed.type === 'file' && parsed.path) {
                            c.innerHTML = `
                                <video controls style="max-width:100%; border-radius:8px; box-shadow:0 4px 20px rgba(0,0,0,0.5);">
                                    <source src="${parsed.path}?token=${API.token}&t=${Date.now()}" type="video/x-msvideo">
                                </video>
                                <div style="margin-top:12px; font-size:11px; color:#8b949e;">录像大小: ${(parsed.size/1024).toFixed(1)} KB | server-go/tmp/recordings/</div>
                            `;
                        }
                    } catch(e) {
                        c.innerHTML = `<div style="color:#f85149;">解析失败: ${task.result.substring(0, 100)}</div>`;
                    }
                }
            } else if (attempts < maxAttempts) {
                setTimeout(checkResult, 2000);
            } else {
                c.innerHTML = '<div style="color:#f85149;">录制超时，请确认客户端在线</div>';
            }
        });
    };
    setTimeout(checkResult, (duration + 3) * 1000);
};

app.recordAudio = async function() {
    if (!this.selectedClient) return;
    const duration = parseInt(document.getElementById('audioDuration')?.value || 10);
    const c0 = document.getElementById('audioContainer');
    if (c0) c0.innerHTML = `<div style="color:#8b949e;"><i class="fas fa-spinner fa-spin" style="font-size:48px;"></i><div style="margin-top:12px;">正在录音 (${duration}秒)...</div></div>`;
    const taskIds = await this.sendTask('record_audio', { duration: duration });
    if (!taskIds || taskIds.length === 0) return;
    const taskId = taskIds[0];

    let attempts = 0;
    const maxAttempts = Math.ceil(duration / 2) + 8;
    const checkResult = () => {
        attempts++;
        API.get(`/api/task/${taskId}`).then(task => {
            const c = document.getElementById('audioContainer');
            if (!c) return;
            if (task && task.status === 'completed' && task.result) {
                if (task.result.startsWith('[ERROR]')) {
                    c.innerHTML = `<div style="color:#f85149;">${task.result}</div>`;
                } else {
                    try {
                        const parsed = JSON.parse(task.result);
                        if (parsed.type === 'file' && parsed.path) {
                            c.innerHTML = `
                                <audio controls style="width:100%; margin-top:12px;">
                                    <source src="${parsed.path}?token=${API.token}&t=${Date.now()}" type="audio/wav">
                                </audio>
                                <div style="margin-top:12px; font-size:11px; color:#8b949e;">音频大小: ${(parsed.size/1024).toFixed(1)} KB | server-go/tmp/audio/</div>
                            `;
                        }
                    } catch(e) {
                        c.innerHTML = `<div style="color:#f85149;">解析失败</div>`;
                    }
                }
            } else if (attempts < maxAttempts) {
                setTimeout(checkResult, 2000);
            } else {
                c.innerHTML = '<div style="color:#f85149;">录音超时，需安装 pyaudio</div>';
            }
        });
    };
    setTimeout(checkResult, (duration + 3) * 1000);
};
