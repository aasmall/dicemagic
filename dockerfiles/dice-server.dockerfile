FROM alpine

RUN apk add --no-cache shadow ca-certificates libc6-compat \
    && sed -i 's/^CREATE_MAIL_SPOOL=yes/CREATE_MAIL_SPOOL=no/' /etc/default/useradd \
    && groupadd --gid 1000 dicemagic \
    && useradd --uid 1000 --gid dicemagic --shell /bin/sh --create-home dicemagic \
    && apk del shadow 

COPY ./out/bin/dice-server /go/bin/dice-server

USER 1000
ENTRYPOINT /go/bin/dice-server

EXPOSE 50051