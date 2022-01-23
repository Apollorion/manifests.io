# Manifests.io

[![main](https://github.com/Apollorion/manifests.io/actions/workflows/main.yml/badge.svg)](https://github.com/Apollorion/manifests.io/actions/workflows/main.yml)

Easy to use online kubernetes documentation.

## Important URLS
- Production
  - app: manifests.io, www.manifests.io
  - api: api.manifests.io
- Staging
  - app: stage.manifests.io, www.stage.manifests.io
  - api: api.stage.manifests.io

## Devving this repo
1. clone this repository
2. `cd manifests.io`
3. `./scripts/startLambdaDev.sh` to start the API.
4. `./scripts/startReactDev.sh` to start the web app.

You can support more kubernetes versions by dropping any k8s version's [Open API spec](https://github.com/kubernetes/kubernetes/blob/master/api/openapi-spec/swagger.json) into `ETL/k8s_versions` and updating `app/k8s_details.js`.