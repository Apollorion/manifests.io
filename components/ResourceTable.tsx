import styles from "./ResourceTable.module.css";
import {Resource} from "@/typings/Resource";

type Props = {
    resources: Array<Resource>;
    item: string;
    version: string;
    linkedResource?: string;
    required?: Array<string>;
    leftHeading: string;
};

export default function ResourceTable({leftHeading, resources, item, version, linkedResource, required}: Props) {

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

    return (
        <table className={styles.table}>
            <thead>
            <tr style={{backgroundColor: "var(--table-heading-bg)"}}>
                <th>{leftHeading}</th>
                <th>Description</th>
            </tr>
            </thead>
            <tbody>
            {resources.map((resource) => (
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
    )
}