import styles from "./ResourceTable.module.css";
import {Resource} from "@/typings/Resource";
import {useState} from "react";

type Props = {
    resources: Array<Resource>;
    item: string;
    version: string;
    linkedResource?: string;
    required?: Array<string>;
    leftHeading: string;
};

export default function ResourceTable({leftHeading, resources, item, version, linkedResource, required}: Props) {
    const [resourcesState, setResourcesState] = useState<Array<Resource>>(resources);

    function generateLink(resource: Resource){
        if(linkedResource){
            return `/${item}/${version}/${resource.key}?linked=${linkedResource}.${resource.resource}`
        }
        return `/${item}/${version}/${resource.key}`
    }

    function generateRequiredStar(key: string){
        if(required && required.includes(key)){
            return <span style={{color: "red"}}> *</span>
        }
        return "";
    }

    function filterResources(filter: string | undefined): void {
        if(filter && filter != ""){
            let filteredResources: Array<Resource> = [];
            for(let resource of resources){
                if(resource.resource.toLowerCase().includes(filter.toLowerCase())){
                    filteredResources.push(resource);
                }
            }
            setResourcesState(filteredResources);
        } else {
            setResourcesState(resources);
        }
    }

    return (
        <div>
        <table className={styles.table}>
            <thead>
            <tr style={{backgroundColor: "var(--table-heading-bg)"}}>
                <th><input type="search" className={styles.search} onKeyUp={(e) => {filterResources((e.target as HTMLInputElement).value)}} placeholder={`🔍 ${leftHeading}`} /></th>
                <th>Description</th>
            </tr>
            </thead>
            <tbody>
            {resourcesState.map((resource) => (
                <tr key={resource.resource}>
                    <td>
                        <b>{resource.links ? <a href={generateLink(resource)}><span className="link">{resource.resource}</span> 🔗</a> : resource.resource}</b><br/>
                        <small>{resource.type} {generateRequiredStar(resource.resource)}</small>
                    </td>
                    <td><p>{resource.description}</p></td>
                </tr>
            ))}
            </tbody>
        </table>
        </div>
    )
}