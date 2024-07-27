let apiUrl = "";
if (window.__API__) {
  apiUrl = window.__API__;
} else if (process.env.NODE_ENV == "development") {
  apiUrl = "http://" + location.hostname + ":6968/api";
} else {
  apiUrl = "/api";
}

interface Download {
  id: number;
  created_at: string;
  updated_at: string;
  download_id: string;
  file_id: string;
  identifier: string;
  site: string;
  file_url: string;
  filename: string;
  save_path: string;
  resource_id: string;
  status: string;
  client: string;
  note: string;
}

interface ResourceDownload {
  id: number;
  created_at: string;
  updated_at: string;
  resource_id: string;
  identifier: string;
  site: string;
  status: string;
  size: number;
  number: string;
  title: string;
  author: string;
  client: string;
  save_path: string;
  note: string;
  tags: string[];
}

interface SiteResource {
  id: string;
  author: string;
  number: string;
  size: number;
  title: string;
  tags: string[];
}

type SiteResourceKey = keyof SiteResource;

interface Basic {
  clients: string[];
  sites: string[];
}

export async function fetchApi<T>(params: { [key: string]: string }) {
  var data = new URLSearchParams();
  for (let [key, value] of Object.entries(params)) {
    if (Array.isArray(value)) {
      for (let v of value) {
        data.append(key, v);
      }
    } else {
      data.set(key, value);
    }
  }
  if (window.__TOKEN__) {
    data.set("token", window.__TOKEN__);
  }
  let res: Response;
  if (window.__GET__) {
    res = await fetch(apiUrl + "?" + data.toString(), {
      method: "GET",
      mode: "cors",
      cache: "no-cache",
    });
  } else {
    res = await fetch(apiUrl, {
      method: "POST",
      mode: "cors",
      cache: "no-cache",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: data.toString(),
    });
  }
  if (res.status != 200) {
    throw new Error(`status=${res.status}`);
  }
  let resdata = (await res.json()) as T;
  return resdata;
}

export async function fetchBasic() {
  return await fetchApi<Basic>({ func: "basic" });
}

export async function fetchDownloads() {
  return await fetchApi<Download[]>({ func: "downloads" });
}

export async function fetchResourceDownloads() {
  return await fetchApi<ResourceDownload[]>({ func: "resource_downloads" });
}

export async function fetchSiteResources(site: string) {
  return await fetchApi<SiteResource[]>({ func: "searchr", qs: "none", site });
}

export type {
  Download,
  ResourceDownload,
  SiteResource,
  SiteResourceKey,
  Basic,
};
