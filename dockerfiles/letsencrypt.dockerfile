FROM certbot/dns-google:latest

RUN apk add --no-cache shadow \
    && sed -i 's/^CREATE_MAIL_SPOOL=yes/CREATE_MAIL_SPOOL=no/' /etc/default/useradd \
    && groupadd --gid 1000 certbot \
    && useradd --uid 1000 --gid certbot --shell /bin/sh --create-home certbot \
    && apk del shadow 

RUN apk add --no-cache lighttpd bash python curl python-dev musl-dev libffi-dev openssl-dev gcc ca-certificates

RUN mkdir /certbot
COPY ./out/include/letsencrypt/renewcerts.sh ./out/include/letsencrypt/deployment-patch-template.json ./out/include/letsencrypt/secret-patch-template.json /certbot/

WORKDIR /certbot

# RUN wget https://bootstrap.pypa.io/get-pip.py \
#     && python get-pip.py \
#     &&  pip install virtualenv \
#     &&  pip install certbot \
#     &&  pip install certbot-dns-google \
#     &&  rm get-pip.py
RUN pip install --upgrade pip && pip install pyasn1 google-api-python-client --upgrade

RUN chmod a+x renewcerts.sh
RUN chown 1000:1000 -R /certbot

USER 1000:1000
ENTRYPOINT [ "/bin/bash", "renewcerts.sh" ]
# ENTRYPOINT [ "/bin/bash" ]
# CMD ["-c","sleep 3000"]
EXPOSE 8080