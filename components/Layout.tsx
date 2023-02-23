import HeartApollorion from "@/components/HeartApollorion";
import ReportIssue from "@/components/ReportIssue";
import SearchBar from "./SearchBar";
import styles from "./Layout.module.css";

type Props = {
    children: React.ReactNode;
    item: string;
    version: string;
}

export default function Layout({children, item, version}: Props) {
    return (
        <>  
            <header className={styles.header}>
                <div className={styles.wrapper}>
                    <div>
                        <h1>Manifests.io</h1>
                        <span>Easy to use online kubernetes documentation</span>
                    </div>
                    <SearchBar pageItem={item} pageVersion={version} />
                </div>
            </header>
                {children}
            <footer style={{ display: 'flex', justifyContent: 'center'}}>
                <HeartApollorion />
            </footer>
        </>
    )
}