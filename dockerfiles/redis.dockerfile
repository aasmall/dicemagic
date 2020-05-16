FROM redis:6-alpine
RUN apk add --no-cache bash libc6-compat
COPY ./out/include/redis-cluster/redis.conf /usr/local/etc/redis/redis.conf
COPY ./out/bin/redis-cluster /usr/local/bin/redis-cluster-configurator
COPY ./out/include/redis-cluster/bootstrap-pod.sh /usr/local/bin/
ENTRYPOINT ["bash", "/usr/local/bin/bootstrap-pod.sh", "/usr/local/etc/redis/redis.conf"]