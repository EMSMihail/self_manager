# Шаг 1: Сборка фронтенда (Svelte)
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Шаг 2: Сборка бэкенда (Go + CGO для SQLite)
FROM golang:1.22-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app/backend
COPY backend/go.mod ./
COPY backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Собираем статический бинарник
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-extldflags=-static" -o server .

# Шаг 3: Финальный минималистичный образ
FROM alpine:latest
# Устанавливаем таймзоны для корректной работы воркера уведомлений
RUN apk add --no-cache tzdata
WORKDIR /app
COPY --from=backend-builder /app/backend/server .
COPY --from=frontend-builder /app/frontend/build ./frontend/build

# Директория для SQLite базы (будет монтироваться через volume)
RUN mkdir -p ./data

EXPOSE 8080
CMD ["./server"]