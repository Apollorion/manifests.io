import styles from '@/styles/Home.module.css'
import {GetServerSideProps} from "next";
import { oaspecFetch } from "@/lib/oaspec";
import Layout from '@/components/Layout';
import ResourceTable from "@/components/ResourceTable";
import ReportIssue from "@/components/ReportIssue";
import ManifestsHeading from "@/components/ManifestsHeading";
import {Resource} from "@/typings/Resource";

type Props = {
    resources: Array<Resource>;
    item: string;
    version: string;
};

export default function Home({resources, item, version}: Props) {
    return (
        <Layout item={item} version={version}>
            <ManifestsHeading
                item={item}
                version={version}
                description={""}
            />
            <main className={styles.main}>
                <div className={styles.intro}>
                    <h3>
                        Available resources for <code>{item}</code> version <code>{version}</code>
                    </h3>
                    <p>
                        Your browsers url supports all resources in the {item} openAPI spec, even if a resource is not listed here.
                    </p>
                </div>
                <ResourceTable leftHeading="Object" resources={resources} item={item} version={version} />
                <ReportIssue item={item} />
            </main>
        </Layout>
    )
}

export const getServerSideProps: GetServerSideProps = async ({query}) => {
    const { item, version } = query;

    const spec = oaspecFetch(item as string, version as string);

    let resources: Array<Resource> = [];
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