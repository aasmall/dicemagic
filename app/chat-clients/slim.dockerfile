FROM alpine

RUN apk add --no-cache ca-certificates shadow \
    && sed -i 's/^CREATE_MAIL_SPOOL=yes/CREATE_MAIL_SPOOL=no/' /etc/default/useradd \
    && groupadd --gid 1000 dicemagic \
    && useradd --uid 1000 --gid dicemagic --shell /bin/sh --create-home dicemagic \
    && apk del shadow 

COPY gopath/bin/chat-clients /go/bin/chat-clients

USER 1000
ENTRYPOINT /go/bin/chat-clients

EXPOSE 7070