FROM alpine

RUN apk add --no-cache shadow ca-certificates libc6-compat \
    && sed -i 's/^CREATE_MAIL_SPOOL=yes/CREATE_MAIL_SPOOL=no/' /etc/default/useradd \
    && groupadd --gid 1003 slack-server \
    && useradd --uid 1003 --gid slack-server --shell /bin/sh --create-home slack-server \
    && apk del shadow 

COPY ./out/bin/mocks/slack-server /go/bin/slack-server

USER 1003
ENTRYPOINT /go/bin/slack-server

EXPOSE 40080