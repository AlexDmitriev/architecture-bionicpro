# architecture-bionicpro

Учебный проект BionicPRO: SSO через Keycloak, BFF **bionicpro-auth** (Go), сессии в Redis.

## Запуск

```bash
# Первый запуск или после смены realm (access token 2 мин)
docker compose down -v
docker compose up --build -d
```

Сервисы:

| Сервис | URL |
|--------|-----|
| Frontend | http://localhost:3000 |
| bionicpro-auth | http://localhost:8081 |
| Keycloak | http://localhost:8080 (admin / admin) |
| OpenLDAP | ldap://localhost:389 |
| Redis | localhost:6379 |
| Profiles DB | localhost:5434 |

## Яндекс ID (OAuth 2.0 Identity Brokering)

1. Создайте приложение на [oauth.yandex.ru](https://oauth.yandex.ru/).
2. **Redirect URI** в кабинете Яндекса:
   `http://localhost:8080/realms/reports-realm/broker/yandex/endpoint`
3. Скопируйте `.env.example` → `.env` и укажите `YANDEX_CLIENT_ID` / `YANDEX_CLIENT_SECRET`.
4. После `docker compose up` выполните:

```bash
export $(grep -v '^#' .env | xargs)
./scripts/configure-yandex-idp.sh
```

### Поток

1. **Войти через Яндекс ID** → Keycloak (brokering) → Яндекс OAuth (scope: профиль, email, аватар).
2. После входа — экран **согласия** на использование данных профиля.
3. При подтверждении `bionicpro-auth` запрашивает профиль у `https://login.yandex.ru/info` и сохраняет в PostgreSQL (`yandex_user_profiles`).

### Проверка

```bash
# Вход через UI: кнопка «Войти через Яндекс ID» → согласие → главная

# Профиль в БД
docker compose exec profiles_db psql -U profiles -d profiles -c \
  "SELECT keycloak_sub, yandex_id, email, display_name, consent_granted_at FROM yandex_user_profiles;"
```

В Keycloak Admin: **Identity Providers** → `yandex`, **Clients** → `bionicpro-auth` → включите **Token Exchange** и разрешите обмен на issuer `yandex` (для прямого запроса API Яндекса).

## LDAP и зарубежное представительство

- **OpenLDAP** (`osixia/openldap`) поднимается с данными из `ldap/config.ldif`.
- **Keycloak** (User Federation `bionicpro-ldap`) аутентифицирует пользователей в LDAP и импортирует их при первом входе.
- Группы LDAP с именами `user`, `prothetic_user`, `administrator` маппятся на **realm roles** Keycloak (одинаковые имена в локальном и зарубежном OU — единая модель ролей для всех представительств).
- Поиск пользователей и групп — **subtree** от `dc=example,dc=com` (охватывает `ou=People` и `ou=Foreign/...`).

### Пользователи LDAP (пароль: `password`)

| Логин | Представительство | Роль в LDAP / Keycloak |
|-------|-------------------|-------------------------|
| john.doe | локальный каталог | prothetic_user |
| jane.smith | локальный каталог | user |
| alex.johnson | локальный каталог | prothetic_user |
| hans.mueller | ou=Foreign | prothetic_user |
| maria.garcia | ou=Foreign | user |
| admin.foreign | ou=Foreign | administrator |

Локальные пользователи Keycloak (`prothetic1`, `user1`, …) по-прежнему доступны параллельно с LDAP.

### Проверка LDAP

```bash
# Список пользователей в каталоге
ldapsearch -x -H ldap://localhost:389 -D "cn=admin,dc=example,dc=com" -w admin \
  -b "dc=example,dc=com" "(objectClass=inetOrgPerson)" uid mail

# Группы зарубежного представительства
ldapsearch -x -H ldap://localhost:389 -D "cn=admin,dc=example,dc=com" -w admin \
  -b "ou=Groups,ou=Foreign,dc=example,dc=com" "(objectClass=groupOfNames)" cn member
```

В Keycloak Admin → **User Federation** → `bionicpro-ldap` → **Synchronize all users**, затем вход через UI как `hans.mueller` / `password`.

## Задача 3: безопасные токены

- OAuth2/PKCE и обмен code→tokens выполняет **bionicpro-auth**, не фронтенд.
- **access_token** — в Redis, привязан к session id.
- **refresh_token** — в Redis в зашифрованном виде (AES-256-GCM).
- Фронтенд получает только cookie `bionicpro_session` (HttpOnly; Secure включается через `COOKIE_SECURE=true`).
- Время жизни сессии (30 мин) больше access token (2 мин); при истечении access сервис обновляет его через refresh.
- На защищённых эндпоинтах (`/auth/reports`, `/auth/token`) выполняется **ротация session id** (защита от session fixation).

### Тестовые пользователи

| Логин | Пароль |
|-------|--------|
| prothetic1 | prothetic123 |
| user1 | password123 |

## Проверка, что авторизация идёт через бэкенд

1. Откройте DevTools → **Application** → Cookies для `http://localhost:3000` и `http://localhost:8081`.
   - Должна быть только `bionicpro_session` (HttpOnly).
   - Не должно быть `access_token` / `refresh_token` в Local Storage.

2. DevTools → **Network** → Login → после редиректа запрос `GET /auth/me`:
   - Request: заголовок `Cookie: bionicpro_session=...`
   - Без `Authorization: Bearer` со стороны фронта.

3. Проверка через curl (после логина в браузере скопируйте cookie или пройдите flow вручную):

```bash
# Health
curl -s http://localhost:8081/health

# Статус без cookie — не авторизован
curl -s http://localhost:8081/auth/me

# Login — редирект на Keycloak (откройте в браузере)
curl -sI http://localhost:8081/auth/login | head -5
```

4. После входа в UI нажмите **Download Report** — в Network будет `GET http://localhost:8081/auth/reports` с `credentials: include` (cookie), без токена в JS.

5. Ротация сессии: дважды вызовите защищённый эндпоинт и сравните значение cookie:

```bash
# Подставьте cookie из браузера после логина
COOKIE="bionicpro_session=ВАШ_ID"
curl -s -D - -o /dev/null -H "Cookie: $COOKIE" http://localhost:8081/auth/me | grep -i set-cookie
curl -s -D - -o /dev/null -H "Cookie: $COOKIE" http://localhost:8081/auth/token | grep -i set-cookie
```

В ответе защищённых запросов приходит новый `Set-Cookie` с другим session id.

6. Keycloak: Realm Settings → Tokens — **Access Token Lifespan** = 2 minutes.

## Локальная разработка фронтенда

```bash
cd frontend
npm install
REACT_APP_AUTH_URL=http://localhost:8081 npm start
```

## Сборка только auth-сервиса

```bash
cd bionicpro-auth
go mod tidy
go build -o bin/bionicpro-auth ./cmd/server
./bin/bionicpro-auth
```
