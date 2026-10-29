# Установка и развертывание Abdal 4iProto Server

Данное руководство описывает полный процесс развертывания сервера: от системных требований и генерации ключей до компиляции из исходного кода и настройки демона `systemd`.

---

## 1. Системные требования

### Аппаратные требования
- **Процессор:** 1 vCPU (x86_64 или arm64).
- **Оперативная память:** от 512 МБ (рекомендуется 1 ГБ для большого количества клиентов).
- **Дисковое пространство:** 100 МБ для бинарника и базовых логов; дополнительное место под `users_log/` при включенном аудите.

### Поддерживаемые операционные системы
- Ubuntu 20.04 LTS / 22.04 LTS / 24.04 LTS
- Debian 11 / 12
- AlmaLinux / Rocky Linux / RHEL 8+
- Arch Linux

### Сетевые требования и фаервол
Серверу требуется доступ к входящим портам:
- **SSH туннели (TCP):** по умолчанию порты `64235`, `64236`, `64237` (настраиваются в `server_config.json`).
- **DNSTT туннель (UDP):** порт `53` (или альтернативный, например `5353`), если активирован DNS-туннель.

**Пример открытия портов в UFW (Ubuntu/Debian):**
```bash
sudo ufw allow 64235:64237/tcp comment "Abdal 4iProto SSH"
sudo ufw allow 53/udp comment "Abdal 4iProto DNSTT"
sudo ufw reload
```

**Пример открытия портов в Firewalld (RHEL/CentOS):**
```bash
sudo firewall-cmd --permanent --add-port=64235-64237/tcp
sudo firewall-cmd --permanent --add-port=53/udp
sudo firewall-cmd --reload
```

---

## 2. Подготовка каталога сервера

Стандартным рабочим каталогом сервера является `/usr/local/abdal-4iproto-server`.

```bash
sudo mkdir -p /usr/local/abdal-4iproto-server
sudo mkdir -p /usr/local/abdal-4iproto-server/data
sudo mkdir -p /usr/local/abdal-4iproto-server/users_traffic
sudo mkdir -p /usr/local/abdal-4iproto-server/users_log
cd /usr/local/abdal-4iproto-server
```

---

## 3. Генерация SSH Host Keys сервера

Сервер требует собственную пару SSH-ключей для аутентификации хоста перед клиентами. Рекомендуется использовать современный и криптостойкий алгоритм **Ed25519**.

Сгенерируйте ключи командой:
```bash
ssh-keygen -t ed25519 -f /usr/local/abdal-4iproto-server/id_ed25519 -N ""
```

Если по соображениям совместимости с устаревшими клиентами требуется RSA:
```bash
ssh-keygen -t rsa -b 4096 -f /usr/local/abdal-4iproto-server/id_rsa -N ""
```

Установите права доступа (приватный ключ должен быть защищен):
```bash
chmod 600 /usr/local/abdal-4iproto-server/id_ed25519
chmod 644 /usr/local/abdal-4iproto-server/id_ed25519.pub
```

> [!IMPORTANT]
> Имена файлов ключей должны совпадать с параметрами `private_key_file` и `public_key_file` в [`server_config.json`](file:///home/imperator/projects/home/agy/4iproto-server/docs/configuration.md).

---

## 4. Варианты установки бинарника

### Вариант A. Использование готового исполняемого файла
Если у вас есть собранный бинарник `abdal_4iproto_server_linux_amd64`:
```bash
sudo cp abdal_4iproto_server_linux_amd64 /usr/local/abdal-4iproto-server/
sudo chmod +x /usr/local/abdal-4iproto-server/abdal_4iproto_server_linux_amd64
```

### Вариант B. Сборка из исходников (Go)
Для самостоятельной сборки потребуется Go 1.22 или новее.

1. **Проверьте версию Go:**
   ```bash
   go version
   ```

2. **Клонируйте репозиторий и соберите бинарник:**
   ```bash
   git clone https://github.com/ebrasha/abdal-4iproto-server.git
   cd abdal-4iproto-server
   
   # Запуск тестов для проверки целостности
   go test ./...
   
   # Сборка оптимизированного бинарника без отладочных символов
   CGO_ENABLED=0 go build -ldflags="-s -w" -o abdal_4iproto_server_linux_amd64 main.go
   
   # Перенос в рабочий каталог
   sudo cp abdal_4iproto_server_linux_amd64 /usr/local/abdal-4iproto-server/
   sudo chmod +x /usr/local/abdal-4iproto-server/abdal_4iproto_server_linux_amd64
   ```

---

## 5. Начальная конфигурация

Скопируйте шаблоны конфигурационных файлов в рабочий каталог:

```bash
# Из репозитория (папка configuration-files-for-linux/)
sudo cp configuration-files-for-linux/server_config.json /usr/local/abdal-4iproto-server/
sudo cp configuration-files-for-linux/users.json /usr/local/abdal-4iproto-server/
sudo cp configuration-files-for-linux/blocked_ips.json /usr/local/abdal-4iproto-server/
```

Проверьте права на конфигурационные файлы:
```bash
sudo chmod 600 /usr/local/abdal-4iproto-server/*.json
```

Подробное описание параметров доступно в руководстве [Конфигурация сервера и пользователей](file:///home/imperator/projects/home/agy/4iproto-server/docs/configuration.md).

---

## 6. Настройка и запуск через Systemd

Для обеспечения автоматического перезапуска при сбоях и старта при загрузке ОС используется сервис `systemd`.

### Шаг 1. Создание сервисного файла
Создайте файл `/etc/systemd/system/abdal-4iproto-server.service` со следующим содержимым:

```ini
[Unit]
Description=Abdal 4iProto Server - High Performance SSH/SOCKS5 Tunnel Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/usr/local/abdal-4iproto-server
ExecStart=/usr/local/abdal-4iproto-server/abdal_4iproto_server_linux_amd64
Restart=always
RestartSec=3
LimitNOFILE=65536

# Параметры безопасности и изоляции
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/usr/local/abdal-4iproto-server
SyslogIdentifier=abdal-4iproto-server

[Install]
WantedBy=multi-user.target
```

Либо воспользуйтесь готовым скриптом установки:
```bash
sudo bash configuration-files-for-linux/install-service.sh
```

### Шаг 2. Активация и запуск сервиса
```bash
# Перечитать конфигурацию systemd
sudo systemctl daemon-reload

# Включить автозагрузку при старте ОС
sudo systemctl enable abdal-4iproto-server.service

# Запустить сервис
sudo systemctl start abdal-4iproto-server.service
```

### Шаг 3. Проверка статуса
```bash
sudo systemctl status abdal-4iproto-server.service
```

В выводе должно быть:
```text
● abdal-4iproto-server.service - Abdal 4iProto Server
     Loaded: loaded (/etc/systemd/system/abdal-4iproto-server.service; enabled)
     Active: active (running)
```

Также убедитесь, что сервер успешно слушает порты:
```bash
ss -tulpn | grep -E '64235|64236|64237'
```

---

## 7. Следующие шаги
- Перейдите к разделу [**Конфигурация сервера и пользователей**](file:///home/imperator/projects/home/agy/4iproto-server/docs/configuration.md) для настройки политик доступа, лимитов сессий и квот.
- Настройте клиентов по инструкции [**Подключение клиентов**](file:///home/imperator/projects/home/agy/4iproto-server/docs/clients.md).
