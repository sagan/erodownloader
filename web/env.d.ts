declare module "process" {
  global {
    namespace NodeJS {
      interface ProcessEnv {
        NODE_ENV?: string;
      }
    }
  }
}

declare interface Window {
  __API__: string;
  __TOKEN__: string;
  __GET__: string;
}
