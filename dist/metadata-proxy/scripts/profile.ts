// Bao v0.0.3's verified linker profile. Ordinary repository Go builds retain
// their separate go.mod toolchain pin.
export const nativeGoVersion = "1.26.10";

export function nativeGoEnvironment(): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = { ...process.env, GOTOOLCHAIN: "local", GOENV: "off" };
  // Let the selected Go executable locate its own SDK, including when a shell
  // has exported GOROOT for the repository's older Go installation.
  delete env.GOROOT;
  return env;
}
