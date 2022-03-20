import os
import requests

REPLACE_TXT = "<meta name=\"ogmetadata\">"

# Lambda @ Edge cannot have environment variables
# So we set this up and replace the {{}} variables in github actions.
URL = os.environ["URL"] if "URL" in os.environ else "{{ URL }}"
API_URL = os.environ["API_URL"] if "API_URL" in os.environ else "{{ API_URL }}"

DEFAULT_TITLE = "Easy to read docs for kubernetes, istio, flux and other k8s things!"
DEFAULT_DESCRIPTION = "Easy Kubernetes Documentation!"
DEFAULT_IMAGE = f"{URL}ogimage.png"

# this needs to be all file extensions we use except `html`
KNOWN_FILE_EXTENSIONS = ["css", "js", "png", "json", "txt", "ico", "svg"]


def handler(event, context):
    try:
        return change_body(event)
    except:
        # never, ever fail.
        return event['Records'][0]['cf']['request']


def get_uri_parts(event):
    uri = event['Records'][0]['cf']['request']["uri"]

    file_extension = uri.split(".")[-1]
    if file_extension in KNOWN_FILE_EXTENSIONS:
        raise Exception("Not index.html")

    uri_parts = uri
    if uri_parts.endswith("/"):
        uri_parts = uri_parts[:-1]
    uri_parts = uri_parts.split("/")
    uri_parts.pop(0)

    return uri_parts


def change_body(event):
    uri_parts = get_uri_parts(event)

    product = ""
    version = ""
    if len(uri_parts) > 0:
        productVersion = uri_parts[0].split("-")
        product = productVersion[0]
        version = productVersion[1]

    url = f"{URL}" + "/".join(uri_parts)
    if len(uri_parts) == 0:
        # Default
        new_content = generate_html(DEFAULT_TITLE, URL, DEFAULT_DESCRIPTION, DEFAULT_IMAGE)
    if len(uri_parts) == 1:
        # OG Data for URLs that only include productVersion
        title = f"Easy kubernetes documentation for {product} v{version}."
        new_content = generate_html(title, url, DEFAULT_DESCRIPTION, DEFAULT_IMAGE)
    elif len(uri_parts) >= 2:
        # OG Data for URLs that include productVersion and fieldPath
        r = requests.get(API_URL + "search/" + "/".join(uri_parts))
        result = r.json()
        description = result["description"]

        if "More info:" in description:
            description = description.split("More info:")
            description = description[0]

        title = f"{uri_parts[1]} - {product} v{version}"

        new_content = generate_html(title, url, description, DEFAULT_IMAGE)

    return good_response(new_content)


def good_response(body):
    return {
        'status': '200',
        'statusDescription': 'OK',
        'headers': {
            'cache-control': [
                {
                    'key': 'Cache-Control',
                    'value': 'max-age=86400'
                }
            ],
            "content-type": [
                {
                    'key': 'Content-Type',
                    'value': 'text/html'
                }
            ]
        },
        'body': body
    }


def generate_html(title, url, description, image):
    f = open(f"{os.path.dirname(os.path.realpath(__file__))}/index.html", "r")
    content = f.read()
    f.close()

    opengraph_html = f"<meta property=\"og:title\" content=\"{title}\"><meta property=\"og:site_name\" content=\"Manifests.io\"><meta property=\"og:url\" content=\"{url}\"><meta property=\"og:description\" content=\"{description}\"><meta property=\"og:image\" content=\"{image}\">"
    return content.replace(REPLACE_TXT, opengraph_html)
