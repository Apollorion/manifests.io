import MenuItem from '@mui/material/MenuItem';
import FormControl from '@mui/material/FormControl';
import TextField from '@mui/material/TextField';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableContainer from '@mui/material/TableContainer';
import TableHead from '@mui/material/TableHead';
import TableCell from '@mui/material/TableCell';
import TableRow from '@mui/material/TableRow';
import Autocomplete from '@mui/material/Autocomplete';
import CircularProgress from '@mui/material/CircularProgress'
import Link from '@mui/material/Link';
import InputIcon from '@mui/icons-material/Input';
import {useState, useEffect, Fragment} from 'react';
import axios from 'axios';
import {CodeBlock, dracula} from "react-code-blocks";
import {MeteorRainLoading} from 'react-loadingg';
import ReactGA from 'react-ga';

import k8s from './k8s_details';

let apiUrl = "http://localhost:8000/";
if (process.env?.REACT_APP_API_URL) {
    apiUrl = process.env.REACT_APP_API_URL;
}
console.log("Using API", apiUrl);

if (process.env?.REACT_APP_GA_ID) {
    ReactGA.initialize(process.env.REACT_APP_GA_ID);
}

function App() {

    // keep query inital state as empty string as to not hit the API with a dumb, unnessary request.
    const [query, setQuery] = useState("");
    const [textBoxQuery, setTextBoxQuery] = useState("");
    const [k8sVersion, setk8sVersion] = useState(0);
    const [details, setDetails] = useState("");
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState("");
    const [requiredList, setRequiredList] = useState([]);
    const [autocomplete, setAutocomplete] = useState([]);
    const [autocompleteLoading, setAutocompleteLoading] = useState(false);

    useEffect(() => {
        function setPathStates() {
            if (window.location.pathname === "/") {
                // Handle naked domais
                setQuery('pod.spec')
                setTextBoxQuery('pod.spec')
                setk8sVersion(k8s.choices.indexOf(k8s.default))
            } else if (window.location.pathname.split("/").length !== 3) {
                // handle incorrect length (assume the k8s version wasnt in the URL)
                const item = window.location.pathname.replace("/", "")
                setQuery(item)
                setTextBoxQuery(item)
                setk8sVersion(k8s.choices.indexOf(k8s.default));
            } else {
                // handle correct URL's
                const pathArr = window.location.pathname.split("/");
                if (pathArr.length === 3) {
                    setQuery(pathArr[2])
                    setTextBoxQuery(pathArr[2])
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
        const search = async () => {
            if (query !== "") {
                setLoading(true);
                setError("");
                setDetails("");
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
                if (process.env?.REACT_APP_GA_ID) {
                    ReactGA.pageview(`/${k8s.choices[k8sVersion]}/${query}`);
                }
                setLoading(false);
            }
        };
        search()
    }, [query, k8sVersion]);

    useEffect(() => {
        const getAutoComplete = async () => {
            if (textBoxQuery !== ""){
                setAutocompleteLoading(true);
                const response = await axios.get(`${apiUrl}keys/${k8s.choices[k8sVersion]}/${textBoxQuery}`);
                setAutocomplete(response.data);
                setAutocompleteLoading(false);
            }
        };

        const timeOutId = setTimeout(async () => {
            if (textBoxQuery !== "") {
                await getAutoComplete();
            }
        }, 1000);
        return () => clearTimeout(timeOutId);
    }, [textBoxQuery, k8sVersion])

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

                if (value?.oname) {
                    result["oname"] = value.oname
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
                    <p>{error}</p>
                    <p>If you think this error was thrown... in error... let me know!</p>
                    <p style={{fontWeight: "bold"}}>Open an issue on <a
                        href="https://github.com/Apollorion/manifests.io/issues/new">github</a>!</p>
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
                                        <span style={{whiteSpace: "nowrap"}}><b>{renderCellTitle(row)}</b></span>
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

    const navigateToItem = (row) => {
        if (row?.links) {
            setQuery(`${query}.${row.title}`);
            setDetails("");
        }
    };

    const renderCellTitle = (row) => {
        let type = "";
        if (row?.type) {
            type = row.type
        }

        if (row?.oname) {
            row.title = row.oname;
        }

        const CustomLink = row?.links ? Link : "span"
        const CustomInputIcon = row?.links ? InputIcon : "span";

        let title = (
            <div>
                <CustomLink onClick={() => navigateToItem(row)} component="button"
                            style={{fontWeight: "bold", color: "black"}}>
                    {row.title} <CustomInputIcon fontSize="xxsmall" style={{marginBottom: -2}}/>
                </CustomLink>
                <br/>
                <span style={{fontSize: "8pt", fontWeight: "normal"}}>
                    {type}
                </span>
            </div>
        );

        if (row.required) {
            title = (
                <div>
                    <CustomLink onClick={() => navigateToItem(row)} component="button"
                                style={{fontWeight: "bold", color: "black"}}>
                        {row.title} <CustomInputIcon fontSize="xxsmall" style={{marginBottom: -2}}/>
                    </CustomLink>
                    <br/>
                    <span style={{fontSize: "8pt", fontWeight: "normal"}}>
                        {type}
                    </span>
                    <span style={{color: 'red'}}>
                        *
                    </span>
                </div>
            );
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

    return (
        <div className="App" style={{marginBottom: "50px"}}>
            <div style={{textAlign: "center", marginTop: "50px", marginBottom: "50px"}}>
                K8s Is Awesome. Made with ❤, <a
                href="https://github.com/apollorion/manifests.io">apollorion</a>.<br/><br/>
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
                        <Autocomplete
                            id="search"
                            value={query}
                            onChange={(event, value) => {
                                if(value !== null){
                                    setQuery(value)
                                }
                            }}
                            inputValue={textBoxQuery}
                            onInputChange={(event, value) => {
                                if(value !== null){
                                    setTextBoxQuery(value)
                                }
                            }}
                            sx={{width: '50ch'}}
                            freeSolo
                            options={autocomplete}
                            renderInput={(params) => {
                                return (<TextField {...params}
                                                   variant="standard"
                                                   label="search"
                                                   placeholder="<resource>.<fieldPath>.[<fieldPath>]"
                                                   InputProps={{
                                                       ...params.InputProps,
                                                       endAdornment: (
                                                           <Fragment>
                                                               {autocompleteLoading ? <CircularProgress color="inherit" size={20} /> : null}
                                                               {params.InputProps.endAdornment}
                                                           </Fragment>
                                                       ),
                                                   }}
                                />);
                            }}
                        />
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
