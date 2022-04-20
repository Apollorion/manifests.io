from fastapi import FastAPI, HTTPException
from fastapi.exception_handlers import request_validation_exception_handler
from fastapi.exceptions import RequestValidationError
from mangum import Mangum
from fastapi.middleware.cors import CORSMiddleware
import os
import json
from sentry_sdk.integrations.asgi import SentryAsgiMiddleware
from sentry_sdk.integrations.aws_lambda import AwsLambdaIntegration
import sentry_sdk
import examples

app = FastAPI(title='Manifests.io API', description='Reads k8s manifests and returns helpful documents')
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

if "SENTRY_INGEST" in os.environ:
    sentry_sdk.init(
        os.environ["SENTRY_INGEST"],
        traces_sample_rate=0.05,
        integrations=[AwsLambdaIntegration()],
        ignore_errors=[HTTPException]
    )

    app.add_middleware(SentryAsgiMiddleware)

files = os.listdir("dist")
supported_versions = []
for file in files:
    supported_versions.append(file.replace(".json", ""))


@app.exception_handler(RequestValidationError)
async def validation_exception_handler(request, e):
    if "SENTRY_INGEST" in os.environ:
        with sentry_sdk.configure_scope() as scope:
            scope.set_context("request", request)
            sentry_sdk.capture_exception(e)
    return await request_validation_exception_handler(request, e)


@app.get("/search/{k8s_version}/{search}")
def search_req(k8s_version, search):
    return get_search_result(k8s_version, search)


@app.get("/search/{k8s_version}/{search}/{version}")
def searchversion_req(k8s_version, search, version):
    return get_search_result(k8s_version, search, version)


def get_search_result(k8s_version, search, version=False):
    if k8s_version not in supported_versions:
        raise HTTPException(status_code=404, detail="K8s version not found.")

    search = expand_search_string(search)

    f = open(f"./dist/{k8s_version}.json", "r")
    swagger = json.loads(f.read())
    f.close()

    result = get_result_from_swagger(search, swagger, wanted_version=version)
    return result


@app.get("/examples/{k8s_version}/{search}")
def examples_req(k8s_version, search):
    if search == "":
        raise HTTPException(status_code=404, detail=f"No example found.")

    search = expand_search_string(search)
    example = examples.get_example(k8s_version, search)
    if example:
        return example

    raise HTTPException(status_code=404, detail=f"No example found.")


@app.get("/keys/{k8s_version}/{search}")
def keys_req(k8s_version, search):
    if search == "":
        return []

    search = expand_search_string(search)
    f = open(f"./dist/{k8s_version}.json", "r")
    swagger = json.loads(f.read())
    f.close()

    # Include original search in the result (if the result is greater than 0) and sort alphabetically
    result = get_next_keys(search, swagger)
    if len(result) > 0:
        result = [search] + result
        result.sort()

    return result


@app.get("/resourceversions/{k8s_version}/{search}")
def resourceversions_req(k8s_version, search):
    if search == "":
        return []

    search = expand_search_string(search)
    f = open(f"./dist/{k8s_version}.json", "r")
    swagger = json.loads(f.read())
    f.close()

    versions = []
    wanted_kind = search.split(".")[-1]
    for key, value in swagger["definitions"].items():
        gvk = get_gvk_from_resource_key(key)
        kind = gvk["kind"]
        if kind.lower() == wanted_kind.lower():
            if "version" in gvk:
                versions.append(gvk["version"])
    return versions


@app.get("/resources/{k8s_version}")
def resources_req(k8s_version):
    if k8s_version not in supported_versions:
        raise HTTPException(status_code=404, detail="K8s version not found.")

    f = open(f"./dist/{k8s_version}.json", "r")
    swagger = json.loads(f.read())
    f.close()

    items = []
    keys = []
    for key, value in swagger["definitions"].items():
        key = key.split(".")[-1]
        # The k8s openapi spec lists all resources (even sub resources) at the root level.
        # This was the best way I could think to get only a list of resources people will actually
        # care about. Im sure there might be a smarter way to do this, but this works for now.
        if key not in keys and "properties" in value and "spec" in value["properties"]:
            keys.append(key)
            description = "" if "description" not in value else value["description"]
            items.append({"resource": key, "description": description})

    items = sorted(items, key=lambda x: x["resource"])

    return items


def get_next_keys(search, swagger, min_requested=10):
    try:
        result = get_result_from_swagger(search, swagger)
    except HTTPException:
        return []

    keys = []
    if "properties" in result:
        for key in result["properties"].keys():
            keys.append(f"{search}.{key}")

    if len(keys) < min_requested and "properties" in result:
        for key, value in result["properties"].items():
            if "properties" in value:
                keys = keys + get_next_keys(f"{search}.{key}", swagger, min_requested - len(keys))

    return keys


def expand_search_string(search):
    search = search.split(".")
    search[0] = expand_shortened_resource_name(search[0])
    return '.'.join(search)


