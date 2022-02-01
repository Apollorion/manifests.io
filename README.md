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
3. `./scripts/startLambdaDev.sh` to start the API. (to test with redis `./scripts/startLambdaDev.sh --redis`)
4. `./scripts/startReactDev.sh` to start the web app.

You can support more kubernetes versions by dropping any k8s version's [Open API spec](https://github.com/kubernetes/kubernetes/blob/master/api/openapi-spec/swagger.json) into `ETL/k8s_versions` and updating `app/k8s_details.js`.

# Supporting new k8s/product versions

### New kubernetes versions
Kubernetes versions are easily updated.
1. Clone the kubernetes project `git clone git@github.com:kubernetes/kubernetes.git`
2. Change to the branch of the version in kubernetes project `git checkout release-1.22`
3. Copy the open API spec from the kubernetes project in `api/openapi-spec/swagger.json` to this project at `ETL/k8s_versions/<k8s_version>.json`
4. Add the version to the frontend application at `./app/src/k8s_details.js`


### New CRD versions
CRDs copied into `./ETL/crds/*/*.yaml` will be combined into a product version based off the directory name under `./ETL/crds`
So for instance `./ETC/crds/certmanager-1.7/*.yaml` will generate a product name called `certmanager-1.7` that supports any of the CRDs under its directory.

1. Create a new directory under `./ETL/crds/` and name it appropriately.
2. Copy all supported CRD YAML files under the new directory
3. Add the version to the frontend application at `./app/src/k8s_details.js` (use the directory name)