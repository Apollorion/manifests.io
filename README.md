# Manifests.io

[![Netlify Status](https://api.netlify.com/api/v1/badges/99772608-03b1-45d2-a943-8ee47bb82a8c/deploy-status)](https://app.netlify.com/sites/manifestsio/deploys)

Easy to use online kubernetes documentation.

## Important URLS
- Production: www.manifests.io


## Devving this repo
1. clone this repository
2. `cd manifests.io`
3. `yarn install`
4. `yarn dev`

You can support more kubernetes versions by dropping any k8s version's [Open API spec](https://github.com/kubernetes/kubernetes/blob/master/api/openapi-spec/swagger.json) into `oaspec/kubernetes` and updating `lib/oaspec.tsx`.

# Supporting new k8s/product versions

### New kubernetes versions
Kubernetes versions are easily updated.
1. Clone the kubernetes project `git clone git@github.com:kubernetes/kubernetes.git`
2. Change to the branch of the version in kubernetes project `git checkout release-1.22`
3. Copy the open API spec from the kubernetes project in `api/openapi-spec/swagger.json` to this project at `oaspec/kubernetes/<k8s_version>.json`
4. Add the version to the frontend application at `lib/oaspec.tsx`


### New CRD versions
CRDs copied into `./ETL/crds/*/*.yaml` will be combined into a product version based off the directory name under `./ETL/crds`
So for instance `./ETC/crds/certmanager-1.7/*.yaml` will generate a product name called `certmanager` for version `1.7` that supports any of the CRDs under its directory.

1. Create a new directory under `./ETL/crds/` and name it appropriately.
2. Copy all supported CRD YAML files under the new directory
3. Add the version to the frontend application at `./lib/oaspec.tsx`
4. Run the ETL script (python3 required) `yarn etl`
