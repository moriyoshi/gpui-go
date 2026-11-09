fn main() {
    let root = std::path::PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../proto");
    let names = [
        "types",
        "feature",
        "automation",
        "style",
        "action",
        "resource",
        "element",
        "entity",
        "window",
        "event",
        "app",
        "protocol",
    ];
    let files: Vec<_> = names
        .iter()
        .map(|name| root.join(format!("gpui/v1/{name}.proto")))
        .collect();
    for file in &files {
        println!("cargo:rerun-if-changed={}", file.display());
    }
    prost_build::Config::new()
        .compile_protos(&files, &[root])
        .expect("compile GPUI protocol");
}
