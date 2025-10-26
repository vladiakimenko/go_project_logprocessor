# Генерация исходных данных

### Требования
- OS: Linux или MacOs  

Создайте csv файл с моковыми логами api-запросов для тестирования.

```bash
./generate_log.sh log.csv total 100000
```
Создаёт файл с исходными данными размером около 7мб

# Настройка приложения

Настройте файл конфигурации `config.json`

**core.filepath** — путь к CSV-файлу с логами  
**core.workers** — количество воркеров  
**core.tops** — количество значений для подсчёта (top N)  
**logging.json** — логи в формате JSON (true) или plaintext(false)  
**logging.level** — уровень логирования (debug, info, warn, error)  

```
{
    "core": {
        "filepath": "log.csv",
        "workers": 4,
        "tops": 3
    },
    "logging": {
        "json": false,
        "level": "debug"
    }
}
```

# Приложение

### Компиляция
```
go build -o logprocessor cmd/logprocessor/main.go
```

### Запуск
./logprocessor <task> [field] [value]


### Аргументы запуска
| Параметр | Описание |
|----------|----------|
| `task`   | Тип задачи: `stats`, `top`, `filter` |
| `field`  | Поле для подсчёта топ-N (`top`) или фильтра (`filter`). Например: `ip`, `status`, `method`, `url`, `response_time`, `timestamp`. |
| `value`  | Значение для фильтрации (`filter`). Например: `200` для `status` или `GET` для `method`. |


### Примеры

Подсчёт общей статистики:
```
./logprocessor stats
```

Топ-3 значений для поля ip:
```
./logprocessor top ip
```
Фильтр по конкретному значению поля, например статус 500:
```
./logprocessor filter status 500
```