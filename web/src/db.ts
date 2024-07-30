import Dexie, { type EntityTable } from "dexie";

interface Resource {
  id?: number;
  site: string;
  title: string;
  number: string;
  author: string;
  tags: string[];
  resource_id: string;
  /** unix timestamp (seconds) */
  time: number;
}

interface Meta {
  id?: number;
  key: string;
  value: string;
}

interface MetaObject {
  [key: string]: string;
}

// https://dexie.org/docs/Tutorial/Svelte#using-typescript
const db = new Dexie("ErodownloaderDatabase") as Dexie & {
  resources: EntityTable<Resource, "id">;
  meta: EntityTable<Meta, "id">;
};

db.version(1).stores({
  resources: "++id, site, title, number, author, *tags", // Primary key and indexed props
  meta: "++id, &key",
});

export type { Resource, MetaObject };
export { db };
