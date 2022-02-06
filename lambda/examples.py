import requests

URL = "https://raw.githubusercontent.com/kubernetes/website"

EXAMPLES = {
    "common": {
        "docs": {
            "pod": {"url": "$URL/$BRANCH/content/en/examples/pods/simple-pod.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "pod.spec.initContainers": {"url": "$URL/$BRANCH/content/en/examples/pods/init-containers.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "pod.spec.containers.lifecycle": {"url": "$URL/$BRANCH/content/en/examples/pods/lifecycle-events.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "pod.spec.containers.env.valuefrom": {"url": "$URL/$BRANCH/content/en/examples/pods/pod-configmap-env-var-valueFrom.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "pod.spec.containers.envfrom": {"url": "$URL/$BRANCH/content/en/examples/pods/pod-configmap-envFrom.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "pod.spec.volumes.configmap.items": {"url": "$URL/$BRANCH/content/en/examples/pods/pod-configmap-volume-specific-key.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "pod.spec.volumes.configmap": {"url": "$URL/$BRANCH/content/en/examples/pods/pod-configmap-volume.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "pod.spec.affinity.nodeaffinity": {"url": "$URL/$BRANCH/content/en/examples/pods/pod-with-node-affinity.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "pod.spec.tolerations": {"url": "$URL/$BRANCH/content/en/examples/pods/pod-with-toleration.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "pod.spec.imagepullsecrets": {"url": "$URL/$BRANCH/content/en/examples/pods/private-reg-pod.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "cronjob": {"url": "$URL/$BRANCH/content/en/examples/application/job/cronjob.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "configmap": {"url": "$URL/$BRANCH/content/en/examples/configmap/configmaps.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "daemonset": {"url": "$URL/$BRANCH/content/en/examples/controllers/daemonset.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "replicaset": {"url": "$URL/$BRANCH/content/en/examples/controllers/replicaset.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "horizontalpodautoscaler": {"url": "$URL/$BRANCH/content/en/examples/controllers/hpa-rs.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "job": {"url": "$URL/$BRANCH/content/en/examples/controllers/job.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
            "service": {"url": "$URL/$BRANCH/content/en/examples/service/nginx-service.yaml", "text": "kubernetes/website", "source": "https://github.com/kubernetes/website"},
        }
    },
    "1.23": {
        "branch": "main",
        "docs": {
            "pod": {"$ref": "common#pod"},
            "pod.spec": {"$ref": "common#pod"},
            "pod.spec.initContainers": {"$ref": "common#pod.spec.initContainers"},
            "pod.spec.containers.lifecycle": {"$ref": "common#pod.spec.containers.lifecycle"},
            "pod.spec.containers.env.valuefrom": {"$ref": "common#pod.spec.containers.env.valuefrom"},
            "pod.spec.containers.envfrom": {"$ref": "common#pod.spec.containers.envfrom"},
            "pod.spec.volumes.configmap.items": {"$ref": "common#pod.spec.volumes.configmap.items"},
            "pod.spec.volumes.configmap": {"$ref": "common#pod.spec.volumes.configmap"},
            "pod.spec.affinity.nodeaffinity": {"$ref": "common#pod.spec.affinity.nodeaffinity"},
            "pod.spec.tolerations": {"$ref": "common#pod.spec.tolerations"},
            "pod.spec.imagepullsecrets": {"$ref": "common#pod.spec.imagepullsecrets"},
            "cronjob": {"$ref": "common#cronjob"},
            "configmap": {"$ref": "common#configmap"},
            "daemonset": {"$ref": "common#daemonset"},
            "replicaset": {"$ref": "common#replicaset"},
            "horizontalpodautoscaler": {"$ref": "common#horizontalpodautoscaler"},
            "job": {"$ref": "common#job"},
            "service": {"$ref": "common#service"},
        }
    },
    "1.22": {
        "branch": "release-1.22",
        "docs": {
            "pod": {"$ref": "common#pod"},
            "pod.spec": {"$ref": "common#pod"},
            "pod.spec.initContainers": {"$ref": "common#pod.spec.initContainers"},
            "pod.spec.containers.lifecycle": {"$ref": "common#pod.spec.containers.lifecycle"},
            "pod.spec.containers.env.valuefrom": {"$ref": "common#pod.spec.containers.env.valuefrom"},
            "pod.spec.containers.envfrom": {"$ref": "common#pod.spec.containers.envfrom"},
            "pod.spec.volumes.configmap.items": {"$ref": "common#pod.spec.volumes.configmap.items"},
            "pod.spec.volumes.configmap": {"$ref": "common#pod.spec.volumes.configmap"},
            "pod.spec.affinity.nodeaffinity": {"$ref": "common#pod.spec.affinity.nodeaffinity"},
            "pod.spec.tolerations": {"$ref": "common#pod.spec.tolerations"},
            "pod.spec.imagepullsecrets": {"$ref": "common#pod.spec.imagepullsecrets"},
            "cronjob": {"$ref": "common#cronjob"},
            "configmap": {"$ref": "common#configmap"},
            "daemonset": {"$ref": "common#daemonset"},
            "replicaset": {"$ref": "common#replicaset"},
            "horizontalpodautoscaler": {"$ref": "common#horizontalpodautoscaler"},
            "job": {"$ref": "common#job"},
            "service": {"$ref": "common#service"},
        }
    },
    "1.21": {
        "branch": "release-1.21",
        "docs": {
            "pod": {"$ref": "common#pod"},
            "pod.spec": {"$ref": "common#pod"},
            "pod.spec.initContainers": {"$ref": "common#pod.spec.initContainers"},
            "pod.spec.containers.lifecycle": {"$ref": "common#pod.spec.containers.lifecycle"},
            "pod.spec.containers.env.valuefrom": {"$ref": "common#pod.spec.containers.env.valuefrom"},
            "pod.spec.containers.envfrom": {"$ref": "common#pod.spec.containers.envfrom"},
            "pod.spec.volumes.configmap.items": {"$ref": "common#pod.spec.volumes.configmap.items"},
            "pod.spec.volumes.configmap": {"$ref": "common#pod.spec.volumes.configmap"},
            "pod.spec.affinity.nodeaffinity": {"$ref": "common#pod.spec.affinity.nodeaffinity"},
            "pod.spec.tolerations": {"$ref": "common#pod.spec.tolerations"},
            "pod.spec.imagepullsecrets": {"$ref": "common#pod.spec.imagepullsecrets"},
            "cronjob": {"$ref": "common#cronjob"},
            "configmap": {"$ref": "common#configmap"},
            "daemonset": {"$ref": "common#daemonset"},
            "replicaset": {"$ref": "common#replicaset"},
            "horizontalpodautoscaler": {"$ref": "common#horizontalpodautoscaler"},
            "job": {"$ref": "common#job"},
            "service": {"$ref": "common#service"},
        }
    },
    "1.20": {
        "branch": "release-1.20",
        "docs": {
            "pod": {"$ref": "common#pod"},
            "pod.spec": {"$ref": "common#pod"},
            "pod.spec.initContainers": {"$ref": "common#pod.spec.initContainers"},
            "pod.spec.containers.lifecycle": {"$ref": "common#pod.spec.containers.lifecycle"},
            "pod.spec.containers.env.valuefrom": {"$ref": "common#pod.spec.containers.env.valuefrom"},
            "pod.spec.containers.envfrom": {"$ref": "common#pod.spec.containers.envfrom"},
            "pod.spec.volumes.configmap.items": {"$ref": "common#pod.spec.volumes.configmap.items"},
            "pod.spec.volumes.configmap": {"$ref": "common#pod.spec.volumes.configmap"},
            "pod.spec.affinity.nodeaffinity": {"$ref": "common#pod.spec.affinity.nodeaffinity"},
            "pod.spec.tolerations": {"$ref": "common#pod.spec.tolerations"},
            "pod.spec.imagepullsecrets": {"$ref": "common#pod.spec.imagepullsecrets"},
            "cronjob": {"$ref": "common#cronjob"},
            "configmap": {"$ref": "common#configmap"},
            "daemonset": {"$ref": "common#daemonset"},
            "replicaset": {"$ref": "common#replicaset"},
            "horizontalpodautoscaler": {"$ref": "common#horizontalpodautoscaler"},
            "job": {"$ref": "common#job"},
            "service": {"$ref": "common#service"},
        }
    }
}


def get_example(product_version, field_ref):
    if product_version in EXAMPLES:
        if field_ref in EXAMPLES[product_version]["docs"]:
            doc = EXAMPLES[product_version]["docs"][field_ref]
            if "branch" in EXAMPLES[product_version]:
                url, text, source = get_info(doc, EXAMPLES[product_version]["branch"])
            else:
                url, text, source = get_info(doc)

            r = requests.get(url)

            if r.status_code == 200:
                return {"result": r.text.strip(), "text": text, "source": source}

    return False


def get_info(obj, branch=None):
    keys = obj.keys()
    key = list(keys)[0]
    if key == "$ref":
        ref = obj["$ref"]
        ref = ref.split("#")
        obj = EXAMPLES[ref[0]]["docs"][ref[1]]

    url = obj["url"].replace("$URL", URL)

    if branch is not None:
        url = url.replace("$BRANCH", branch)

    return url, obj["text"], obj["source"]
