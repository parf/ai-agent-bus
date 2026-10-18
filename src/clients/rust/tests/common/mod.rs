use std::{
    os::unix::{ffi::OsStrExt, fs::DirBuilderExt},
    path::{Path, PathBuf},
};

use tokio::net::UnixListener;

// check.sh sets TMPDIR below the checkout. Nesting sockets there can exceed
// sockaddr_un's pathname limit, even when ordinary fixture files fit.
pub struct SocketDir(pub PathBuf);
impl SocketDir {
    pub fn new() -> Self {
        let root = Path::new("/tmp").join(format!("ab-{}", uuid::Uuid::new_v4().simple()));
        std::fs::DirBuilder::new()
            .mode(0o700)
            .create(&root)
            .expect("create a private short socket fixture directory");
        Self(root)
    }
}
impl Drop for SocketDir {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

pub fn bind(path: PathBuf) -> UnixListener {
    // 103 bytes fits both Linux (107) and the shorter BSD/macOS pathname limit.
    const MAX_PATH_BYTES: usize = 103;
    let bytes = path.as_os_str().as_bytes().len();
    assert!(
        bytes <= MAX_PATH_BYTES,
        "socket fixture path is {bytes} bytes; portable Unix socket limit is {MAX_PATH_BYTES}: {}",
        path.display()
    );
    UnixListener::bind(&path)
        .unwrap_or_else(|err| panic!("bind socket fixture {}: {err}", path.display()))
}
