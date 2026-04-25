// api.js — все запросы к бэкенду
// Меняй BASE_URL под свой сервер

const API_BASE = 'http://localhost:8081'; // ← поменяй на свой адрес

const Api = {
  // Базовый метод — добавляет токен в заголовки автоматически
  async _request(method, path, body = null) {
    const headers = { 'Content-Type': 'application/json' };

    const token = Auth.getToken();
    if (token) headers['Authorization'] = 'Bearer ' + token;

    const options = { method, headers };
    if (body) options.body = JSON.stringify(body);

    const resp = await fetch(API_BASE + path, options);

    if (resp.status === 401) {
      // Токен протух — выгнать
      Auth.logout();
      return;
    }

    if (!resp.ok) {
      const err = await resp.json().catch(() => ({ message: 'Ошибка сервера' }));
      throw new Error(err.message || `HTTP ${resp.status}`);
    }

    return resp.json();
  },

  // ── Авторизация ──────────────────────────────────────────────

  // POST /auth/login { username, password }
  // Ожидаем ответ: { token, role, name, id }
  async login(username, password) {
    return this._request('POST', '/login', { username, password });
  },

  // ── Пользователи (только для админа) ─────────────────────────

  // GET /users — список пользователей
  async getUsers() {
    return this._request('GET', '/users');
  },

  // POST /users { username, password, name, role }
  async createUser(userData) {
    return this._request('POST', '/users', userData);
  },

  // DELETE /users/:id
  async deleteUser(userId) {
    return this._request('DELETE', `/users/${userId}`);
  },
};