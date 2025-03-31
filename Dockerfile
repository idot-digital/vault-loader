FROM alpine

ARG TARGETARCH
COPY vault-loader-linux-${TARGETARCH} /usr/local/bin/vault-loader

RUN apk add --no-cache coreutils && \
    chmod +x /usr/local/bin/vault-loader

CMD ["/bin/sh"] 