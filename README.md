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
| Minio (S3 API) | API: `http://localhost:9100`, Console: `http://localhost:9101` (minioadmin / minioadmin) |
| Reports CDN (Nginx) | http://localhost:8082 |

## Яндекс ID (OAuth 2.0 через bionicpro-auth)

1. Создайте приложение на [oauth.yandex.ru](https://oauth.yandex.ru/).
2. **Redirect URI** в кабинете Яндекса:
   `http://localhost:8081/auth/callback`
3. Скопируйте `.env.example` → `.env` и укажите `YANDEX_CLIENT_ID` / `YANDEX_CLIENT_SECRET`.

### Поток

1. **Войти через Яндекс ID** -> `bionicpro-auth` -> Яндекс OAuth.
2. Callback приходит в `bionicpro-auth` (`/auth/callback`), сервис сам меняет `code` на токен Яндекса.
3. `bionicpro-auth` получает профиль у `https://login.yandex.ru/info`, сохраняет его в PostgreSQL (`yandex_user_profiles`) и создаёт серверную сессию.
4. При первом входе показывается экран согласия на использование данных.

### Проверка

```bash
# Вход через UI: кнопка «Войти через Яндекс ID» → согласие → главная

# Профиль в БД
docker compose exec profiles_db psql -U profiles -d profiles -c \
  "SELECT keycloak_sub, yandex_id, email, display_name, consent_granted_at FROM yandex_user_profiles;"
```

Keycloak в этом сценарии используется как основной IAM для логина по LDAP и хранения пользователей, а поток Яндекс OAuth выполняется в `bionicpro-auth`.

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

## Кэширование отчётов через S3 + CDN

- `bionicpro-reports` больше не отдаёт JSON отчёта напрямую из OLAP при каждом запросе.
- Сначала сервис проверяет наличие объекта в S3 (`minio`) и возвращает CDN ссылку.
- Если объекта нет, отчёт читается из OLAP один раз, сохраняется в S3 и затем отдаётся CDN ссылка.

### Формат ответа API отчётов

`GET /reports` возвращает:

```json
{
  "report_url": "http://localhost:8082/reports/v1/<hash>.json",
  "cached": true
}
```

### Обновление кэша при новом ETL

- Структура ключа в S3: `<REPORTS_DATA_VERSION>/<sha256(user_id)>.json`.
- Для новой выгрузки ETL нужно сменить `REPORTS_DATA_VERSION` (например, `v2` или `2026-05-28`).
- После смены версии сервис начинает читать/писать в новый префикс, старые ключи не мешают.
- Nginx CDN кэширует объекты по URL, поэтому новая версия автоматически даёт `cache miss` и прогрев без ручной инвалидации.

## Сборка только auth-сервиса

```bash
cd bionicpro-auth
go mod tidy
go build -o bin/bionicpro-auth ./cmd/server
./bin/bionicpro-auth
```
