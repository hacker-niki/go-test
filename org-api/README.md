# Org API

REST API организационной структуры (подразделения + сотрудники) на Go.

## Стек

- **Go 1.22** + `net/http` (стандартный роутер с `{id}`)
- **PostgreSQL 16** — хранилище данных
- **GORM** — работа с БД
- **goose** — миграции
- **Docker + docker-compose** — контейнеризация
- **testify** — интеграционные тесты

## Структура проекта

```
org-api/
├── cmd/api/main.go              # точка входа
├── internal/
│   ├── config/                  # конфигурация через env
│   ├── db/                      # подключение и миграции
│   │   └── migrations/          # SQL-миграции goose
│   ├── models/                  # GORM-модели
│   ├── repository/              # слой работы с БД
│   ├── service/                 # бизнес-логика
│   └── handler/                 # HTTP-обработчики
├── tests/                       # интеграционные тесты
├── Dockerfile
└── docker-compose.yml
```

## Запуск

```bash
docker-compose up --build
```

API будет доступен на `http://localhost:8080`.

## Переменные окружения

| Переменная | По умолчанию | Описание           |
|------------|--------------|--------------------|
| DB_HOST    | localhost    | Хост PostgreSQL    |
| DB_PORT    | 5432         | Порт PostgreSQL    |
| DB_USER    | postgres     | Пользователь БД    |
| DB_PASSWORD| postgres     | Пароль БД          |
| DB_NAME    | orgdb        | Имя базы данных    |
| SERVER_PORT| 8080         | Порт HTTP-сервера  |

## API

### Создать подразделение

```
POST /departments/
Content-Type: application/json

{"name": "Engineering", "parent_id": null}
```

### Создать сотрудника

```
POST /departments/{id}/employees/
Content-Type: application/json

{"full_name": "Alice Smith", "position": "Backend Dev", "hired_at": "2023-01-15"}
```

### Получить подразделение с деревом

```
GET /departments/{id}?depth=2&include_employees=true&sort_by=full_name
```

Параметры:
- `depth` — глубина дочерних подразделений (по умолчанию 1, максимум 5)
- `include_employees` — включить список сотрудников (по умолчанию true)
- `sort_by` — сортировка сотрудников: `created_at` (по умолчанию) или `full_name`

### Обновить подразделение

```
PATCH /departments/{id}
Content-Type: application/json

{"name": "Platform Engineering", "parent_id": 5}
```

Оба поля опциональны. `"parent_id": null` переносит подразделение в корень.

### Удалить подразделение

```
DELETE /departments/{id}?mode=cascade
DELETE /departments/{id}?mode=reassign&reassign_to_department_id=3
```

- `cascade` — удалить подразделение, всех сотрудников и все дочерние подразделения
- `reassign` — перевести сотрудников в другое подразделение, затем удалить

## Запуск тестов

Тесты интеграционные, требуют запущенной БД:

```bash
# с запущенным docker-compose
docker-compose up -d postgres
DB_HOST=localhost go test ./tests/ -v

# или внутри контейнера
docker-compose run --rm api go test ./tests/ -v
```

## Бизнес-правила

- Имя подразделения: 1–200 символов, обрезаются пробелы по краям
- Уникальность имени в пределах одного родителя
- Нельзя назначить подразделение родителем самого себя → 409
- Нельзя создать цикл в дереве → 409
- Создание сотрудника в несуществующем подразделении → 404
