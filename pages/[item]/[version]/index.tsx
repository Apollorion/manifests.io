import Head from 'next/head'
import styles from '@/styles/Home.module.css'
import {GetServerSideProps} from "next";
import { oaspecFetch } from "@/lib/oaspec";
import HeartApollorion from "@/components/HeartApollorion";
import SearchBar from "@/components/SearchBar";
import ResourceTable from "@/components/ResourceTable";
import ReportIssue from "@/components/ReportIssue";
import ManifestsHeading from "@/components/ManifestsHeading";

type ResourcesArr = Array<{
    resource: string;
    description: string;
    key: string;
    links: boolean;
    type: string;
}>;

type Props = {
    resources: ResourcesArr;
    item: string;
    version: string;
};

export default function Home({resources, item, version}: Props) {
    return (
        <>
            <ManifestsHeading
                item={item}
                version={version}
                description={""}
            />
            <main className={styles.main}>
                <HeartApollorion />
                <SearchBar pageItem={item} pageVersion={version} />
                <p style={{marginTop: "50px", marginBottom: "20px", textAlign: "center"}}>
                    Available resources for {item} version {version}.<br/>
                    Your browsers url supports all resources in the {item} openAPI spec, even if a resource is not listed here.
                </p>
                <ResourceTable resources={resources} item={item} version={version} />
                <ReportIssue item={item} />
            </main>
        </>
    )
}

export const getServerSideProps: GetServerSideProps = async ({query}) => {
    const { item, version } = query;

    const spec = oaspecFetch(item as string, version as string);

    let resources: ResourcesArr = [];
    let resourceNames: Array<string> = [];
    for (const [key, value] of Object.entries(spec.definitions)) {
        if("x-kubernetes-group-version-kind" in value
            && "description" in value && value.description !== undefined
        ){
            const resource = key.split(".").pop();
            if(resource && !resource.endsWith("List") && !(resourceNames.includes(resource))){
                resources.push({resource, description: value.description, key, links: true, type: resource});
                resourceNames.push(resource);
            }
        }
    }

    resources.sort((a, b) => (a.resource > b.resource) ? 1 : -1);

    return {
        props: {
            resources,
            item,
            version
        }
    }
}