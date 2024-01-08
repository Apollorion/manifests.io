#!/usr/bin/env python3

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


def process_crd(crdFile):
    new_crds = {}
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

                        gvk_group = y["spec"]["group"].split(".")
                        gvk_group.reverse()
                        gvk_group = ".".join(gvk_group)

                        gvk_version = version["name"]

                        schema = version["schema"]["openAPIV3Schema"]
                        schema = replace_int_or_string(schema)
                        schema["x-kubernetes-group-version-kind"] = [{"group": gvk_group, "version": gvk_version, "kind": gvk_kind}]

                        if "description" not in schema:
                            schema["description"] = ""

                        for key in schema["properties"]:
                            if "description" not in schema["properties"][key]:
                                schema["properties"][key]["description"] = ""


                        new_crds[f"{gvk_group}.{gvk_version}.{gvk_kind}"] = schema
                    elif "validation" in y["spec"] and "openAPIV3Schema" in y["spec"]["validation"]:
                        gvk_kind = y["spec"]["names"]["kind"]

                        gvk_group = y["spec"]["group"].split(".")
                        gvk_group.reverse()
                        gvk_group = ".".join(gvk_group)

                        gvk_version = version["name"]

                        schema = y["spec"]["validation"]["openAPIV3Schema"]
                        schema = replace_int_or_string(schema)
                        schema["x-kubernetes-group-version-kind"] = [{"group": gvk_group, "version": gvk_version, "kind": gvk_kind}]
                        if "description" not in schema:
                            schema["description"] = ""

                        for key in schema["properties"]:
                            if "description" not in schema["properties"][key]:
                                schema["properties"][key]["description"] = ""

                        new_crds[f"{gvk_group}.{gvk_version}.{gvk_kind}"] = schema
            elif "spec" in y and "validation" in y["spec"] and "openAPIV3Schema" in y["spec"]["validation"]:
                gvk_kind = y["spec"]["names"]["kind"]

                gvk_group = y["spec"]["group"].split(".")
                gvk_group.reverse()
                gvk_group = ".".join(gvk_group)

                gvk_version = version["name"]

                schema = y["spec"]["validation"]["openAPIV3Schema"]
                schema = replace_int_or_string(schema)
                schema["x-kubernetes-group-version-kind"] = [{"group": gvk_group, "version": gvk_version, "kind": gvk_kind}]
                if "description" not in schema:
                    schema["description"] = ""

                for key in schema["properties"]:
                    if "description" not in schema["properties"][key]:
                        schema["properties"][key]["description"] = ""

                new_crds[f"{gvk_group}.{gvk_version}.{gvk_kind}"] = schema
    return new_crds

def remove_props(crds):
    new_crds = {}
    deletes = []
    found_update = False
    for key in crds:
        crd = crds[key]
        if "properties" in crd or "items" in crd:
            if type(crd) is str:
                deletes.append(key)
                continue
            crdThing = "properties" if "properties" in crd else "items"
            for prop in crd[crdThing]:
                if isinstance(crd[crdThing][prop], (int, float)):
                    continue
                if "manifests_processed" in crd[crdThing][prop]:
                    continue
                if "properties" in crd[crdThing][prop] or "items" in crd[crdThing][prop]:
                    new_crds[key + prop.capitalize()] = crds[key][crdThing][prop]
                    thisThing = "properties" if "properties" in crds[key][crdThing][prop] else "items"
                    if thisThing == "items":
                        crds[key][crdThing][prop] = {
                            "type": "array",
                            "items": {
                                "$ref": "#/definitions/" + key + prop.capitalize(),
                            },
                            "description": "" if "description" not in crds[key][crdThing][prop] else crds[key][crdThing][prop]["description"],
                            "manifests_processed": True
                        }
                    else:
                        crds[key][crdThing][prop] = {
                            "$ref": "#/definitions/" + key + prop.capitalize(),
                            "description": "" if "description" not in crds[key][crdThing][prop] else crds[key][crdThing][prop]["description"]
                        }
                    found_update = True
                    print(key + prop.capitalize() + " added")

    crds.update(new_crds)
    for d in deletes:
        del crds[d]
    if found_update:
        print("going again")
        crds = remove_props(crds)

    return crds
def process_files(files, destination):
    new_crds = {}
    for crdFile in files:
        results = process_crd(crdFile)
        new_crds.update(remove_props(results))

    os.makedirs(os.path.dirname(destination), exist_ok=True)
    f = open(destination, "w")
    f.write(json.dumps({"definitions": new_crds}))
    f.close()
    print("done")


# read the crds directory and loop for all directories inside
crds = os.listdir("crds")
if len(crds) > 0:
    for resource in crds:
        requests = os.listdir(f"./crds/{resource}")

        full_path_requests = []
        for request in requests:
            full_path_requests.append(f"./crds/{resource}/{request}")

        item, version = resource.split("-")

        print(f"Processing {resource}...")
        process_files(full_path_requests, f"../oaspec/{item}/{version}.json")


print("done with all processes")

