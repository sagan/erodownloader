import React, { useState } from "react";
import { Helmet } from "react-helmet";
import { Download, ResourceDownload } from "./api";

interface DownloadsProps {
  downloads: Download[];
  resourceDownloads: ResourceDownload[];
}

export default function Downloads({
  downloads,
  resourceDownloads,
}: DownloadsProps) {
  let [activeResourceId, setActiveResourceId] = useState("active");
  let [statusFilter, setStatusFilter] = useState("active");

  let activeResource = resourceDownloads.find(
    (r) => r.resource_id == activeResourceId
  );
  let activeFiles: Download[] = [];
  if (activeResource) {
    activeFiles = downloads.filter(
      (d) => d.resource_id == activeResource.resource_id
    );
  }

  let counts = {
    active: 0,
    downloading: 0,
    error: 0,
    queue: 0,
    completed: 0,
  };
  for (let r of resourceDownloads) {
    if (r.status == "downloading") {
      counts.downloading++;
      counts.active++;
    } else if (r.status == "") {
      counts.queue++;
    } else if (r.status == "completed") {
      counts.completed++;
    } else if (r.status == "error") {
      counts.error++;
    }
  }

  let resourcesDisplay = resourceDownloads.filter((r) => {
    if (statusFilter == "none") {
      return true;
    }
    if (statusFilter == "active") {
      return r.status == "downloading" || r.status == "";
    }
    if (statusFilter == "queue") {
      return r.status == "";
    }
    return r.status == statusFilter;
  });

  return (
    <>
      <Helmet>
        <title>Downloads</title>
      </Helmet>
      <div className="grow-4 h-0 flex flex-col">
        <p>
          Resources:&nbsp;
          <span className="space-x-1">
            <a onClick={() => setStatusFilter("none")}>
              All ({resourceDownloads.length})
            </a>
            <a onClick={() => setStatusFilter("active")}>
              Active ({counts.active})
            </a>
            <a onClick={() => setStatusFilter("downloading")}>
              Downloading ({counts.downloading})
            </a>
            <a onClick={() => setStatusFilter("queue")}>
              Queue ({counts.queue})
            </a>
            <a onClick={() => setStatusFilter("error")}>
              Error ({counts.error})
            </a>
            <a onClick={() => setStatusFilter("completed")}>
              Completed ({counts.completed})
            </a>
          </span>
        </p>
        <div className="flex">
          <div className="w-1/12">Site</div>
          <div className="w-1/12">Number</div>
          <div className="w-1/12">Status</div>
          <div className="w-1/12">Size</div>
          <div className="w-5/12">Title</div>
          <div className="w-3/12">Note</div>
        </div>
        <div className="flex-col overflow-auto">
          <div>
            {resourcesDisplay.map((r) => {
              return (
                <div
                  className="flex"
                  key={r.id}
                  onClick={() => {
                    console.log("select", r.resource_id);
                    setActiveResourceId(r.resource_id);
                  }}
                >
                  <div className="w-1/12">{r.site}</div>
                  <div className="w-1/12">{r.number}</div>
                  <div className="w-1/12">{r.status}</div>
                  <div className="w-1/12">{r.size}</div>
                  <div className="w-5/12">{r.title}</div>
                  <div className="w-3/12">{r.note}</div>
                </div>
              );
            })}
          </div>
        </div>
      </div>
      <div className="grow-1 h-0 flex flex-col">
        <p>Files</p>
        <div className="flex">
          <div className="w-1/12">Status</div>
          <div className="w-2/12">TaskId</div>
          <div className="w-6/12">Name</div>
          <div className="w-3/12">Note</div>
        </div>
        <div className="flex-col overflow-auto">
          <div>
            {activeFiles.map((d, index) => {
              return (
                <div className="flex" key={index}>
                  <div className="w-1/12">{d.status}</div>
                  <div className="w-2/12">{d.download_id}</div>
                  <div className="w-6/12">{d.filename}</div>
                  <div className="w-3/12">{d.note}</div>
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </>
  );
}
