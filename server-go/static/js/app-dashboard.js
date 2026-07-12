// app-dashboard.js - App 仪表盘相关方法（renderDashboard, initCharts）

app.renderDashboard = function() {
    if (!this.stats) return '<div style="text-align:center; padding:60px; color:#8b949e;">加载中...</div>';

    const s = this.stats;
    return `
                <div style="display:grid; grid-template-columns:repeat(4, 1fr); gap:10px; margin-bottom:12px;">
                    <div class="card" style="padding:10px 14px;">
                        <div style="display:flex; align-items:center; justify-content:space-between;">
                            <div>
                                <div style="color:#8b949e; font-size:11px;">总主机数</div>
                                <div style="font-size:20px; font-weight:700; margin-top:2px;">${s.total_clients}</div>
                            </div>
                            <div style="font-size:22px; color:#58a6ff;"><i class="fas fa-desktop"></i></div>
                        </div>
                        <div style="margin-top:4px; font-size:10px; color:#3fb950;">
                            <i class="fas fa-arrow-up"></i> 今日 ${s.today_tasks} 任务
                        </div>
                    </div>
                    <div class="card" style="padding:10px 14px;">
                        <div style="display:flex; align-items:center; justify-content:space-between;">
                            <div>
                                <div style="color:#8b949e; font-size:11px;">在线主机</div>
                                <div style="font-size:20px; font-weight:700; margin-top:2px; color:#3fb950;" class="glow-green">${s.online_clients}</div>
                            </div>
                            <div style="font-size:22px; color:#3fb950;"><i class="fas fa-circle-check"></i></div>
                        </div>
                        <div class="progress-bar" style="margin-top:4px; height:3px;">
                            <div class="progress-fill" style="width:${s.total_clients ? (s.online_clients/s.total_clients*100) : 0}%"></div>
                        </div>
                    </div>
                    <div class="card" style="padding:10px 14px;">
                        <div style="display:flex; align-items:center; justify-content:space-between;">
                            <div>
                                <div style="color:#8b949e; font-size:11px;">离线主机</div>
                                <div style="font-size:20px; font-weight:700; margin-top:2px; color:#f85149;">${s.offline_clients}</div>
                            </div>
                            <div style="font-size:22px; color:#f85149;"><i class="fas fa-circle-xmark"></i></div>
                        </div>
                        <div style="margin-top:4px; font-size:10px; color:#8b949e;">
                            离线率: ${s.total_clients ? ((s.offline_clients/s.total_clients)*100).toFixed(1) : 0}%
                        </div>
                    </div>
                    <div class="card" style="padding:10px 14px;">
                        <div style="display:flex; align-items:center; justify-content:space-between;">
                            <div>
                                <div style="color:#8b949e; font-size:11px;">总任务数</div>
                                <div style="font-size:20px; font-weight:700; margin-top:2px; color:#bc8cff;">${s.total_tasks}</div>
                            </div>
                            <div style="font-size:22px; color:#bc8cff;"><i class="fas fa-list-check"></i></div>
                        </div>
                        <div style="margin-top:4px; font-size:10px; color:#8b949e;">
                            今日执行: ${s.today_tasks} 次
                        </div>
                    </div>
                </div>

                <div style="display:grid; grid-template-columns:2fr 1fr; gap:10px; margin-bottom:12px;">
                    <div class="card" style="padding:10px 14px;">
                        <h3 style="margin-bottom:6px; font-size:13px;"><i class="fas fa-chart-line" style="color:#58a6ff;"></i> 近7天上线趋势</h3>
                        <div style="position:relative; height:140px;"><canvas id="weekChart"></canvas></div>
                    </div>
                    <div class="card" style="padding:10px 14px;">
                        <h3 style="margin-bottom:6px; font-size:13px;"><i class="fab fa-windows" style="color:#58a6ff;"></i> 操作系统分布</h3>
                        <div style="position:relative; height:140px;"><canvas id="osChart"></canvas></div>
                    </div>
                </div>

                <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:10px; margin-bottom:12px;">
                    <div class="card" style="padding:10px 14px;">
                        <h3 style="margin-bottom:6px; font-size:13px;"><i class="fas fa-layer-group" style="color:#3fb950;"></i> 分组分布</h3>
                        <div style="position:relative; height:120px;"><canvas id="groupChart"></canvas></div>
                    </div>
                    <div class="card" style="padding:10px 14px;">
                        <h3 style="margin-bottom:6px; font-size:13px;"><i class="fas fa-network-wired" style="color:#58a6ff;"></i> 网段分布</h3>
                        <div style="position:relative; height:120px;"><canvas id="subnetChart"></canvas></div>
                    </div>
                    <div class="card" style="padding:10px 14px;">
                        <h3 style="margin-bottom:6px; font-size:13px;"><i class="fas fa-shield-halved" style="color:#d29922;"></i> 权限等级</h3>
                        <div style="position:relative; height:120px;"><canvas id="privilegeChart"></canvas></div>
                    </div>
                </div>

                <div style="display:grid; grid-template-columns:2fr 1fr 1fr; gap:10px;">
                    <div class="card" style="padding:10px 14px;">
                        <h3 style="margin-bottom:6px; font-size:13px;">
                            <i class="fas fa-chart-area" style="color:#bc8cff;"></i>
                            流量统计
                            <span style="float:right; font-size:11px; color:#8b949e; font-weight:normal;">
                                总计: <span style="color:#bc8cff;">${(s.total_traffic_kb / 1024).toFixed(2)} MB</span>
                            </span>
                        </h3>
                        <div style="position:relative; height:120px;"><canvas id="trafficChart"></canvas></div>
                    </div>
                    <div class="card" style="padding:10px 14px;">
                        <h3 style="margin-bottom:6px; font-size:13px;"><i class="fas fa-globe" style="color:#f85149;"></i> 地域分布</h3>
                        <div>
                            ${Object.entries(s.country_stats).slice(0, 5).map(([k,v]) => `
                                <div style="display:flex; justify-content:space-between; padding:3px 0; border-bottom:1px solid #21262d; font-size:11px;">
                                    <span>${k}</span>
                                    <span style="color:#3fb950;">${v} 台</span>
                                </div>
                            `).join('')}
                        </div>
                    </div>
                    <div class="card" style="padding:10px 14px;">
                        <h3 style="margin-bottom:6px; font-size:13px;"><i class="fas fa-clock" style="color:#d29922;"></i> 最近日志</h3>
                        <div style="font-size:11px;">
                            ${this.logs.slice(0, 6).map(log => `
                                <div style="padding:3px 0; border-bottom:1px solid #21262d; display:flex; gap:6px; overflow:hidden;">
                                    <span class="badge badge-blue" style="font-size:9px; flex-shrink:0;">${log.type}</span>
                                    <span style="color:#8b949e; flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;">${log.content || ''}</span>
                                </div>
                            `).join('')}
                        </div>
                    </div>
                </div>
            `;
};

