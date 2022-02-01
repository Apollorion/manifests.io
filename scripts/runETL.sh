#!/bin/bash

cd ETL
mkdir processable
cp k8s_versions/* processable/.

python3 main.py
cp -r ./dist ../lambda/.
cd -