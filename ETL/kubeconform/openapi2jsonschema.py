#!/usr/bin/env python3

# Derived from https://github.com/instrumenta/openapi2jsonschema
import yaml
import json
import sys
import os

def replace_int_or_string(data):
    new = {}
    try:
        for k, v in iter(data.items()):
            new_v = v
            if isinstance(v, dict):
                if "format" in v and v["format"] == "int-or-string":
                    new_v = {"oneOf": [{"type": "string"}, {"type": "integer"}]}
                else:
                    new_v = replace_int_or_string(v)
            elif isinstance(v, list):
                new_v = list()
                for x in v:
                    new_v.append(replace_int_or_string(x))
            else:
                new_v = v
            new[k] = new_v
        return new
    except AttributeError:
        return data

def write_schema_file(schema, filename):
    schemaJSON = json.dumps(schema, indent=2)

    # Dealing with user input here..
    filename = os.path.basename(filename)
    f = open(filename, "w")
    print(schemaJSON, file=f)
    f.close()
    print("JSON schema written to {filename}".format(filename=filename))


def process_files(files, destination):

    new_crds = {}
    for crdFile in files:
        f = open(crdFile)
        with f:
            defs = []
            for y in yaml.load_all(f, Loader=yaml.SafeLoader):
                if y is None:
                    continue
                if "items" in y:
                    defs.extend(y["items"])
                if "kind" not in y:
                    continue
                if y["kind"] != "CustomResourceDefinition":
                    continue
                else:
                    defs.append(y)

            for y in defs:
                if "spec" in y and "versions" in y["spec"] and y["spec"]["versions"]:
                    for version in y["spec"]["versions"]:
                        if "schema" in version and "openAPIV3Schema" in version["schema"]:
                            gvk_kind = y["spec"]["names"]["kind"]
                            gvk_group = y["spec"]["group"].split(".")[0]
                            gvk_version = version["name"]

                            schema = version["schema"]["openAPIV3Schema"]
                            schema = replace_int_or_string(schema)
                            new_crds[f"{gvk_group}.{gvk_version}.{gvk_kind}"] = schema
                        elif "validation" in y["spec"] and "openAPIV3Schema" in y["spec"]["validation"]:
                            gvk_kind = y["spec"]["names"]["kind"]
                            gvk_group = y["spec"]["group"].split(".")[0]
                            gvk_version = version["name"]

                            schema = y["spec"]["validation"]["openAPIV3Schema"]
                            schema = replace_int_or_string(schema)
                            new_crds[f"{gvk_group}.{gvk_version}.{gvk_kind}"] = schema
                elif "spec" in y and "validation" in y["spec"] and "openAPIV3Schema" in y["spec"]["validation"]:
                    gvk_kind = y["spec"]["names"]["kind"]
                    gvk_group = y["spec"]["group"].split(".")[0]
                    gvk_version = version["name"]

                    schema = y["spec"]["validation"]["openAPIV3Schema"]
                    schema = replace_int_or_string(schema)
                    new_crds[f"{gvk_group}.{gvk_version}.{gvk_kind}"] = schema

    os.makedirs(os.path.dirname(destination), exist_ok=True)
    f = open(destination, "w")
    f.write(json.dumps({"definitions": new_crds}))
    f.close()
