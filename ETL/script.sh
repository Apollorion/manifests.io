#!/usr/bin/env bash

# check if pyyaml is installed
if ! python3 -c "import yaml" &> /dev/null; then
    echo "pyyaml is not installed. Installing..."
    python3 -m pip install pyyaml
fi

cd ETL
python3 main.py
cd -
echo "ETL script finished"