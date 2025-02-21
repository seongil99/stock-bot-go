# Use an official Golang image to build the app.
FROM golang:1.23-bullseye as builder
WORKDIR /app
COPY . .
RUN go build -o myapp .

# Create the final image.
FROM debian:bullseye-slim

# Install necessary packages and Google Chrome.
RUN apt-get update && apt-get install -y wget gnupg ca-certificates unzip && \
  wget -q -O - https://dl.google.com/linux/linux_signing_key.pub | apt-key add - && \
  echo "deb [arch=amd64] http://dl.google.com/linux/chrome/deb/ stable main" \
  > /etc/apt/sources.list.d/google-chrome.list && \
  apt-get update && \
  apt-get install -y google-chrome-stable && \
  rm -rf /var/lib/apt/lists/*

# Copy the Go binary from the builder.
COPY --from=builder /app/myapp /app/myapp

# Run the application.
CMD ["/app/myapp"]
