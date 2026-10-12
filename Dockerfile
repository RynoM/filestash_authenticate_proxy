# STEP1: CLONE THE CODE
FROM alpine/git as builder_prepare
WORKDIR /home/
ARG GIT_REPO=https://github.com/mickael-kerjean/filestash
ARG GIT_BRANCH=master
RUN git clone --depth 1 --single-branch --branch ${GIT_BRANCH} ${GIT_REPO}

# STEP2: BUILD BACKEND
FROM golang:1.26-trixie AS builder_backend
WORKDIR /home/filestash/
COPY --from=builder_prepare /home/filestash/ .

COPY server/plugin/plg_authenticate_proxy_header/ server/plugin/plg_authenticate_proxy_header/
RUN sed -i '/plg_authenticate_passthrough"/a\	_ "github.com/mickael-kerjean/filestash/server/plugin/plg_authenticate_proxy_header"' server/plugin/index.go

RUN apt-get update > /dev/null && \
    apt-get install -y curl make > /dev/null 2>&1 && \
    apt-get install -y libjpeg-dev libtiff-dev libpng-dev libwebp-dev libraw-dev libheif-dev libgif-dev libvips-dev > /dev/null 2>&1 && \
    apt-get install -y libavcodec-dev libavdevice-dev libavfilter-dev libavformat-dev libswresample-dev libswscale-dev libavutil-dev > /dev/null 2>&1 && \
    make init && \
    make build

# STEP3: BUILD PROD IMAGE
FROM debian:stable-slim
MAINTAINER mickael@kerjean.me
WORKDIR /app/
COPY --from=builder_backend /home/filestash/dist/ .
RUN apt-get update > /dev/null && \
    apt-get -y upgrade > /dev/null && \
    apt-get install -y --no-install-recommends ca-certificates curl ffmpeg libbrotli1 poppler-utils && \
    useradd filestash && \
    mkdir -p /app/data/state/ && \
    chown -R filestash:filestash /app/ && \
    chmod 730 /app/filestash && \
    rm -rf /var/lib/apt/lists/* && \
    rm -rf /tmp/*

USER filestash
CMD ["/app/filestash"]
EXPOSE 8334
