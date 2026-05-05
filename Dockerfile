FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/tg-todo .

FROM alpine:3.20
RUN adduser -D -H -s /sbin/nologin app
USER app

WORKDIR /app
ENV DB_DIR=/data
COPY --from=build /out/tg-todo /app/tg-todo

VOLUME ["/data"]
ENTRYPOINT ["/app/tg-todo"]

