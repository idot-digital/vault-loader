FROM alpine

ARG TARGETARCH
COPY vault-loader-linux-${TARGETARCH} /usr/local/bin/vault-loader

RUN chmod +x /usr/local/bin/vault-loader

ENTRYPOINT ["vault-loader"] 