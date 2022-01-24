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


@app.exception_handler(RequestValidationError)
async def validation_exception_handler(request, e):
    if "SENTRY_INGEST" in os.environ:
        with sentry_sdk.configure_scope() as scope:
            scope.set_context("request", request)
            sentry_sdk.capture_exception(e)
    return await request_validation_exception_handler(request, e)


@app.get("/{k8s_version}/{search}")
def search(k8s_version, search):
    r = None
    if "REDIS_HOST" in os.environ:
        r = redis.Redis(host=os.environ["REDIS_HOST"], port=6379, db=0, ssl_cert_reqs=None)

    files = os.listdir("dist")
    supported_versions = []
    for file in files:
        supported_versions.append(file.replace(".json", ""))

    if k8s_version not in supported_versions:
        raise HTTPException(status_code=404, detail="K8s version not found.")

    # lowercase the entire search string
    search = search.lower()
    if "REDIS_HOST" in os.environ:
        redis_result = r.get(f"manifest.io:{k8s_version}:{search}")
        if redis_result:
            print("from cache")
            return json.loads(redis_result)

    f = open(f"./dist/{k8s_version}.json", "r")
    swagger = json.loads(f.read())
    f.close()

    resource, field_path = get_resource_and_field_path_from_search_string(search)

    if resource not in swagger:
        raise HTTPException(status_code=404, detail=f"Resource {resource} not found.")

    rd = swagger[resource]
    result = rd["properties"]
    description = None
    for path in field_path:
        description = None

        # get properties of the resource definition
        if path in result:
            rd = result[path]
            result = get_result(rd)
        else:
            if "properties" in result:
                try:
                    result = get_result(result)[path]
                    if "description" in result:
                        description = result["description"]
                    if "gvk" not in result:
                        result = get_result(result)
                        if "description" in result:
                            description = result["description"]
                except KeyError:
                    raise HTTPException(status_code=404, detail=f"Field path {'.'.join(field_path)} not found in resource {resource}.")
            else:
                raise HTTPException(status_code=404, detail=f"Field path {'.'.join(field_path)} not found in resource {resource}.")

    if len(field_path) == 0 and "gvk" in rd:
        result = {"gvk": rd["gvk"], **result}

    if description is not None:
        result = {"description": description, **result}

    if "REDIS_HOST" in os.environ:
        r.set(f"manifest.io:{k8s_version}:{search}", json.dumps(result))

    print("from disk")
    return result


def get_result(rd):
    if "properties" in rd:
        result = rd["properties"]
    else:
        result = rd
    return result


def get_resource_and_field_path_from_search_string(s):
    items = s.split(".")
    resource = items[0]
    field_path = items[1:]
    return resource, field_path


handler = Mangum(app=app)
