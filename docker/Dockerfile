# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/paywatch ./cmd/paywatch

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
RUN adduser -D -H paywatch
USER paywatch
COPY --from=build /bin/paywatch /usr/local/bin/paywatch
COPY db/migrations /db/migrations
WORKDIR /
ENV ADDR=:8081
EXPOSE 8081
ENTRYPOINT ["paywatch"]
