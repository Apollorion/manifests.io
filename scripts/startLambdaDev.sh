#!/bin/bash

./scripts/runETL.sh

if [[ "$1" == "--redis" ]]; then
  docker run --name manifests-io-redis -p 6379:6379 -d redis
fi

cd lambda
python3 -m pip install -r requirements.txt
python3 -m pip install uvicorn

if [[ "$1" == "--redis" ]]; then
  REDIS_HOST="localhost" python3 -m uvicorn main:app --reload
  docker rm --force manifests-io-redis
else
  python3 -m uvicorn main:app --reload
fi