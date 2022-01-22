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
                    gvk = get_gvk_from_string(v)
                    if gvk is not None:
                        new_item = {"gvk": gvk, **new_item, **get_value_from_ref_link(v, definitions)}
                    else:
                        new_item = {**new_item, **get_value_from_ref_link(v, definitions)}
                else:

                    # items just differentiates arrays from objects
                    # we can tell the difference with "type" so lets just make it easy
                    # and list everything under "properties"
                    k = k.lower()
                    if k == "items":
                        k = "properties"

                    new_item[k.lower()] = v

            # do this after getting ref material so we can parse the ref material for more refs
            item = parse_refs(new_item, definitions)

        new[value.lower()] = item
    return new

def get_gvk_from_ref_link(ref_link):
    if type(ref_link) == str:
        ref_link = ref_link.split("/")[::-1]
        ref_link = ref_link[0]
        return get_gvk_from_string(ref_link)
    return None


def get_value_from_ref_link(ref_link, definitions):
    if type(ref_link) == str:
        ref_link = ref_link.split("/")[::-1]
        ref_link = ref_link[0]
        return definitions[ref_link]
    return ref_link

def get_gvk_from_string(s):
    if type(s) == str:
        resource = s.split(".")[::-1]
        gvk = {
            "group": resource[2],
            "version": resource[1],
            "kind": resource[0]
        }
        return gvk
    return None

def print_keys(obj):
    for k, v in obj.items():
        print(k)

if __name__ == "__main__":
    main()