#!/bin/bash

cd ETL
python3 main.py
cp ./k8s_versions/* ./dist/.
cp -r ./dist ../lambda/.
cd -