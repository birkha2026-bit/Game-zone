FROM golang:1.22-alpine AS builder

WORKDIR /app

# نسخ جميع الملفات أولاً لكي يتعرف المترجم على الحزم المستخدمة في الكود
COPY . .

# جلب التبعيات وتوليد ملف go.sum تلقائياً
RUN go mod tidy

ENV CGO_ENABLED=0
ENV GOOS=linux
RUN go build -o tracker_app main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/tracker_app .

EXPOSE 8080
CMD ["./tracker_app"]