def expand_shortened_resource_name(resource):
    shortened_map = {
        "csr": "certificatesigningrequest",
        "cs": "componentstatus",
        "cm": "configmap",
        "ds": "daemonset",
        "deploy": "deployment",
        "ep": "endpoint",
        "ev": "event",
        "hpa": "horizontalpodautoscaler",
        "ing": "ingress",
        "limits": "limitrange",
        "ns": "namespace",
        "no": "node",
        "pvc": "persistentvolumeclaim",
        "pv": "persistentvolume",
        "po": "pod",
        "pdb": "poddisruptionbudget",
        "psp": "podsecuritypolicy",
        "rs": "replicaset",
        "rc": "replicationcontroller",
        "quota": "resourcequota",
        "sa": "serviceaccount",
        "svc": "service"
    }

    if resource in shortened_map:
        return shortened_map[resource]
    return resource


def get_result_from_swagger(search, swagger, wanted_version=False):

    swagger = swagger["definitions"]
    search = search.split(".")
    original_resource = search[0].lower()

    # We only set wanted version here, because the resource we get from here
    # will have subresources versioned within its spec automatically.
    resource = get_resource(original_resource, swagger, wanted_version)
    if resource is None:
        raise HTTPException(status_code=404, detail=f"Resource {original_resource} not found.")

    # find item based off search term
    del search[0]
    for term in search:
        if "items" in resource:
            resource["properties"] = resource["items"]["properties"]
            del resource["items"]

        resource = get_resource(term, resource["properties"])
        if resource is None:
            raise HTTPException(status_code=404, detail=f"FieldPath {'.'.join(search)} not found in {original_resource}.")

        hold = get_hold_object(resource)
        if "$ref" in resource or ("items" in resource and "$ref" in resource["items"]):
            if "items" in resource:
                resource = {**get_next_resource(search, resource["items"]["$ref"], swagger), **hold}
            else:
                resource = {**get_next_resource(search, resource["$ref"], swagger), **hold}

    return replace_top_level_refs(search, resource, swagger)


def replace_top_level_refs(search, resource, swagger):
    i = "properties"
    if "items" in resource:
        i = "items"

    if i not in resource:
        return resource

    new_resource = resource.copy()
    new_resource[i] = {}

    for key, value in resource[i].items():

        hold = get_hold_object(value)
        if "$ref" in value or ("items" in value and "$ref" in value["items"]):
            if "items" in value:
                next_resource = {**get_next_resource(search, value["items"]["$ref"], swagger), **hold}
            else:
                next_resource = {**get_next_resource(search, value["$ref"], swagger), **hold}
            new_resource[i][key] = next_resource
        else:
            if "items" in value:
                value = value["items"]
                value["type"] = "array"
            new_resource[i][key] = value

    if "items" in new_resource:
        new_resource["properties"] = new_resource["items"]["properties"]
        del new_resource["items"]

    return new_resource


def get_hold_object(resource):
    # We need to "hold" the description and type of the original resource
    # so it doesnt get overwritten in the documentation
    hold = {}
    if "description" in resource:
        hold["description"] = resource["description"]
    if "type" in resource:
        hold["type"] = resource["type"]
    if "x-kubernetes-group-version-kind" in resource:
        hold["x-kubernetes-group-version-kind"] = resource["x-kubernetes-group-version-kind"]
    if "enum" in resource:
        hold["enum"] = resource["enum"]

    return hold


def get_next_resource(search, key, swagger):
    key = key.replace("#/definitions/", "")
    if key in swagger:
        subtype = key.split(".")[::-1][0]
        return {**swagger[key], "subtype": subtype}
    else:
        raise HTTPException(status_code=404, detail=f"FieldPath {'.'.join(search)} not found.")


def get_resource(wanted_kind, swagger, wanted_version=False):
    # Find the resource
    for key, value in swagger.items():
        gvk = get_gvk_from_resource_key(key)
        kind = gvk["kind"]
        if kind.lower() == wanted_kind.lower():
            resource = {**swagger[key], **{"gvk": gvk}}
            if wanted_version:
                if "version" in gvk and gvk["version"] == wanted_version:
                    return resource
            else:
                return resource

    return None


def get_gvk_from_resource_key(key):
    arr = key.split(".")
    if len(arr) > 2:
        kind = arr[-1]
        del arr[-1]
        version = arr[-1]
        del arr[-1]
        arr.reverse()
        group = ".".join(arr)

        if group == "core.api.k8s.io":
            group = ""

        return {
            "group": group,
            "version": version,
            "kind": kind
        }

    elif len(arr) == 2:
        kind = arr[-1]
        del arr[-1]
        version = arr[-1]

        return {
            "version": version,
            "kind": kind
        }

    else:
        return {
            "kind": arr[0]
        }


handler = Mangum(app=app)
