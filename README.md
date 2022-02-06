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

# Examples
Examples are all maintained outside of this repo and are pulled down on the fly.
A dictionary of examples is in `lambda/examples.py`.

## Adding or modifying an example

Examples are found via the k8s/product version as a top key in the examples dictionary.  
This example would support k8s 1.20 and certmanager-1.7
```
examples = {
  "1.20"...
  "certmanager-1.7"...
}
```

Docs can be referenced via a ref that leads to a link (refs are only parsed 1 ref down, no recursion) or directly to a link.  
This would reference the `common` product and would look for a key called `pod`.  
`"pod": {"$ref": "common#pod"}`

This would directly reference an example.  
`pod: {"url": "https://myamazingpod.com/example.yaml", "text": "myamazingpod", "source": "https://myamazingpod.com"}`  
`url` attribute is a url to the yaml that will be displayed to the user.  
`text` is the text displayed in the frontend application as the source of the example  
`source` is the url the text will link to  

If the url needs to reference a specific branch, you can use `$BRANCH` within the `url`.  
The `url` in the below example would resolve to `https://myamazingpod.com/my-awesome-branch/example.yaml`.  
This is useful when using github raw content and the same path can support multiple different versions.
```python
example = {
  "1.20": {
    "branch": "my-awesome-branch",
    "docs": {
      "pod": {"url": "https://myamazingpod.com/$BRANCH/example.yaml", "text": "myamazingpod", "source": "https://myamazingpod.com"}
    }
  }
}
```

