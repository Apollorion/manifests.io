type Resource = {
    resource: string;
    description: string;
    links?: boolean;
    key?: string;
    type: string;
};

type Props = {
    resources: Array<Resource>;
    item: string;
    version: string;
    linkedResource?: string;
    required?: Array<string>;
};

export default function ResourceTable({resources, item, version, linkedResource, required}: Props) {

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
        <table style={{width: "100%", borderCollapse: "collapse", marginTop: "1rem"}}>
            <thead>
            <tr style={{padding: "10px", borderBottom: "1pt solid var(--table-border)", backgroundColor: "var(--table-heading-bg)"}}>
                <td style={{padding: "10px"}}><b>Item</b></td>
                <td style={{padding: "10px"}}><b>Description</b></td>
            </tr>
            </thead>
            <tbody>
            {resources.map((resource) => (
                <tr key={resource.resource} style={{padding: "10px", borderBottom: "1pt solid var(--table-border)"}}>
                    <td style={{padding: "10px", whiteSpace: "nowrap"}}>
                        <b>{resource.links ? <a href={generateLink(resource)}><span className="link">{resource.resource}</span> 🔗</a> : resource.resource}</b><br/>
                        <small>{resource.type} {generateRequiredStar(resource.resource)}</small>
                    </td>
                    <td style={{padding: "10px"}}><p style={{whiteSpace: "pre-line"}}>{resource.description}</p></td>
                </tr>
            ))}
            </tbody>
        </table>
    )
}