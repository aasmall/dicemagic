FROM alpine

RUN apk add --no-cache shadow ca-certificates libc6-compat \
    && sed -i 's/^CREATE_MAIL_SPOOL=yes/CREATE_MAIL_SPOOL=no/' /etc/default/useradd \
    && groupadd --gid 1003 kms \
    && useradd --uid 1003 --gid kms --shell /bin/sh --create-home kms \
    && apk del shadow 

COPY ./out/bin/mocks/kms /go/bin/kms

USER 1003
ENTRYPOINT /go/bin/kms

EXPOSE 40080