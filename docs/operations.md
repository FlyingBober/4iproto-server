# Эксплуатация и мониторинг Abdal 4iProto Server

Данный раздел содержит инструкции для системных инженеров и дежурных администраторов по управлению жизненным циклом сервиса, аудиту, мониторингу трафика, резервному копированию и решению инцидентов.

---

## 1. Управление сервисом через Systemd

Сервер управляется стандартными командами `systemctl`:

```bash
# Проверка текущего статуса сервиса
sudo systemctl status abdal-4iproto-server.service

# Запуск / остановка
sudo systemctl start abdal-4iproto-server.service
sudo systemctl stop abdal-4iproto-server.service

# Перезапуск (с применением новой конфигурации)
sudo systemctl restart abdal-4iproto-server.service
```

---

## 2. Корректное завершение работы (Graceful Shutdown)

Сервер поддерживает безопасное завершение работы по сигналам операционной системы:
- `SIGINT` (прерывание клавишами `Ctrl+C` в терминале)
- `SIGTERM` (штатная команда `systemctl stop` или перезагрузка сервера)

### Что происходит при поступлении сигнала:
1. Сервер немедленно перестает принимать новые входящие TCP/UDP подключения на всех портах.
2. Инициируется процедура завершения активных туннелей клиентов.
3. **Сохранение трафика:** Менеджер трафика выполняет вызов `SaveAllTraffic()` — все накопленные в оперативной памяти счетчики байтов сбрасываются на диск в файлы `users_traffic/traffic_<username>.json` через механизм **атомарной записи** (запись во временный файл `.tmp` с последующим системным вызовом `rename()`). Это исключает обнуление или повреждение файлов при внезапном выключении.
4. Корректно закрывается база данных BoltDB (`data/sessions.db`).

---

## 3. Мониторинг и анализ журналов (Логи)

Сервер ведет несколько типов логов для различных задач мониторинга:

### А. Системный журнал службы (systemd journal)
Выводит общую информацию о запуске, прослушиваемых портах, подключении пользователей, вытеснении сессий и ошибках:

```bash
# Просмотр логов в реальном времени
sudo journalctl -u abdal-4iproto-server.service -f

# Просмотр последних 100 строк без пагинации
sudo journalctl -u abdal-4iproto-server.service -n 100 --no-pager
```

**Пример сообщений о вытеснении сессии в логе:**
```text
[SESSION] User mob_1 reached session limit (1/1) - evicting oldest session (token: a1b2c3d4...)
[SESSION] Successfully evicted session a1b2c3d4... for user mob_1
```

### Б. Журнал безопасности (`invalid_logins.log`)
Находится в рабочей директории сервера (`/usr/local/abdal-4iproto-server/invalid_logins.log`). В него записываются все неудачные попытки авторизации (неверный логин или пароль, превышение порога брутфорса Guard).

```bash
sudo tail -f /usr/local/abdal-4iproto-server/invalid_logins.log
```

### В. Журналы активности пользователей (`users_log/`)
Если для пользователя в `users.json` включен параметр `"log": "yes"`, сервер пишет каждое открытие туннеля и целевой домен/порт в персональный файл `users_log/<username>.log`:

```bash
sudo tail -f /usr/local/abdal-4iproto-server/users_log/admin.log
```

---

## 4. Ротация логов (Logrotate)

Чтобы журналы не заполнили свободное место на диске, настройте системную ротацию. Создайте файл `/etc/logrotate.d/abdal-4iproto-server`:

```text
/usr/local/abdal-4iproto-server/*.log
/usr/local/abdal-4iproto-server/users_log/*.log {
    daily
    missingok
    rotate 14
    compress
    delaycompress
    notifempty
    copytruncate
}
```

Проверка конфигурации ротации без ожидания расписания:
```bash
sudo logrotate -d /etc/logrotate.d/abdal-4iproto-server
```

---

## 5. Учет и сброс квот трафика

### Хранение статистики
Данные по каждому пользователю сохраняются в каталоге `users_traffic/` в файле `traffic_<username>.json`:

```bash
# Просмотр статистики пользователя mob_1 с форматированием JSON
jq . /usr/local/abdal-4iproto-server/users_traffic/traffic_mob_1.json
```

**Пример содержимого файла:**
```json
{
  "username": "mob_1",
  "ip": "178.62.204.15",
  "last_bytes_sent": 142080,
  "last_bytes_received": 1845120,
  "last_bytes_total": 1987200,
  "total_bytes_sent": 524288000,
  "total_bytes_received": 4718592000,
  "total_bytes": 5242880000,
  "last_timestamp": "2026-10-29 17:15:00"
}
```

