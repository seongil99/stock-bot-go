FROM golang:1.23-bullseye AS builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o myapp .

FROM --platform=amd64 debian:bullseye-slim

WORKDIR /app

RUN apt-get update && apt-get install -y wget gnupg ca-certificates unzip && \
    wget -qO - https://dl.google.com/linux/linux_signing_key.pub | gpg --dearmor > /usr/share/keyrings/google-linux-signing-key.gpg && \
    echo "deb [arch=amd64 signed-by=/usr/share/keyrings/google-linux-signing-key.gpg] http://dl.google.com/linux/chrome/deb/ stable main" > /etc/apt/sources.list.d/google-chrome.list && \
    apt-get update && apt-get install -y google-chrome-stable && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/myapp /app/myapp
COPY --from=builder /app/.env /app/.env

CMD ["/app/myapp"]
