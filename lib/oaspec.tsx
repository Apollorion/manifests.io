import {KubernetesOpenApiSpec} from "@/typings/KubernetesSpec";

import kubernetes128 from "../oaspec/kubernetes/1.28.json";
import kubernetes127 from "../oaspec/kubernetes/1.27.json";
import kubernetes126 from "../oaspec/kubernetes/1.26.json";
import kubernetes125 from "../oaspec/kubernetes/1.25.json";
import kubernetes124 from "../oaspec/kubernetes/1.24.json";

import certmanager171 from "../oaspec/certmanager/1.7.json";

import flagger1190 from "../oaspec/flagger/1.19.0.json";

import flux201 from "../oaspec/flux/2.0.1.json";
import flux0312 from "../oaspec/flux/0.31.2.json";
import flux0273 from "../oaspec/flux/0.27.3.json";

import istio1133 from "../oaspec/istio/1.13.3.json";

import cosignpolicycontroller057 from "../oaspec/cosignpolicycontroller/0.5.7.json";

export function oaspecFetch(item: string, version: string): KubernetesOpenApiSpec {
    if (item === "kubernetes") {
        switch(version) {
            case "1.28":
                return kubernetes128;
            case "1.27":
                return kubernetes127;
            case "1.26":
                return kubernetes126;
            case "1.25":
                return kubernetes125;
            case "1.24":
                return kubernetes124;
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
        switch(version) {
            case "2.0.1":
                return flux201;
            case "0.31.2":
                return flux0312;
            case "0.27.3":
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
        "flux": ["0.27.3", "0.31.2", "2.0.1"],
        "istio": ["1.13.3"],
        "kubernetes": ["1.24", "1.25", "1.26", "1.27", "1.28"]
    }
}

export function defaultItemVersion(): {item: string, version: string} {
    return {
        item: "kubernetes",
        version: "1.28"
    }
}
