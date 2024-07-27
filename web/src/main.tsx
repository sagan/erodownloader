import React, { useState, useEffect } from "react";
import ReactDOM from "react-dom/client";
import "normalize.css";
import "tailwindcss/tailwind.css";
import { BrowserRouter as Router, Routes, Route } from "react-router-dom";
import { useLiveQuery } from "dexie-react-hooks";
import About from "./About.js";
import Resources from "./Resources.jsx";
import Downloads from "./Downloads.jsx";
import Layout from "./includes/Layout.js";
import "./main.css";
import { MetaObject, db } from "./db.js";
import {
  Download,
  ResourceDownload,
  fetchBasic,
  fetchDownloads,
  fetchResourceDownloads,
} from "./api.js";

ReactDOM.createRoot(document.getElementById("root")).render(<App />);

function App() {
  let [sites, setSites] = useState<string[]>(null);
  let [updating, setUpdating] = useState(false);
  let [downloads, setDownloads] = useState<Download[]>([]);
  let [resourceDownloads, setResourceDownloads] = useState<ResourceDownload[]>(
    []
  );
  let resources = useLiveQuery(() => db.resources.toArray());
  let metaData = useLiveQuery(() => db.meta.toArray());
  resources = resources || [];
  let meta: MetaObject = !metaData
    ? {}
    : metaData.reduce((pv, v) => {
        pv[v.key] = v.value;
        return pv;
      }, {});

  useEffect(() => {
    (async () => {
      try {
        let { sites } = await fetchBasic();
        setSites(sites);
      } catch (e: any) {
        alert(`load error: ${e}`);
      }
    })();
  }, []);
  useEffect(() => {
    update();
    const intervalId = setInterval(update, 5000);
    return () => clearInterval(intervalId); //This is important
  }, [useState]);

  if (sites == null) {
    return <>Loading...</>;
  }

  return (
    <React.StrictMode>
      <Router>
        <Routes>
          <Route
            path="/"
            element={
              <Layout
                resources={resources}
                resourceDownloads={resourceDownloads}
              />
            }
          >
            <Route
              index
              element={
                <Resources sites={sites} resources={resources} meta={meta} />
              }
            />
            <Route path="/about" element={<About />} />
            <Route
              path="/downloads"
              element={
                <Downloads
                  resourceDownloads={resourceDownloads}
                  downloads={downloads}
                />
              }
            />
          </Route>
        </Routes>
      </Router>
    </React.StrictMode>
  );

  async function update() {
    if (updating) {
      return;
    }
    setUpdating(true);
    let err: any;
    try {
      let ds = await fetchDownloads();
      let rds = await fetchResourceDownloads();
      setDownloads(ds);
      setResourceDownloads(rds);
    } catch (e: any) {
      err = e;
    }
    setUpdating(false);
    if (err != null) {
      throw err;
    }
  }
}
