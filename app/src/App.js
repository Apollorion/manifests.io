import { TextField, Table, TableBody, TableContainer, TableHead, TableCell, TableRow } from '@material-ui/core';
import { useState, useEffect} from 'react';
import axios from 'axios';
import { CodeBlock, dracula } from "react-code-blocks";
import { MeteorRainLoading } from 'react-loadingg';


function App() {

  const [query, setQuery] = useState(window.location.pathname.replace("/", ""));
  const [details, setDetails] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const timeOutId = setTimeout(async () => {

      if(query !== ""){
        setLoading(true);
        let details = await search(query);
        setDetails(details);
        setLoading(false);
      }
    }, 500);
    return () => clearTimeout(timeOutId);
  }, [query]);

  const renderDetails = (details) => {
    let rows = [];
    if(details !== undefined && details !== null && details.hasOwnProperty("details")){
      for (const [key, value] of Object.entries(details.details)) {
        rows.push({"title": key, "description": value.description, "links": value.href})
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
              {renderDetails(details).map((row) => (
                <TableRow key={row.title}>
                  <TableCell component="th" scope="row">
                    <span style={{whiteSpace: "nowrap"}}><b>{row.title}</b> {row.links ? (<button onClick={() => {
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

  const renderExample = (details) => {
    if(details !== undefined && details !== null && details.hasOwnProperty("example") && details.example !== null && loading == false){
      return (
        <div>
          <div style={{textAlign: "center"}}>
            <h4>Example</h4>
          </div>
          <div style={{width: "fit-content", minWidth: "30%", margin: "auto"}}>
            <CodeBlock
              language={"yaml"}
              text={details.example}
              showLineNumbers={true}
              theme={dracula}
              wrapLines={true}
          />
        </div>
        </div>
      )
    }
  }

  return (
    <div className="App" style={{marginBottom: "50px"}}>
      <div style={{textAlign: "center", marginTop: "50px", marginBottom: "50px"}}>
      K8s Is Awesome. Made with ❤, <a href="https://apollorion.com">apollorion</a>.<br /><br />
        <TextField id="search" value={query} label="search" placeholder="pod.spec.containers" variant="outlined" style={{width: "75%", textColor: "white"}} onChange={event => setQuery(event.target.value)} />
      </div>
      <div style={{marginBottom: "50px"}}>
        {renderExample(details)}
      </div>
      <div>
      <div style={{textAlign: "center"}}>
        <h4>Spec</h4>
      </div>
          {renderBottom()}
      </div>
    </div>
  );
}

async function search(query) {
  try {
    const response = await axios.get(`https://api.manifests.io/${query}`);
    return response.data;
  } catch (error) {
    console.error(error);
  }
}

export default App;
