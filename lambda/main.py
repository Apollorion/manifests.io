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
from manifests_io_shared import parser
import random

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
        redis_result = r.get(f"manifests.io:{k8s_version}:{search}")
        if redis_result:
            print("from cache")
            return json.loads(redis_result)

    f = open(f"./dist/{k8s_version}.json", "r")
    swagger = json.loads(f.read())
    f.close()

    try:
        result = parser.get_result_from_disk(search, swagger)
    except parser.PassableHTTPException as e:
        raise HTTPException(status_code=e.status_code, detail=e.detail)

    if "REDIS_HOST" in os.environ:
        r.set(f"manifests.io:{k8s_version}:{search}", json.dumps(result))

    print("from disk")
    return result

@app.get("/keys/{k8s_version}/{search}")
def search(k8s_version, search):
    if search == "":
        return []

    if "REDIS_HOST" in os.environ:
        r = redis.StrictRedis(host=os.environ["REDIS_HOST"], port=6379, db=0, ssl_cert_reqs=None)
        search = search.lower()
        scan = r.scan_iter(f"manifests.io:{k8s_version}:{search}*", count=500)
        new_scan = []

        for key in scan:
            new_scan.append(key.decode("utf-8").replace(f"manifests.io:{k8s_version}:", ""))

        if len(new_scan) > 10:
            new_scan = random.sample(new_scan, 10)

        return sorted(new_scan)
    else:
        return []



handler = Mangum(app=app)
