import json
import sys
import os

sys.setrecursionlimit(3500)

def main():

    files = os.listdir("k8s_versions")

    for file in files:

        print(f"Generating {file}...")

        f = open(f"./k8s_versions/{file}", "r")
        swagger = json.loads(f.read())
        f.close()

        definitions = swagger["definitions"]

        product = {}
        for rd in definitions:
            gvk = get_gvk_from_string(rd)
            value = parse_refs(definitions[rd], definitions)
            product[gvk["kind"].lower()] = {"gvk": gvk, "properties": value}

        filename = f"./dist/{file}"
        os.makedirs(os.path.dirname(filename), exist_ok=True)
        f = open(filename, "w")
        f.write(json.dumps(product))
        f.close()

def parse_refs(values, definitions):
    new = {}
    for value in values:
        item = values[value]
        if type(item) == dict:

            new_item = {}
            for k, v in item.items():
                if k == "$ref":
                    if "JSONSchemaProps" in v:
                        continue
                    new_item = {**new_item, **get_value_from_ref_link(v, definitions)}
                else:
                    new_item[k.lower()] = v

            # do this after getting ref material so we can parse the ref material for more refs
            item = parse_refs(new_item, definitions)

        new[value] = item
    return new

def get_value_from_ref_link(refLink, definitions):
    if type(refLink) == str:
        refLink = refLink.split("/")[::-1]
        refLink = refLink[0]
        return definitions[refLink]
    return refLink

def get_gvk_from_string(s):
    resource = s.split(".")[::-1]
    gvk = {
        "group": resource[2],
        "version": resource[1],
        "kind": resource[0]
    }
    return gvk

def print_keys(obj):
    for k, v in obj.items():
        print(k)

if __name__ == "__main__":
    main()