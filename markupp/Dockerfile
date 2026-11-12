# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/markupp ./cmd/markupp

FROM alpine:3.20
# A pasta do segredo do MCP nasce com o dono do servidor: um volume nomeado novo herda
# esse dono, e o servidor sem root consegue gravar nele.
RUN apk add --no-cache ca-certificates \
  && adduser -D -u 10001 markupp \
  && install -d -o markupp -g markupp /run/markupp-mcp
COPY --from=builder /out/markupp /usr/local/bin/markupp
USER markupp
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/markupp"]
