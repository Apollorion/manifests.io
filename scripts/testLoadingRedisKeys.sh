#!/bin/bash

redisContainer=$(docker ps | grep manifests-io-redis)
if [[ $? -ne 0 ]]; then
  echo "Redis is not running!"
  echo "Start lambda with './scripts/startLambdaDev.sh --redis' then rerun this script"
  exit 1
fi

cd ETL
python3 -m pip install -r requirements.txt
REDIS_HOST=localhost python3 ./load_redis.py