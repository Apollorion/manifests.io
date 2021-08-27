#!/bin/bash

rm payload.zip || true

mkdir package
cd package
cp ../requirements.txt .
cp ../main.py .
cp ../apiRef.html .

docker run -it --rm -v $PWD:/var/task lambci/lambda:build-python3.8 pip install --target ./ -r requirements.txt
zip -r payload.zip .
mv payload.zip ../
cd ../
rm -rf ./package/