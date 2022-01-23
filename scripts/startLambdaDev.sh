#!/bin/bash

cd ETL
python3 main.py
cp -r ./dist ../lambda/.
cd -

docker run --name manifests-io-redis -p 6379:6379 -d redis

cd lambda
python3 -m pip install -r requirements.txt
python3 -m pip install uvicorn
REDIS_HOST="localhost" python3 -m uvicorn main:app --reload

docker rm --force manifests-io-redis