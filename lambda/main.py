from fastapi import FastAPI, HTTPException
from fastapi.exception_handlers import request_validation_exception_handler
from fastapi.exceptions import RequestValidationError
from mangum import Mangum
from fastapi.middleware.cors import CORSMiddleware
import redis
import os
import json
from sentry_sdk.integrations.asgi import SentryAsgiMiddleware
from sentry_sdk.integrations.aws_lambda import AwsLambdaIntegration
import sentry_sdk
import random
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

if "REDIS_HOST" in os.environ:
    r = redis.Redis(host=os.environ["REDIS_HOST"], port=6379, db=0, ssl_cert_reqs=None)

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


@app.get("/{k8s_version}/{search}")
def search_req(k8s_version, search):

    if k8s_version not in supported_versions:
        raise HTTPException(status_code=404, detail="K8s version not found.")

    # lowercase the entire search string
    search = expand_search_string(search)

    if "REDIS_HOST" in os.environ:
        redis_result = r.get(f"manifests.io:{k8s_version}:{search}")
        if redis_result:
            print("from cache")
            return json.loads(redis_result)

    f = open(f"./dist/{k8s_version}.json", "r")
    swagger = json.loads(f.read())
    f.close()

    result = get_result_from_swagger(search, swagger)

    if "REDIS_HOST" in os.environ:
        r.set(f"manifests.io:{k8s_version}:{search}", json.dumps(result))

    print("from disk")
    return result

@app.get("/keys/{k8s_version}/{search}")
def keys_req(k8s_version, search):
    if search == "":
        return []

    if "REDIS_HOST" in os.environ:
        r = redis.StrictRedis(host=os.environ["REDIS_HOST"], port=6379, db=0, ssl_cert_reqs=None)
        search = expand_search_string(search)

        scan = r.scan_iter(f"manifests.io:{k8s_version}:{search}*", count=500)
        new_scan = []

        for key in scan:
            new_scan.append(key.decode("utf-8").replace(f"manifests.io:{k8s_version}:", ""))

        if len(new_scan) > 10:
            new_scan = random.sample(new_scan, 10)

        return sorted(new_scan)
    else:
        return []

@app.get("/examples/{k8s_version}/{search}")
def examples_req(k8s_version, search):
    if search == "":
        raise HTTPException(status_code=404, detail=f"No example found.")

    search = expand_search_string(search)
    example = examples.get_example(k8s_version, search)
    if example:
        return example

    raise HTTPException(status_code=404, detail=f"No example found.")


def expand_search_string(search):
    search = search.lower().split(".")
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


def get_result_from_swagger(search, swagger):

    swagger = swagger["definitions"]
    search = search.split(".")
    original_resource = search[0]

    resource = get_resource(original_resource, swagger)
    if resource is None:
        raise HTTPException(status_code=404, detail=f"Resource {original_resource} not found.")

    # Return the resource if its all that was searched
    if len(search) == 1:
        return replace_top_level_refs(search, resource, swagger)
    else:

        # find item based off search term
        del search[0]
        for term in search:
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
            new_resource[i][key] = value

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


def get_resource(resource_search_term, swagger):
    # Find the resource
    for key, value in swagger.items():
        search_key = key.lower().split(".")[-1]
        if search_key == resource_search_term:
            return swagger[key]

    return None


handler = Mangum(app=app)
