FROM alpine

RUN apk add --no-cache shadow libc6-compat \
    && sed -i 's/^CREATE_MAIL_SPOOL=yes/CREATE_MAIL_SPOOL=no/' /etc/default/useradd \
    && groupadd --gid 1000 dicemagic \
    && useradd --uid 1000 --gid dicemagic --shell /bin/sh --create-home dicemagic \
    && apk del shadow 


COPY ./out/bin/www /usr/bin/www
COPY ./out/include/www/ /srv

USER 1000:1000

ENTRYPOINT /usr/bin/www

EXPOSE 8080