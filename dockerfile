FROM golang:1.23.0 AS build
WORKDIR /build
COPY go.mod ./
RUN go mod download
COPY *.go ./
RUN go build -o pipeline -ldflags="-s -w"

FROM alpine:latest
LABEL version="1.0"
LABEL maintainer="Mosin Evgeny <m0s1nevgeny@yandex.ru>"
WORKDIR /app
COPY --from=build /build/pipeline .
CMD ["./pipeline"]