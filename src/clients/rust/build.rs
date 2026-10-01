fn main() {
    let source = std::path::Path::new("../../internal/version/VERSION");
    println!("cargo:rerun-if-changed={}", source.display());
    let version = std::fs::read_to_string(source)
        .expect("build from the agent-bus checkout; canonical VERSION is required");
    println!("cargo:rustc-env=AGENT_BUS_VERSION={}", version.trim());
}
