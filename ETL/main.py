import sys
import os
from kubeconform import openapi2jsonschema

sys.setrecursionlimit(3500)

def main():

    crds = os.listdir("crds")
    if len(crds) > 0:
        for resource in crds:
            requests = os.listdir(f"./crds/{resource}")

            full_path_requests = []
            for request in requests:
                full_path_requests.append(f"./crds/{resource}/{request}")

            openapi2jsonschema.process_files(full_path_requests, f"./dist/{resource}.json")

if __name__ == "__main__":
    main()