import styles from "./HeartApollorion.module.css";

export default function HeartApollorion() {
    return (
        <div className={styles.note}>
            <p>
                <span><a href="https://github.com/apollorion/manifests.io">View in GitHub</a></span> • <span>K8s Is Awesome</span> • <span>Made with <span>❤</span></span>
            </p>
            <p style={{fontSize: "smaller", marginTop: "5px"}}>

                Authored by <a href="https://github.com/apollorion/">apollorion</a> •{' '}
                Design support by <a href="https://github.com/baadaa/">baadaa</a>
            </p>
        </div>
    )
}