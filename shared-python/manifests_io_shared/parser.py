def get_result_from_disk(search, swagger):
    resource, field_path = get_resource_and_field_path_from_search_string(search)

    if resource not in swagger:
        raise PassableHTTPException(status_code=404, detail=f"Resource {resource} not found.", search=search)

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
                    raise PassableHTTPException(status_code=404, detail=f"Field path {'.'.join(field_path)} not found in resource {resource}.", search=search)
            else:
                raise PassableHTTPException(status_code=404, detail=f"Field path {'.'.join(field_path)} not found in resource {resource}.", search=search)

    if len(field_path) == 0 and "gvk" in rd:
        result = {"gvk": rd["gvk"], **result}

    if description is not None:
        result = {"description": description, **result}

    return result


class PassableHTTPException(Exception):
    def __init__(self, status_code=None, detail=None, search=None):
        self.status_code = status_code
        self.detail = detail
        self.search = search
        super().__init__(self.detail)


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