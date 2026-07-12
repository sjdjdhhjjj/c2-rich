    const API = {
        token: localStorage.getItem('c2_token') || '',

        async request(path, options = {}) {
            const headers = { 'Content-Type': 'application/json', ...options.headers };
            if (this.token) headers['Authorization'] = 'Bearer ' + this.token;

            const res = await fetch(path, { ...options, headers });
            if (res.status === 401) {
                this.token = '';
                localStorage.removeItem('c2_token');
                app.render();
                throw new Error('Unauthorized');
            }
            if (res.status === 204) return {};
            if (!res.ok) {
                // 非 2xx 状态码：尝试解析 JSON 错误体，失败则抛 HTTP 错误
                let msg = `HTTP ${res.status}`;
                try { const e = await res.json(); if (e && e.error) msg = e.error; } catch (_) {}
                throw new Error(msg);
            }
            // 部分成功响应可能无 body（如 200 但空），容错
            const text = await res.text();
            if (!text) return {};
            try { return JSON.parse(text); }
            catch (_) { return { raw: text }; }
        },

        get(path) { return this.request(path); },
        post(path, data) { return this.request(path, { method: 'POST', body: JSON.stringify(data) }); },
        put(path, data) { return this.request(path, { method: 'PUT', body: JSON.stringify(data) }); },
        del(path) { return this.request(path, { method: 'DELETE' }); }
    };
