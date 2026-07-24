FROM golang:1.25-alpine AS builder
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/tinder-core ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/tinder-core /usr/local/bin/tinder-core

EXPOSE 8080
ENTRYPOINT ["tinder-core"]
