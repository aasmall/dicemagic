#!/bin/bash
#set -e
set -m
_term() { 
  echo "Caught SIGTERM signal!" 
  kill -TERM %1 2>/dev/null
  kill -TERM %2 2>/dev/null
}
trap _term SIGTERM
FILE=nodes.conf
if test -f "$FILE"; then
  sed -i -E "s/(.+)\b([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})(:.*myself.*)/\1$(hostname -i)\3/" $FILE
fi

redis-server $1 &
redis-cluster-configurator &
sleep 5

wait %1