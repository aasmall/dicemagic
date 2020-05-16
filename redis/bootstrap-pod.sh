#!/bin/bash
#set -e
set -m
_term() { 
  echo "Caught SIGTERM signal!" 
  kill -TERM %1 2>/dev/null
  kill -TERM %2 2>/dev/null
}
trap _term SIGTERM

redis-server $1 &
redis-cluster-configurator &
sleep 5

wait %1