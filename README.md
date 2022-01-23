# Manifests.io

Easy to use online kubernetes documentation.

## Devving this repo
1. clone this repository
2. `cd manifests.io`
3. `./scripts/startLambdaDev.sh` to start the API.
4. `./scripts/startReactDev.sh` to start the web app.

You can support more kubernetes versions by dropping any k8s version's [Open API spec](https://github.com/kubernetes/kubernetes/blob/master/api/openapi-spec/swagger.json) into `ETL/k8s_versions` and updating `app/k8s_details.js`.