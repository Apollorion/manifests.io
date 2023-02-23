import styles from '@/styles/Home.module.css'
import {GetServerSideProps} from "next";
import { oaspecFetch } from "@/lib/oaspec";
import Layout from '@/components/Layout';
import ResourceTable from "@/components/ResourceTable";
import ReportIssue from "@/components/ReportIssue";
import ManifestsHeading from "@/components/ManifestsHeading";

type Resource = {
    resource: string;
    description: string;
    links?: boolean;
    key: string;
    type: string;
};

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
    const { item, version, resource, linked } = query;

    if (typeof resource !== "string") {
        return {
            redirect: {
                destination: `/${item}/${version}`,
                permanent: false,
            },
        }
    }

    let linkedResource = linked;
    if(linked === undefined && resource) {
        linkedResource = resource.split(".").pop();
    }

    const spec = oaspecFetch(item as string, version as string);

    let resources: Array<Resource> = [];
    let resourceNames: Array<string> = [];
    const resourceSpec = spec.definitions[resource];

    if("properties" in resourceSpec){
        // @ts-ignore
        for (const [key, value] of Object.entries(resourceSpec.properties)) {
            const resource = key.split(".").pop();
            if (resource && !resource.endsWith("List") && !(resourceNames.includes(resource))) {
                if(!value.description){
                    value.description = "";
                }
                // @ts-ignore
                let resourceObj: Resource = {resource, description: value.description, key};
                // @ts-ignore
                if ("$ref" in value) {
                    resourceObj.links = true;
                    // @ts-ignore
                    resourceObj.key = value.$ref.replace("#/definitions/", "")
                    // @ts-ignore
                    resourceObj.type = value.type ? value.type : value.$ref.split(".").pop();
                } else { // @ts-ignore
                    if ("items" in value && "$ref" in value.items) {
                        resourceObj.links = true;
                        // @ts-ignore
                        resourceObj.key = value.items.$ref.replace("#/definitions/", "");
                        // @ts-ignore
                        resourceObj.type = `${value.items.$ref.split(".").pop()}[]`;
                    } else {
                        resourceObj.type = value.type ? value.type : resource;
                    }
                }
                // @ts-ignore
                resources.push(resourceObj);
                resourceNames.push(resource);
            }
        }
    } else {
        // @ts-ignore
        resources.push({resource: resource.split(".").pop(), description: resourceSpec.description});
    }

    resources.sort((a, b) => (a.resource > b.resource) ? 1 : -1);

    // @ts-ignore
    let props: Props = {resources, item, version, linkedResource, description: resourceSpec.description, resource}

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