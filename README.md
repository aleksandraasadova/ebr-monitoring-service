

## API Endpoints

| Method     | Endpoint             | Done? | Interface? | Description                                        |
| ---------- | -------------------- | ----- | ---------- | -------------------------------------------------- |
| **POST**   | `/api/v1/auth/login` |   да   |       да    | Аутентификация пользователя (получение JWT токена) |
| **POST**   | `/api/v1/users`      |    да  |     нет      | Создание нового пользователя (только для админов)  |


TODO next: middleware (**POST**   | `/api/v1/users`  - admin-only access via JWT) + server wrapper with graceful shutdown