#!/bin/bash

cd ETL
python3 main.py
cp -r ./dist ../lambda/.
cd -

cd lambda
python3 -m pip install -r requirements.txt
python3 -m pip install uvicorn
python3 -m uvicorn main:app --reload