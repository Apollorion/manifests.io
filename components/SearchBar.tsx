import {availableItemVersions} from "@/lib/oaspec";

function SearchBar({pageItem, pageVersion}: {pageItem: string, pageVersion: string}){
    const itemVersions = availableItemVersions();
    return (
        <div>
            <select
                style={{width: "50ch", textAlignLast: "center"}}
                name="product"
                id="product"
                defaultValue={`${pageItem}/${pageVersion}`}
            >
                {Object.keys(itemVersions).map((item) => (
                    <optgroup key={item} label={item}>
                        {itemVersions[item].map((version) => (
                            <option key={`${item}/${version}`}
                                    value={`${item}/${version}`}
                                    onClick={() => window.location.href=`/${item}/${version}`}
                            >{`${item} - ${version}`}</option>
                        ))}
                    </optgroup>
                ))}
            </select>
        </div>
    )
}

export default SearchBar;