import styles from '@/styles/Home.module.css'
import {GetServerSideProps} from "next";
import { oaspecFetch } from "@/lib/oaspec";
import Layout from '@/components/Layout';
import ResourceTable from "@/components/ResourceTable";
import ReportIssue from "@/components/ReportIssue";
import ManifestsHeading from "@/components/ManifestsHeading";
import {NextParsedUrlQuery} from "next/dist/server/request-meta";
import {KubernetesOpenApiSpec} from "@/typings/KubernetesSpec";
import {Resource} from "@/typings/Resource";

type Props = {
    resources: Array<Resource>;
    item: string;
    version: string;
    linkedResource: string;
    description: string;
    resource: string;
    required?: Array<string>;
    gvk?: Array<{
        group: string;
        kind: string;
        version: string;
    }>
};

function Gvk({gvk}: {gvk: Array<{group: string, kind: string, version: string}>}){
    return (<div>
        {gvk.map((gvk) => (
            <p key={`${gvk.group}/${gvk.version}/${gvk.kind}`}>{gvk.group}/{gvk.version}/{gvk.kind}</p>
        ))}
    </div>)
}

export default function Home({resources, item, version, linkedResource, description, resource, gvk, required}: Props) {
    return (
        <Layout item={item} version={version} resource={resource} linked={linkedResource}>
            <ManifestsHeading
                item={item}
                version={version}
                description={description}
                resource={resource}
            />
            <main className={styles.main}>
                <h1 style={{marginTop: "20px", marginBottom: "50px"}}>{linkedResource}</h1>
                {gvk && (
                    <div style={{marginTop: "-30px", marginBottom: "30px", textAlign: "center"}}>
                        <h3>Group/Version/Kind</h3>
                        <Gvk gvk={gvk} />
                    </div>
                )}
                <p style={{whiteSpace: "pre-wrap"}}>{description}</p>
                <ResourceTable leftHeading="Key" required={required} resources={resources} item={item} version={version} linkedResource={linkedResource} />
                <ReportIssue resource={resource} item={item} />
            </main>
        </Layout>
    )
}

export const getServerSideProps: GetServerSideProps = async ({query}) => {
    function parseQuery(query: NextParsedUrlQuery){
        const { item, version, resource, linked } = query;
        if(Array.isArray(item) || !item){
            throw new Error("Item is not a string");
        }
        if(Array.isArray(version) || !version){
            throw new Error("Version is not a string");
        }
        if(Array.isArray(resource) || !resource){
            throw new Error("Resource is not a string");
        }
        if(Array.isArray(linked)){
            throw new Error("Linked is not a string");
        }
        return { item, version, resource, linked };
    }

    function defaultString(value: string | undefined, defaultValue?: string): string {
        if(value === undefined) {
            return defaultValue ? defaultValue : "";
        }
        return value;
    }

    function popString(value: string): string {
        let returnValue = value.split(".").pop();
        if(returnValue === undefined){
            return value;
        }
        return returnValue;
    }

    function buildResource(resource: string, key: string, value:any): Resource {
        let resourceObj: Resource = {resource, description: value.description, key, type: "Unknown"};
        if (value.$ref) {
            resourceObj.links = true;
            resourceObj.key = value.$ref.replace("#/definitions/", "");
            resourceObj.type = value.type ? value.type : popString(value.$ref);
        } else {
            if (value.items && value.items.$ref) {
                resourceObj.links = true;
                resourceObj.key = value.items.$ref.replace("#/definitions/", "");
                resourceObj.type = `${popString(value.items.$ref)}[]`;
            } else {
                resourceObj.type = value.type ? value.type : resource;
            }
        }
        return resourceObj;
    }

    function getResourceArrayFromSpec(item: string, version: string, spec: KubernetesOpenApiSpec): Array<Resource> {
        let resources: Array<Resource> = [];
        let resourceNames: Array<string> = [];
        const resourceSpec = spec.definitions[resource];
        if(resourceSpec.properties){
            for (const [key, value] of Object.entries(resourceSpec.properties)) {
                const resource = popString(key);
                if (resource && !resource.endsWith("List") && !(resourceNames.includes(resource))) {
                    resources.push(buildResource(resource, key, value));
                    resourceNames.push(resource);
                }
            }
        } else {
            resources.push({
                resource: popString(resource),
                description: defaultString(resourceSpec.description),
                type: "Unknown",
                key: resource
            });
        }
        resources.sort((a, b) => (a.resource > b.resource) ? 1 : -1);
        return resources
    }

    const { item, version, resource, linked } = parseQuery(query);

    const spec = oaspecFetch(item, version);
    const resourceSpec = spec.definitions[resource];
    const linkedResource = defaultString(linked, popString(resource));
    const resources = getResourceArrayFromSpec(item, version, spec);

    let props: Props = {
        resources,
        item,
        version,
        linkedResource,
        description: defaultString(resourceSpec.description),
        resource
    };

    if(resourceSpec["x-kubernetes-group-version-kind"]) {
        props["gvk"] = resourceSpec["x-kubernetes-group-version-kind"];
    }

    if(resourceSpec.required) {
        props["required"] = resourceSpec.required;
    }

    return {
        props
    }
}