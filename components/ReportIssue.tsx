export default function ReportIssue({item, resource}: {item: string, resource?: string}) {
    return (
        <a style={{marginTop: "10px", marginBottom: "50px"}} href={`https://github.com/Apollorion/manifests.io/issues/new?title=${encodeURIComponent(resource ? `${item} - ${resource}` : item)}&body=%23%23%20Description%20of%20issue%0A`}>See an issue here?</a>
    )
}