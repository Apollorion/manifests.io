import { TextField, Table, TableBody, TableContainer, TableHead, TableCell, TableRow } from '@material-ui/core';
import MenuItem from '@mui/material/MenuItem';
import FormControl from '@mui/material/FormControl';
import Select from '@mui/material/Select';
import InputLabel from '@mui/material/InputLabel';
import { useState, useEffect} from 'react';
import axios from 'axios';
import { CodeBlock, dracula } from "react-code-blocks";
import { MeteorRainLoading } from 'react-loadingg';
import k8s from './k8s_details';

const apiUrl = "http://localhost:8000/"

function App() {

  const [query, setQuery] = useState(window.location.pathname.replace("/", ""));
  const [k8sVersion, setk8sVersion] = useState(0);
  const [details, setDetails] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [requiredList, setRequiredList] = useState([]);

  useEffect(() => {
    const timeOutId = setTimeout(async () => {

      if(query !== ""){
        await search();
      }
    }, 500);
    return () => clearTimeout(timeOutId);
  }, [query, k8sVersion]);

  const renderDetails = () => {
    let rows = [];
    if(details?.properties){
      for (const [key, value] of Object.entries(details.properties)) {
        let result = {"title": key, "links": value.hasOwnProperty("properties")};

        result["required"] = !!requiredList.includes(key);

        if(value?.description){
          result["description"] = value.description.split('\n').map((str, index) => <p key={index}>{str}</p>)
        }

        if(value?.type){
          result["type"] = value.type
        }


        rows.push(result)
      }
      window.history.replaceState(null, null, `/${query}`)
    }
    return rows;
  };

  const renderBottom = () => {
      if(loading){
        return (
          <div style={{textAlign: "center"}}>
            Searching...<br />
            <MeteorRainLoading />
          </div>
        )
      } else if(error !== "") {
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
                    <span style={{whiteSpace: "nowrap"}}><b>{renderCellTitle(row)}</b> {row.links ? (<button onClick={() => {
                          setQuery(`${query}.${row.title}`)
                          setDetails("");
                      }}>↑</button>) : "" }</span>
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
    let title = row.title;
    if(row.required){
      title = (<div>{title}<span style={{color: 'red'}}>*</span></div>)
    }
    return title;
  };

  const renderTitle = () => {
    if(details?.gvk){
      return details.gvk.kind
    }
  };

  const renderGVK = () => {
    if(details?.gvk){

      let apiVersion = details.gvk.version;
      if(details.gvk.group !== "core"){
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
    if(details?.description){
      return details.description.split('\n').map((str, index) => <p key={index}>{str}</p>);
    }
  };

  const search = async() => {
    setLoading(true);
    setError("");
    try {
      const response = await axios.get(`${apiUrl}${k8s.choices[k8sVersion]}/${query}`);
      setDetails(response.data);
      if(response.data?.required){
        setRequiredList(response.data.required);
      }
    } catch (error) {
      if(error?.response?.data?.detail){
        setDetails("")
        setError(error.response.data.detail);
      }
    }
    setLoading(false);
  };

  return (
    <div className="App" style={{marginBottom: "50px"}}>
      <div style={{textAlign: "center", marginTop: "50px", marginBottom: "50px"}}>
      K8s Is Awesome. Made with ❤, <a href="https://apollorion.com">apollorion</a>.<br /><br />
        <div style={{flexDirection: "row", justifyContent: "center"}}>
          <FormControl>
            <InputLabel id="k8s-select">k8s</InputLabel>
            <Select
                autoWidth
                id="k8s-select"
                value={k8sVersion}
                label="k8s"
                onChange={(event) => {
                  setk8sVersion(event.target.value);
                }}
            >
              {k8s.choices.map((item, index) => {
                return <MenuItem key={index} value={index}>{item}</MenuItem>
              })}
            </Select>
          </FormControl>
          <TextField id="search" value={query} label="search" placeholder="<resource>.<fieldPath>.[<fieldPath>]" variant="outlined" style={{width: "75%", textColor: "white"}} onChange={event => setQuery(event.target.value)} />
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
