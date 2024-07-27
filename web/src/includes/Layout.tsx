import React from "react";
import { Outlet } from "react-router-dom";
import { Link } from "react-router-dom";
import { ResourceDownload } from "../api";
import { Resource } from "../db";

interface LayoutProps {
  resourceDownloads: ResourceDownload[];
  resources: Resource[];
}

export default function Layout({ resourceDownloads, resources }: LayoutProps) {
  return (
    <>
      <header>
        <ul className="list-none inline-flex space-x-1">
          <li>
            <Link to="/">Resources ({resources.length})</Link>
          </li>
          <li>
            <Link to="/downloads">Downloads ({resourceDownloads.length})</Link>
          </li>
          <li>
            <Link to="/about">About</Link>
          </li>
        </ul>
      </header>
      <main className="flex-1 flex flex-col">
        <Outlet />
      </main>
    </>
  );
}
