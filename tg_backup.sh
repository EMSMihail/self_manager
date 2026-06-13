#!/bin/bash

# Конфигурация
BOT_TOKEN="ТВОЙ_ТОКЕН_ОТ_BOTFATHER"
CHAT_ID="ТВОЙ_CHAT_ID"
DB_PATH="/path/to/your/app/data/app.db" # Путь к примонтированному volume базы
BACKUP_DIR="/tmp"
DATE=$(date +"%Y-%m-%d_%H-%M")
BACKUP_FILE="kanban_backup_${DATE}.db.gz"
TEMP_DB="${BACKUP_DIR}/temp.db"
TG_API_URL="https://api.telegram.org/bot${BOT_TOKEN}/sendDocument"

# 1. Безопасное создание копии (горячий бэкап SQLite)
sqlite3 "$DB_PATH" ".backup '$TEMP_DB'"

# 2. Сжатие базы
gzip -c "$TEMP_DB" > "${BACKUP_DIR}/${BACKUP_FILE}"

# 3. Отправка архива в Telegram через multipart/form-data
curl -s -X POST "$TG_API_URL" \
  -F chat_id="$CHAT_ID" \
  -F document="@${BACKUP_DIR}/${BACKUP_FILE}" \
  -F caption="📦 Бэкап базы данных Kanban за $DATE" > /dev/null

# 4. Очистка временных файлов
rm "$TEMP_DB"
rm "${BACKUP_DIR}/${BACKUP_FILE}"

echo "Бэкап успешно отправлен в Telegram."