# استخدام بيئة بناء Go الرسمية
FROM golang:1.22-alpine AS builder

# تثبيت حزم النظام الضرورية لـ CGO و SQLite
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# نسخ ملفات التبعيات والكود
COPY go.mod ./
RUN go mod download

COPY . .

# تفعيل CGO وبناء الملف التنفيذي
ENV CGO_ENABLED=1
ENV GOOS=linux
RUN go build -o tracker_app main.go

# المرحلة النهائية للتشغيل
FROM alpine:latest
RUN apk add --no-cache sqlite-libs ca-certificates

WORKDIR /app
COPY --from=builder /app/tracker_app .

EXPOSE 8080
CMD ["./tracker_app"]