### Сброс квоты (ежемесячное обнуление)
Для сброса накопленного трафика конкретного пользователя:
1. Остановите сервис (чтобы не перезаписать данные из оперативной памяти):
   ```bash
   sudo systemctl stop abdal-4iproto-server.service
   ```
2. Удалите или отредактируйте файл трафика:
   ```bash
   sudo rm /usr/local/abdal-4iproto-server/users_traffic/traffic_mob_1.json
   ```
3. Запустите сервис:
   ```bash
   sudo systemctl start abdal-4iproto-server.service
   ```

---

## 6. База данных сессий (BoltDB) и резервное копирование

Сервер использует встроенную key-value базу данных **BoltDB**, расположенную по пути `data/sessions.db`. В ней хранятся:
- Активные токены сессий;
- Временные метки последней активности (`LastSeen`);
- Идентификаторы SSH-каналов для обеспечения вытеснения.

### Резервное копирование
Рекомендуется настроить периодический бэкап конфигураций и баз данных:
```bash
# Создание архива резервной копии
tar -czf /root/abdal_backup_$(date +%F).tar.gz \
    /usr/local/abdal-4iproto-server/*.json \
    /usr/local/abdal-4iproto-server/id_ed25519* \
    /usr/local/abdal-4iproto-server/data/sessions.db \
    /usr/local/abdal-4iproto-server/users_traffic/
```

### Восстановление поврежденной базы `sessions.db`
Если в результате сбоя питания файл `sessions.db` был поврежден (в логах ошибки `bolt: file too short` или `invalid page type`):
```bash
sudo systemctl stop abdal-4iproto-server.service
sudo mv /usr/local/abdal-4iproto-server/data/sessions.db /usr/local/abdal-4iproto-server/data/sessions.db.corrupt
sudo systemctl start abdal-4iproto-server.service
```
Сервер автоматически инициализирует чистую базу `sessions.db` при следующем старте.

---

## 7. Процедура обновления бинарника сервера

При обновлении бинарного файла операционная система Linux блокирует перезапись запущенного файла с ошибкой `cp: cannot create regular file ...: Text file busy` (`ETXTBSY`).

**Правильный порядок обновления:**
```bash
# 1. Остановить сервис
sudo systemctl stop abdal-4iproto-server.service

# 2. Скопировать новый исполняемый файл
sudo cp abdal_4iproto_server_linux_amd64 /usr/local/abdal-4iproto-server/
sudo chmod +x /usr/local/abdal-4iproto-server/abdal_4iproto_server_linux_amd64

# 3. Запустить сервис
sudo systemctl start abdal-4iproto-server.service

# 4. Проверить статус
sudo systemctl status abdal-4iproto-server.service --no-pager
```

---

## 8. Руководство по устранению неполадок (Troubleshooting)

### 🔴 Симптом 1: Клиент не подключается, ошибка `max_sessions reached`
- **Причина:** У пользователя закончился лимит одновременных сессий, а старое соединение зависло (например, телефон отключился от сотовой сети без закрытия сокета).
- **Решение:** Включите вытеснение сессий для пользователя в `users.json`:
  ```json
  "evict_on_max_sessions": true
  ```
  После сохранения файла выполните `sudo systemctl restart abdal-4iproto-server.service`.

### 🔴 Симптом 2: Сервер падает при старте: `bind: address already in use`
- **Причина:** Порт (например, `64235` или `53`) уже занят другим процессом или зависшим экземпляром сервера.
- **Диагностика:**
  ```bash
  sudo ss -tulpn | grep 64235
  ```
- **Решение:** Завершите зависший процесс (`sudo kill -9 <PID>`) и перезапустите службу.

### 🔴 Симптом 3: DNSTT не запускается на порту 53 (`permission denied`)
- **Причина:** Порты ниже 1024 в Linux являются привилегированными.
- **Решение:** Убедитесь, что сервис запускается от пользователя `root` (в `abdal-4iproto-server.service` указано `User=root`), либо выдайте бинарнику разрешение `cap_net_bind_service`:
  ```bash
  sudo setcap 'cap_net_bind_service=+ep' /usr/local/abdal-4iproto-server/abdal_4iproto_server_linux_amd64
  ```

### 🔴 Симптом 4: IP-адрес заблокирован Guard после серии ошибок
- **Причина:** Пользователь несколько раз ошибся в пароле и превысил `max_auth_attempts`.
- **Диагностика:** Проверьте `invalid_logins.log`.
- **Решение:** Перезапустите сервис `sudo systemctl restart abdal-4iproto-server.service` (сбрасывает временный кэш блокировок Guard в памяти) или подождите истечения окна блокировки.
