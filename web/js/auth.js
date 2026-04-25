// auth.js — всё что связано с авторизацией и токеном
// Этот файл подключается на КАЖДОЙ странице
 
const Auth = {
  // Сохранить данные после логина
  // data = { token: "...", role: "admin"|"operator", name: "Иванова Мария Сергеевна", id: "OP-001" }
  save(data) {
    localStorage.setItem('token', data.token);
    localStorage.setItem('role', data.role);
    localStorage.setItem('name', data.name);
    localStorage.setItem('user_id', data.id);
  },
 
  // Получить токен (для отправки в заголовках запросов)
  getToken() {
    return localStorage.getItem('token');
  },
 
  getRole() {
    return localStorage.getItem('role');
  },
 
  // Получить отформатированное имя: "Иванова Мария Сергеевна" → "Иванова М.С."
  getShortName() {
    const name = localStorage.getItem('name') || '';
    const parts = name.trim().split(' ');
    if (parts.length < 2) return name;
    const last = parts[0];
    const first = parts[1] ? parts[1][0] + '.' : '';
    const middle = parts[2] ? parts[2][0] + '.' : '';
    return `${last} ${first}${middle}`;
  },
 
  getFullName() {
    return localStorage.getItem('name') || '';
  },
 
  getUserId() {
    return localStorage.getItem('user_id') || '';
  },
 
  isLoggedIn() {
    return !!this.getToken();
  },

   // Получить код сотрудника (user_code из твоего LoginResponse)
  getUserCode() {
    const user = JSON.parse(localStorage.getItem('user') || '{}');
    return user.user_code || null;
  },

  // Получить ФИО (full_name из твоего LoginResponse)
  getFullName() {
    const user = JSON.parse(localStorage.getItem('user') || '{}');
    return user.full_name || null;
  },

  // Получить логин (user_name из твоего LoginResponse)
  getUserName() {
    const user = JSON.parse(localStorage.getItem('user') || '{}');
    return user.user_name || null;
  },
 
  logout() {
    console.log('Выход из системы...');
    localStorage.clear();
    window.location.href = '/';
  },

 
  // Вызвать в начале каждой страницы с нужной ролью
  // Пример: Auth.requireRole('admin') — если не админ, выгонит
  requireRole(requiredRole) {
    if (!this.isLoggedIn()) {
      window.location.href = '/';
      return false;
    }
    if (this.getRole() !== requiredRole) {
      // Перенаправить на свою страницу
      window.location.href = this.getRole() === 'admin' ? '/admin.html' : '/operator.html';
      return false;
    }
    return true;
  },
 
  // Заполнить блок с именем пользователя (вызывается на каждой странице)
  // Ожидает элементы с id="user-short-name", "user-id-display", "user-initials"
  renderUserInfo() {
    const shortName = this.getShortName();
    const userId = this.getUserId();
 
    const el = (id) => document.getElementById(id);
 
    if (el('user-short-name')) el('user-short-name').textContent = shortName;
    if (el('user-id-display')) el('user-id-display').textContent = 'ID: ' + userId;
 
    // Инициалы для аватара: "Иванова М.С." → "ИМ"
    if (el('user-initials')) {
      const parts = shortName.split(' ');
      const initials = parts.map(p => p[0]).filter(Boolean).join('').slice(0, 2).toUpperCase();
      el('user-initials').textContent = initials;
    }
  }
};
 
