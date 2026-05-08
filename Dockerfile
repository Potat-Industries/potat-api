# syntax=docker/dockerfile:1.7

FROM golang:1.26
WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 go build -o potat-api ./

EXPOSE 8080

ENTRYPOINT ["/app/potat-api"]
