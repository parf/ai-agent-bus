// Its own test binary: it sets process environment, which no other test may share.
use agent_bus::*;
mod common;
use common::{SocketDir, bind};
use tokio::io::{AsyncReadExt, AsyncWriteExt};
use tokio::net::UnixListener;

/// Answers /status as `you`, whatever the request.
fn answer(listener: UnixListener, you: &'static str) {
    tokio::spawn(async move {
        while let Ok((mut socket, _)) = listener.accept().await {
            let mut buffer = [0; 4096];
            let _ = socket.read(&mut buffer).await;
            let body = format!("{{\"you\":\"{you}\"}}");
            let reply = format!(
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                body.len()
            );
            let _ = socket.write_all(reply.as_bytes()).await;
        }
    });
}

#[tokio::test]
async fn default_socket_follows_the_credential_given_to_the_builder() {
    let runtime = SocketDir::new();
    let root = &runtime.0;
    let dir = root.join("agent-bus");
    std::fs::create_dir_all(&dir).unwrap();
    let account = users::get_current_username().unwrap();
    answer(bind(dir.join("bus.sock")), "#worker@node");
    answer(
        bind(dir.join(format!("user-{}.sock", account.to_string_lossy()))),
        "account@node",
    );
    // SAFETY: the only test in this binary, set before any client reads the environment.
    unsafe {
        std::env::set_var("XDG_RUNTIME_DIR", root);
        std::env::remove_var("AGENT_BUS_TOKEN");
        std::env::remove_var("AGENT_BUS_ADDR");
    }
    let agent = Bus::builder("#worker@node")
        .credential("agent-token")
        .connect()
        .await;
    let account_bus = Bus::builder("account@node").connect().await;
    assert_eq!(agent.expect("an agent token reaches the agent socket").name(), "#worker@node");
    assert_eq!(account_bus.expect("no token reaches the account socket").name(), "account@node");
}
