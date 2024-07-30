import React, { useState } from "react";
import { Helmet } from "react-helmet";
import { Table, AutoSizer, Column, SortIndicator } from "react-virtualized";
import { MetaObject, Resource, db } from "./db";
import { SiteResourceKey, fetchSiteResources } from "./api";
import { format_date } from "./funcs";

interface ResourcesProps {
  sites: string[];
  resources: Resource[];
  meta: MetaObject;
}

export default function Resources({ sites, resources, meta }: ResourcesProps) {
  const [sort, setSort] = useState<SiteResourceKey>("number");
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(false);
  const [site, setSite] = useState(sites[0] || "");
  const siteResources = resources.filter((r) => r.site == site);
  siteResources.sort((a, b) => {
    if (a[sort] < b[sort]) {
      return -1;
    } else if (a[sort] > b[sort]) {
      return 1;
    }
    return 0;
  });

  return (
    <>
      <Helmet>
        <title>Resources</title>
      </Helmet>
      <form>
        <p>
          <label>
            Site:&nbsp;
            <select
              name="site"
              value={site}
              onChange={(e) => setSite(e.target.value)}
            >
              {sites.map((site) => (
                <option key={site}>{site}</option>
              ))}
            </select>
            <button type="button" onClick={queryResources}>
              Query
            </button>
          </label>
          <span>
            {(!meta || !siteResources) && "Loading local data..."}
            {loading && "Loading..."}
            {!!error && error.toString()}
          </span>
        </p>
      </form>
      {!!siteResources && (
        <>
          <p>Resources: {siteResources.length}</p>
          <article className="flex-1">
            <AutoSizer>
              {({ width, height }) => (
                <Table
                  ref="Table"
                  disableHeader={false}
                  headerClassName="header"
                  headerHeight={20}
                  width={width}
                  height={height}
                  noRowsRenderer={() => <>No data</>}
                  overscanRowCount={1000}
                  rowClassName="flex"
                  rowHeight={20}
                  rowGetter={({ index }) => siteResources[index]}
                  rowCount={siteResources.length}
                  scrollToIndex={-1}
                  sort={() => {}}
                  sortBy={sort}
                  sortDirection="ASC"
                >
                  <Column
                    dataKey="number"
                    disableSort={false}
                    className="column column-number"
                    headerRenderer={headerRenderer}
                    width={100}
                  />
                  <Column
                    dataKey="title"
                    disableSort={false}
                    className="column column-title"
                    headerRenderer={headerRenderer}
                    flexGrow={1}
                    width={300}
                  />
                  <Column
                    dataKey="author"
                    disableSort={false}
                    className="column column-author"
                    headerRenderer={headerRenderer}
                    flexGrow={1}
                    width={200}
                  />
                  <Column
                    dataKey="size"
                    disableSort={false}
                    className="column column-size"
                    headerRenderer={headerRenderer}
                    width={100}
                  />
                  <Column
                    dataKey="tags"
                    disableSort={false}
                    className="column column-tags"
                    headerRenderer={headerRenderer}
                    cellRenderer={({ cellData }) => (
                      <>{(cellData || []).join(", ")}</>
                    )}
                    flexGrow={1}
                    width={300}
                  />
                  <Column
                    dataKey="time"
                    disableSort={false}
                    className="column column-time"
                    headerRenderer={headerRenderer}
                    cellRenderer={({ cellData }) => (
                      <>{format_date(cellData)}</>
                    )}
                    width={100}
                  />
                  <Column
                    dataKey="actions"
                    disableSort={true}
                    className="column column-actions"
                    headerRenderer={headerRenderer}
                    cellRenderer={({ rowData, rowIndex }) => <>{rowIndex}</>}
                    width={100}
                  />
                </Table>
              )}
            </AutoSizer>
          </article>
        </>
      )}
    </>
  );

  function headerRenderer({ dataKey, sortBy, sortDirection }) {
    let title = dataKey ? dataKey.toLowerCase() : "Actions";
    return (
      <div>
        {title}
        {sortBy === dataKey && <SortIndicator sortDirection={sortDirection} />}
      </div>
    );
  }

  async function queryResources() {
    let _site = site;
    try {
      let siteResources = await fetchSiteResources(_site);
      let resources: Resource[] = [];
      siteResources.forEach((r) => {
        resources.push({
          resource_id: r.id,
          site: _site,
          title: r.title,
          tags: r.tags,
          number: r.number,
          author: r.author,
          time: r.time,
        });
      });
      await db.transaction("rw", [db.resources], async () => {
        await db.resources.where({ site: _site }).delete();
        await db.resources.bulkAdd(resources);
      });
    } catch (e: any) {
      console.log("error", e);
      setError(e);
    }
  }
}
