import {availableItemVersions} from "@/lib/oaspec";

function SearchBar({pageItem, pageVersion}: {pageItem: string, pageVersion: string}){
    const itemVersions = availableItemVersions();

    const handleChanges = (event: React.ChangeEvent<HTMLSelectElement>) => {
        window.location.href=event.target.value
    }

    return (
        <div>
            <select
                style={{width: "50ch", textAlignLast: "center"}}
                name="product"
                id="product"
                defaultValue={`/${pageItem}/${pageVersion}`}
                onChange={handleChanges}
            >
                {Object.keys(itemVersions).map((item) => (
                    <optgroup key={item} label={item}>
                        {itemVersions[item].map((version) => (
                            <option key={`/${item}/${version}`}
                                    value={`/${item}/${version}`}
                            >{`${item} - ${version}`}</option>
                        ))}
                    </optgroup>
                ))}
            </select>
        </div>
    )
}

export default SearchBar;