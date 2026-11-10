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

// One target's native-release-<goos>-<goarch>.json asset, written by the
// release workflow and read back by the Worker for every version it serves.
export interface ReleaseMetadata {
  readonly schema: 1;
  readonly target: string;
  readonly entry: ModuleEntry;
  readonly zip: { readonly name: string; readonly sha256: string; readonly size: number };
}

// The parts of dist/native-targets.json the Worker uses.
export interface NativeTable {
  readonly modulePrefix: string;
  readonly repository: string;
  readonly targets: readonly { readonly go: string }[];
}

// Returns the release asset's text, or undefined when it does not exist.
export type AssetLoader = (url: string) => Promise<string | undefined>;

export interface WorkerEnv {
  // The version @latest and @v/list advertise; set by the deploy workflow.
  readonly NATIVE_VERSION?: string;
}

export interface MetadataWorker {
  // True for module routes; everything else belongs to the documentation site.
  serves(request: Request): boolean;
  fetch(request: Request, env: WorkerEnv): Promise<Response>;
}

export interface SiteEnv extends WorkerEnv {
  readonly ASSETS: { fetch(request: Request): Promise<Response> };
}

export interface SiteWorker {
  fetch(request: Request, env: SiteEnv): Response | Promise<Response>;
}
