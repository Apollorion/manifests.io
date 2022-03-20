#!/bin/bash

if [ ! -f "./lambda@edgefunction/.built" ]; then
    python3 -m pip install -r ./lambda@edgefunction/requirements.txt
    touch ./lambda@edgefunction/.built
fi

export URL="https://manifests.local/"
export API_URL="http://localhost:8000/"

python3 ./lambda@edgefunction/local.py