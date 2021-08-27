from bs4 import BeautifulSoup, Tag
from fastapi import FastAPI
from mangum import Mangum
from fastapi.middleware.cors import CORSMiddleware
import redis
import os
import json

origins = [
    "https://manifests.io",
    "http://localhost:3000",
]

app = FastAPI(title='Manifests.io API', description='Reads k8s manifests and returns helpful documents')
app.add_middleware(
    CORSMiddleware,
    allow_origins=origins,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Save these into cache
page = open("apiRef.html", "r")
soup = BeautifulSoup(page.read(), "html.parser")
page_content = soup.find(id="page-content-wrapper")

r = redis.Redis(host=os.environ["REDIS_HOST"], port=6379, db=0)

@app.get("/{search}")
def search(search):
    original_search = search
    redis_result = r.get(f"manifest.io:{original_search}")
    if redis_result:
        print("from cache")
        return json.loads(redis_result)

    search = generate_correct_search(search)
    if search is not None:
        details = get_details(search)
        example = get_example(search)
        result = {"details": details, "example": example}
        r.set(f"manifest.io:{original_search}", json.dumps(result))
        print("from files")
        return result
    else:
        r.set(f"manifest.io:{original_search}", json.dumps(None))
        return None

def get_example(id):
    id = f"example-{id}"
    example = page_content.find(id=id)
    if example is not None:
        return example.find("code").text.strip()
    return None


def generate_correct_search(search):
    kinds = find_group_version_kind()
    search = search.split(".")

    id = None
    for query in search:
        try:
            gv = kinds[query]
            id = f"{query}-{gv['version']}-{gv['group']}"
        except KeyError:
            if id == None:
                return None
            else:
                details = get_details(id)
                if query in details:
                    obj = details[query]
                    if "href" in obj:
                        id = obj["href"]
                    else:
                        return None
                else:
                    return None
    return id

def get_details(id):
    header = page_content.find(id=id)

    # Find the second table, this gives us details about the thing
    next_sibling = header
    skip = True
    while True:
        next_sibling = next_sibling.nextSibling
        if next_sibling is None:
            break
        if isinstance(next_sibling, Tag):
            if next_sibling.name == "table":
                if next_sibling.has_attr("class"):
                    continue
                else:
                    break

    rows = next_sibling.find_all("tr")
    skip = True

    new_rows = {}
    for row in rows:
        if skip:
            skip = False
            continue
        tds = row.find_all("td")
        field = stripCodeElement(tds[0].find("code"))
        description = tds[1].text
        href = tds[0].find("a", href=True)
        if href is not None:
            href = href["href"].replace("#", "")

        new_rows[field] = {"description": description, "href": href}

    return new_rows

def find_group_version_kind():
    group_version_kinds = page_content.find_all("table", class_="col-md-8")

    kinds = {}

    for gvk in group_version_kinds:
        row = gvk.find_all("tr")[1]
        new_gvk = row.find_all("code")

        group = stripCodeElement(new_gvk[0])
        version = stripCodeElement(new_gvk[1])
        kind = stripCodeElement(new_gvk[2])

        kinds[kind.lower()] = {"group": group, "version": version}

    return kinds

def stripCodeElement(txt):
    if txt is not None:
        return txt.text

handler = Mangum(app=app)
