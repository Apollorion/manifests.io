import MenuItem from '@mui/material/MenuItem';
import FormControl from '@mui/material/FormControl';
import TextField from '@mui/material/TextField';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableCell from '@mui/material/TableCell';
import TableRow from '@mui/material/TableRow';
import {useState, useEffect} from 'react';
import axios from 'axios';
import {CodeBlock, dracula} from "react-code-blocks";
import {MeteorRainLoading} from 'react-loadingg';
import k8s from './k8s_details';

let apiUrl = "http://localhost:8000/";
if(process.env?.API_URL){
    apiUrl = process.env.API_URL;
}

function App() {

    // keep query inital state as empty string as to not hit the API with a dumb, unnessary request.
    const [query, setQuery] = useState("");
    const [k8sVersion, setk8sVersion] = useState(0);
    const [details, setDetails] = useState("");
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState("");
    const [requiredList, setRequiredList] = useState([]);

    useEffect(() => {
        function setPathStates() {
            if (window.location.pathname === "/") {
                // Handle naked domais
                setQuery(`pod.spec`)
                setk8sVersion(k8s.choices.length - 1)
            } else if (window.location.pathname.split("/").length !== 3) {
                // handle incorrect length (assume the k8s version wasnt in the URL)
                const item = window.location.pathname.replace("/", "")
                setQuery(item)
                setk8sVersion(k8s.choices.indexOf(k8s.default));
            } else {
                // handle correct URL's
                const pathArr = window.location.pathname.split("/");
                if (pathArr.length === 3) {
                    setQuery(pathArr[2])
                    setk8sVersion(k8s.choices.indexOf(pathArr[1]));
                }
            }
        }

        setPathStates();
        window.onpopstate = () => {
            //TODO: Theres a bug here where you sometimes have to push the browser back button twice to actually go back.
            setPathStates();
        };
    }, []);

    useEffect(() => {
        const timeOutId = setTimeout(async () => {
            if (query !== "") {
                await search();
            }
        }, 500);
        return () => clearTimeout(timeOutId);
    }, [query, k8sVersion]);

    const renderDetails = () => {
        let rows = [];
        if (details?.properties) {
            for (const [key, value] of Object.entries(details.properties)) {
                let result = {"title": key, "links": value.hasOwnProperty("properties")};

                result["required"] = !!requiredList.includes(key);

                if (value?.description) {
                    result["description"] = value.description.split('\n').map((str, index) => <p key={index}>{str}</p>)
                }

                if (value?.type) {
                    let newType;
                    switch (value.type) {
                        case "string":
                            newType = "str"
                            break;
                        case "array":
                            newType = "[]"
                            break;
                        case "integer":
                            newType = "int"
                            break;
                        case "object":
                            newType = "obj"
                            break;
                        default:
                            newType = value.type
                            break;
                    }

                    result["type"] = newType
                    if ((value?.properties?.gvk || value?.gvk) && ["array", "object"].includes(value.type)) {
                        if (value?.properties?.gvk) {
                            result["type"] = `${newType.replace("obj", "")}${value.properties.gvk.kind}`
                        } else {
                            result["type"] = `${newType.replace("obj", "")}${value.gvk.kind}`
                        }
                    }
                }
                rows.push(result)
            }
        }
        return rows;
    };

    const renderBottom = () => {
        if (loading) {
            return (
                <div style={{textAlign: "center"}}>
                    Searching...<br/>
                    <MeteorRainLoading/>
                </div>
            )
        } else if (error !== "") {
            return (
                <div style={{textAlign: "center"}}>
                    {error}
                </div>
            )
        } else {
            return (
                <TableContainer>
                    <Table aria-label="simple table">
                        <TableHead>
                            <TableRow>
                                <TableCell><b>Item</b></TableCell>
                                <TableCell><b>Description</b></TableCell>
                            </TableRow>
                        </TableHead>
                        <TableBody>
                            {renderDetails().map((row) => (
                                <TableRow key={row.title}>
                                    <TableCell component="th" scope="row">
                    <span style={{whiteSpace: "nowrap"}}><b>{renderCellTitle(row)}</b> {row.links ? (
                        <button onClick={() => {
                            setQuery(`${query}.${row.title}`)
                            setDetails("");
                        }}>↑</button>) : ""}</span>
                                    </TableCell>
                                    <TableCell>{row.description}</TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                </TableContainer>
            )
        }
    }

    const renderCellTitle = (row) => {
        let type = "";
        if (row?.type) {
            type = row.type
        }

        let title = (<div>{row.title}<br/><span style={{fontSize: "xxsmall", fontWeight: "normal"}}>{type}</span></div>)

        if (row.required) {
            title = (<div>{row.title}<span style={{color: 'red'}}>*</span><br/><span
                style={{fontSize: "xxsmall", fontWeight: "normal"}}>{type}</span></div>)
        }
        return title;
    };

    const renderTitle = () => {
        if (details?.gvk) {
            return details.gvk.kind
        }
    };

    const renderGVK = () => {
        if (details?.gvk) {

            let apiVersion = details.gvk.version;
            if (details.gvk.group !== "core") {
                apiVersion = `${details.gvk.group}/${apiVersion}`
            }

            const gvkYaml = `apiVersion: ${apiVersion}\nkind: ${details.gvk.kind}`.trim();

            return (
                <div>
                    <div style={{width: "fit-content", margin: "auto", textAlign: "left"}}>
                        <CodeBlock
                            language={"yaml"}
                            text={gvkYaml}
                            theme={dracula}
                            wrapLines={true}
                        />
                    </div>
                </div>
            )
        }
    }

    const renderDescription = () => {
        if (details?.description) {
            return details.description.split('\n').map((str, index) => <p key={index}>{str}</p>);
        }
    };

    const search = async () => {
        setLoading(true);
        setError("");
        try {
            const response = await axios.get(`${apiUrl}${k8s.choices[k8sVersion]}/${query}`);
            setDetails(response.data);
            if (response.data?.required) {
                setRequiredList(response.data.required);
            }
        } catch (error) {
            if (error?.response?.data?.detail) {
                setDetails("")
                setError(error.response.data.detail);
            }
        }
        window.history.pushState(null, null, `/${k8s.choices[k8sVersion]}/${query}`)
        setLoading(false);
    };

    return (
        <div className="App" style={{marginBottom: "50px"}}>
            <div style={{textAlign: "center", marginTop: "50px", marginBottom: "50px"}}>
                K8s Is Awesome. Made with ❤, <a href="https://github.com/apollorion/manifests.io">apollorion</a>.<br/><br/>
                <div style={{flexDirection: "row", justifyContent: "center"}}>
                    <FormControl style={{flexDirection: "row"}}>
                        <TextField
                            select
                            sx={{width: '10ch'}}
                            id="k8s-select"
                            value={k8sVersion}
                            variant="standard"
                            label="k8s version"
                            onChange={(event) => {
                                setk8sVersion(event.target.value);
                            }}
                        >
                            {k8s.choices.map((item, index) => {
                                return <MenuItem key={index} value={index}>{item}</MenuItem>
                            })}
                        </TextField>
                    <TextField
                        id="search" value={query} label="search" placeholder="<resource>.<fieldPath>.[<fieldPath>]"
                        sx={{ width: '50ch' }}
                        variant="standard"
                        onChange={event => setQuery(event.target.value)}/>
                    </FormControl>
                </div>
            </div>
            <div>
                <div style={{textAlign: "center"}}>
                    <h4>{renderTitle()}</h4>
                    {renderGVK()}<br/>
                    {renderDescription()}
                </div>
                {renderBottom()}
            </div>
        </div>
    );
}

export default App;
