export interface ModuleRelease {
  readonly time: string;
  readonly tag: string;
  readonly asset: string;
  // The exact published go.mod bytes, not a generated approximation.
  readonly goMod: string;
}

export interface ModuleEntry {
  readonly path: string;
  readonly repository: string;
  readonly versions: Readonly<Record<string, ModuleRelease>>;
}

export interface MetadataWorker {
  // True for module routes; everything else belongs to the documentation site.
  serves(request: Request): boolean;
  fetch(request: Request): Response;
}

export interface SiteEnv {
  readonly ASSETS: { fetch(request: Request): Promise<Response> };
}

export interface SiteWorker {
  fetch(request: Request, env: SiteEnv): Response | Promise<Response>;
}
