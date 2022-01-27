import json
import redis
import os
from manifests_io_shared import parser

ignored = [
    "oname",
    "gvk",
    "group",
    "version",
    "kind",
    "type",
    "required",
    "description",
    "x-kubernetes-map-type",
    "x-kubernetes-patch-merge-key",
    "x-kubernetes-patch-strategy"
]

if "REDIS_HOST" in os.environ:
    r = redis.Redis(host=os.environ["REDIS_HOST"], port=6379, db=0, ssl_cert_reqs=None)


def main():
    files = os.listdir("dist")

    for file in files:
        f = open(f"./dist/{file}", "r")
        swagger = json.loads(f.read())
        f.close()

        k8s_version = file.replace(".json", "")
        for resource in swagger:
            rd = swagger[resource]["properties"]
            redis_key = f"manifests.io:{k8s_version}:{resource}"
            walk_spec(rd, redis_key, swagger, k8s_version)

        if "REDIS_HOST" in os.environ:
            print(f"loading: manifests.io:{k8s_version}.json")
            r.set(f"manifests.io:{k8s_version}.json", json.dumps(swagger))


def walk_spec(rd, start_key, swagger, k8s_version):
    if "properties" in rd:
        for key in rd["properties"]:
            if key in ignored:
                continue

            if key == "properties":
                walk_spec(rd["properties"], start_key, swagger, k8s_version)
                continue

            if type(rd["properties"][key]) == dict:
                redis_key = f"{start_key}.{key}"

                # Load the property into redis
                load_key = redis_key
                try:
                    load_value = parser.get_result_from_disk(load_key.replace(f"manifests.io:{k8s_version}:", ""), swagger)

                    if "REDIS_HOST" in os.environ:
                        print(f"loading: {load_key}")
                        r.set(load_key, json.dumps(load_value))
                    else:
                        print(load_key)
                except parser.PassableHTTPException as e:
                    # generating all the keys isnt perfect
                    # if something 404's thats okay to ignore
                    if e.status_code != 404:
                        print(f"unable to find {e.search}")
                        raise e
                    else:
                        print("404")

                walk_spec(rd["properties"][key], redis_key, swagger, k8s_version)


def lambda_handler(a, b):
    main()


if __name__ == "__main__":
    main()
