# build
FROM golang:1.21-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags='-s -w' -o /microapi ./cmd/server

# final
FROM scratch
COPY --from=builder /microapi /microapi
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/microapi"]
