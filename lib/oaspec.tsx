import {KubernetesOpenApiSpec} from "@/typings/KubernetesSpec";

import kubernetes126 from "../oaspec/kubernetes/1.26.json";
import kubernetes125 from "../oaspec/kubernetes/1.25.json";
import kubernetes124 from "../oaspec/kubernetes/1.24.json";
import kubernetes123 from "../oaspec/kubernetes/1.23.json";
import kubernetes122 from "../oaspec/kubernetes/1.22.json";
import kubernetes121 from "../oaspec/kubernetes/1.21.json";

import certmanager171 from "../oaspec/certmanager/1.7.json";

import flagger1190 from "../oaspec/flagger/1.19.0.json";

import flux0273 from "../oaspec/flux/0.27.3.json";

import istio1133 from "../oaspec/istio/1.13.3.json";

import cosignpolicycontroller057 from "../oaspec/cosignpolicycontroller/0.5.7.json";

export function oaspecFetch(item: string, version: string): KubernetesOpenApiSpec {
    if (item === "kubernetes") {
        switch(version) {
            case "1.26":
                return kubernetes126;
            case "1.25":
                return kubernetes125;
            case "1.24":
                return kubernetes124;
            case "1.23":
                return kubernetes123;
            case "1.22":
                return kubernetes122;
            case "1.21":
                return kubernetes121;
        }
    }

    if (item === "certmanager") {
        if (version === "1.7") {
            return certmanager171;
        }
    }

    if(item === "flagger"){
        if(version === "1.19.0"){
            return flagger1190;
        }
    }

    if(item === "flux"){
        if(version === "0.27.3"){
            return flux0273;
        }
    }

    if(item === "istio"){
        if(version === "1.13.3"){
            return istio1133;
        }
    }

    if(item === "cosign policy-controller"){
        if(version === "0.5.7"){
            return cosignpolicycontroller057;
        }
    }

    throw new Error("Open Api Spec Not Found");
}

export function availableItemVersions(): {[key: string]: Array<string>} {
    return {
        "certmanager": ["1.7"],
        "cosign policy-controller": ["0.5.7"],
        "flagger": ["1.19.0"],
        "flux": ["0.27.3"],
        "istio": ["1.13.3"],
        "kubernetes": ["1.21", "1.22", "1.23", "1.24", "1.25", "1.26"]
    }
}

export function defaultItemVersion(): {item: string, version: string} {
    return {
        item: "kubernetes",
        version: "1.26"
    }
}