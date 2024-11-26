FROM golang:latest AS compiling_stage
RUN mkdir -p /go/src/pipeline
WORKDIR /go/src/pipeline
ADD pipeline.go .
ADD go.mod .
RUN go install .

FROM alpine:latest
LABEL version="1.0"
LABEL maintainer="Mosin Evgeny <m0s1nevgeny@yandex.ru>"
WORKDIR /root/
COPY --from=compiling_stage /go/bin/pipeline .
ENTRYPOINT ./pipeline