app.initCharts = function() {
    if (this.page !== 'dashboard' || !this.stats) return;

    const s = this.stats;
    // 通用图表选项：禁用宽高比锁定，让 canvas 严格按 height 属性渲染
    const baseOpts = {
        responsive: true,
        maintainAspectRatio: false,
    };

    const weekCtx = document.getElementById('weekChart');
    if (weekCtx) {
        new Chart(weekCtx, {
            type: 'line',
            data: {
                labels: s.week_data.map(d => d.date.slice(5)),
                datasets: [{
                    label: '新增主机',
                    data: s.week_data.map(d => d.count),
                    borderColor: '#3fb950',
                    backgroundColor: 'rgba(63,185,80,0.1)',
                    fill: true,
                    tension: 0.4,
                    pointBackgroundColor: '#3fb950'
                }]
            },
            options: {
                ...baseOpts,
                plugins: { legend: { display: false } },
                scales: {
                    x: { grid: { color: '#21262d' }, ticks: { color: '#8b949e', font: { size: 10 } } },
                    y: { grid: { color: '#21262d' }, ticks: { color: '#8b949e', font: { size: 10 } } }
                }
            }
        });
    }

    const osCtx = document.getElementById('osChart');
    if (osCtx) {
        new Chart(osCtx, {
            type: 'doughnut',
            data: {
                labels: Object.keys(s.os_stats),
                datasets: [{
                    data: Object.values(s.os_stats),
                    backgroundColor: ['#58a6ff', '#3fb950', '#d29922', '#bc8cff', '#f85149']
                }]
            },
            options: {
                ...baseOpts,
                plugins: {
                    legend: {
                        position: 'right',
                        labels: { color: '#8b949e', font: { size: 10 }, boxWidth: 10, padding: 6 }
                    }
                }
            }
        });
    }

    const groupCtx = document.getElementById('groupChart');
    if (groupCtx) {
        new Chart(groupCtx, {
            type: 'bar',
            data: {
                labels: Object.keys(s.group_stats),
                datasets: [{
                    label: '主机数',
                    data: Object.values(s.group_stats),
                    backgroundColor: ['#58a6ff', '#3fb950', '#d29922', '#bc8cff']
                }]
            },
            options: {
                ...baseOpts,
                plugins: { legend: { display: false } },
                scales: {
                    x: { grid: { display: false }, ticks: { color: '#8b949e', font: { size: 10 } } },
                    y: { grid: { color: '#21262d' }, ticks: { color: '#8b949e', font: { size: 10 } } }
                }
            }
        });
    }

    const subnetCtx = document.getElementById('subnetChart');
    if (subnetCtx) {
        new Chart(subnetCtx, {
            type: 'bar',
            data: {
                labels: Object.keys(s.subnet_stats || {}),
                datasets: [{
                    label: '主机数',
                    data: Object.values(s.subnet_stats || {}),
                    backgroundColor: '#58a6ff'
                }]
            },
            options: {
                ...baseOpts,
                indexAxis: 'y',
                plugins: { legend: { display: false } },
                scales: {
                    x: { grid: { color: '#21262d' }, ticks: { color: '#8b949e', font: { size: 10 } } },
                    y: { grid: { display: false }, ticks: { color: '#8b949e', font: { size: 10 } } }
                }
            }
        });
    }

    const privCtx = document.getElementById('privilegeChart');
    if (privCtx) {
        new Chart(privCtx, {
            type: 'doughnut',
            data: {
                labels: Object.keys(s.privilege_stats || {}),
                datasets: [{
                    data: Object.values(s.privilege_stats || {}),
                    backgroundColor: ['#f85149', '#3fb950', '#d29922']
                }]
            },
            options: {
                ...baseOpts,
                plugins: {
                    legend: {
                        position: 'right',
                        labels: { color: '#8b949e', font: { size: 10 }, boxWidth: 10, padding: 6 }
                    }
                }
            }
        });
    }

    const trafficCtx = document.getElementById('trafficChart');
    if (trafficCtx) {
        const trafficLabels = Object.keys(s.traffic_stats || {}).map(d => d.slice(5));
        const trafficData = Object.values(s.traffic_stats || {}).map(v => (v / 1024).toFixed(2));
        new Chart(trafficCtx, {
            type: 'line',
            data: {
                labels: trafficLabels,
                datasets: [{
                    label: '流量 (MB)',
                    data: trafficData,
                    borderColor: '#bc8cff',
                    backgroundColor: 'rgba(188,140,255,0.15)',
                    fill: true,
                    tension: 0.4,
                    pointBackgroundColor: '#bc8cff'
                }]
            },
            options: {
                ...baseOpts,
                plugins: { legend: { display: false } },
                scales: {
                    x: { grid: { color: '#21262d' }, ticks: { color: '#8b949e', font: { size: 10 } } },
                    y: { grid: { color: '#21262d' }, ticks: { color: '#8b949e', font: { size: 10 } } }
                }
            }
        });
    }
};
