# Build stage
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bin/nut-webgui ./cmd/nut-webgui

# Runtime stage
FROM scratch
COPY --from=builder /bin/nut-webgui /nut-webgui
EXPOSE 9000
ENTRYPOINT ["/nut-webgui"]
