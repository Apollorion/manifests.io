import {availableItemVersions} from "@/lib/oaspec";
import styles from "./SearchBar.module.css";

function SearchBar({pageItem, pageVersion, resource, linked}: {pageItem: string, pageVersion: string, resource?: string, linked?: string}) {
    const itemVersions = availableItemVersions();

    const handleChanges = (event: React.ChangeEvent<HTMLSelectElement>) => {
        window.location.href=event.target.value
    }

    const generateDefault = () => {
        if(resource){
            if(linked){
                return `/${pageItem}/${pageVersion}/${resource}?linked=${linked}`
            }
            return `/${pageItem}/${pageVersion}/${resource}`
        } else {
            return `/${pageItem}/${pageVersion}`
        }
    }

    const generateValue = (item: string, version: string) => {
        if(resource && item === pageItem){
            if(linked){
                return `/${item}/${version}/${resource}?linked=${linked}`
            }
            return `/${item}/${version}/${resource}`
        } else {
            return `/${item}/${version}`
        }
    }

    return (
        <div className={styles.selector}>
            <label htmlFor="product">Choose a spec:</label>
            <select
                name="product"
                id="product"
                defaultValue={generateDefault()}
                onChange={handleChanges}
            >
                {Object.keys(itemVersions).map((item) => (
                    <optgroup key={item} label={item}>
                        {itemVersions[item].map((version) => (
                            <option key={`/${item}/${version}`}
                                    value={generateValue(item, version)}
                            >{`${item} - ${version}`}</option>
                        ))}
                    </optgroup>
                ))}
            </select>
        </div>
    )
}

export default SearchBar;