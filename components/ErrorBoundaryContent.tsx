import styles from "@/styles/Home.module.css";
import ReportIssue from "@/components/ReportIssue";
import Layout from "@/components/Layout";
import React from "react";
import {defaultItemVersion} from "@/lib/oaspec";

type Props = {
    errorType: string;
}
export default function ErrorBoundaryContent({errorType}: Props) {
    const defaults = defaultItemVersion();
    return (
        <Layout item={defaults.item} version={defaults.version}>
            <main className={styles.main}>
                <div className={styles.intro} style={{textAlign: "center"}}>
                    <h1>
                        Whoops. {errorType} Error.
                    </h1>
                    <p>
                        One of three things just happened.
                    </p>
                    <div style={{textAlign: "center"}}>
                        <ol type="A"
                            style={{display: "inline-block", listStylePosition: "inside", marginTop: "10px", marginBottom: "10px", textAlign: "left"}}>
                            <li>The version of the spec you are looking for does not exist.</li>
                            <li>The resource you are looking for does not exist in this version of the spec.</li>
                            <li>You found a legitimate bug in the code, please use the below button to report it.</li>
                        </ol>
                    </div>
                    <p>
                        Try going back, or change to a different spec above.
                    </p>
                </div>
                <ReportIssue item="ErrorBoundary"/>
            </main>
        </Layout>
    )
}