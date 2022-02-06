#!/bin/bash

./scripts/runETL.sh

cd lambda
python3 -m pip install -r requirements.txt
python3 -m pip install uvicorn

python3 -m uvicorn main:app --reload
