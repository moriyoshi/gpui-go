fn main() {
    let root = std::path::PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../proto");
    let file = root.join("gpui/editor/v1/editor.proto");
    println!("cargo:rerun-if-changed={}", file.display());
    prost_build::Config::new()
        .compile_protos(&[file], &[root])
        .expect("compile editor protocol");
}
