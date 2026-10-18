mod common;
use common::{SocketDir, bind};
use agent_bus::*;
use sha2::{Digest, Sha256};
use std::{
    sync::{
        Arc,
        atomic::{AtomicUsize, Ordering},
    },
    time::Duration,
};
use tokio::io::{AsyncRead, AsyncReadExt, AsyncWrite, AsyncWriteExt};
async fn status<S: AsyncRead + AsyncWrite + Unpin>(mut socket: S, count: Arc<AtomicUsize>) {
    let mut buffer = [0; 4096];
    if let Ok(n) = socket.read(&mut buffer).await {
        if n == 0 {
            return;
        }
        let text = String::from_utf8_lossy(&buffer[..n]).to_lowercase();
        assert!(text.contains("x-agent-bus-token: token-secret"));
        count.fetch_add(1, Ordering::SeqCst);
        socket.write_all(b"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 17\r\nConnection: close\r\n\r\n{\"you\":\"#worker\"}").await.unwrap();
    }
}
#[tokio::test]
async fn pinned_tls_accepts_exact_certificate_and_rejects_before_credentials() {
    let generated = rcgen::generate_simple_self_signed(vec!["localhost".into()]).unwrap();
    let der = generated.cert.der().clone();
    let hash = Sha256::digest(der.as_ref());
    let pin = format!(
        "sha256:{}",
        hash.iter()
            .map(|n| format!("{n:02X}"))
            .collect::<Vec<_>>()
            .join(":")
    );
    let key = rustls::pki_types::PrivateKeyDer::Pkcs8(rustls::pki_types::PrivatePkcs8KeyDer::from(
        generated.signing_key.serialize_der(),
    ));
    let config = rustls::ServerConfig::builder_with_provider(Arc::new(
        rustls::crypto::ring::default_provider(),
    ))
    .with_safe_default_protocol_versions()
    .unwrap()
    .with_no_client_auth()
    .with_single_cert(vec![der], key)
    .unwrap();
    let acceptor = tokio_rustls::TlsAcceptor::from(Arc::new(config));
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = format!(
        "https://localhost:{}",
        listener.local_addr().unwrap().port()
    );
    let count = Arc::new(AtomicUsize::new(0));
    let received = count.clone();
    tokio::spawn(async move {
        loop {
            let (socket, _) = listener.accept().await.unwrap();
            let acceptor = acceptor.clone();
            let count = received.clone();
            tokio::spawn(async move {
                if let Ok(socket) = acceptor.accept(socket).await {
                    status(socket, count).await;
                }
            });
        }
    });
    let bus = Bus::builder("#worker")
        .transport(&address)
        .credential("token-secret")
        .tls_fingerprint(&pin)
        .connect()
        .await
        .unwrap();
    assert_eq!(count.load(Ordering::SeqCst), 1);
    bus.close().await.unwrap();
    let result = Bus::builder("#worker")
        .transport(&address)
        .credential("token-secret")
        .tls_fingerprint("00".repeat(32))
        .connect()
        .await;
    assert!(matches!(result, Err(BusError::Transport(_))));
    assert_eq!(
        count.load(Ordering::SeqCst),
        1,
        "wrong pin transmitted HTTP credentials"
    );
    let result = Bus::builder("#other")
        .transport(&address)
        .credential("token-secret")
        .tls_fingerprint(&pin)
        .connect()
        .await;
    assert!(matches!(result, Err(BusError::IdentityMismatch { .. })));
}
#[tokio::test]
async fn unix_socket_uses_same_authenticated_http_contract() {
    let directory = SocketDir::new();
    let path = directory.0.join("bus.sock");
    let listener = bind(path.clone());
    let count = Arc::new(AtomicUsize::new(0));
    let received = count.clone();
    let server = tokio::spawn(async move {
        let (socket, _) = listener.accept().await.unwrap();
        status(socket, received).await;
    });
    let bus = Bus::builder("#worker")
        .unix_socket(&path)
        .credential("token-secret")
        .timeout(Duration::from_secs(1))
        .connect()
        .await
        .unwrap();
    assert_eq!(count.load(Ordering::SeqCst), 1);
    bus.close().await.unwrap();
    server.await.unwrap();
}

async fn loss_fixture() -> (Bus, Arc<AtomicUsize>, Arc<AtomicUsize>) {
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = format!("http://{}", listener.local_addr().unwrap());
    let writes = Arc::new(AtomicUsize::new(0));
    let takes = Arc::new(AtomicUsize::new(0));
    let write_count = writes.clone();
    let take_count = takes.clone();
    tokio::spawn(async move {
        loop {
            let (mut socket, _) = listener.accept().await.unwrap();
            let writes = write_count.clone();
            let takes = take_count.clone();
            tokio::spawn(async move {
                let mut buffer = Vec::new();
                let mut chunk = [0; 4096];
                loop {
                    let n = socket.read(&mut chunk).await.unwrap();
                    if n == 0 {
                        return;
                    }
                    buffer.extend_from_slice(&chunk[..n]);
                    if let Some(end) = buffer.windows(4).position(|x| x == b"\r\n\r\n") {
                        let headers = String::from_utf8_lossy(&buffer[..end]).to_lowercase();
                        let length = headers
                            .lines()
                            .find_map(|line| {
                                line.strip_prefix("content-length:")
                                    .and_then(|n| n.trim().parse::<usize>().ok())
                            })
                            .unwrap_or(0);
                        if buffer.len() >= end + 4 + length {
                            break;
                        }
                    }
                }
                let request = String::from_utf8_lossy(&buffer);
                if request.starts_with("GET /status ") {
                    socket.write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 17\r\nConnection: close\r\n\r\n{\"you\":\"#worker\"}").await.unwrap();
                } else if request.starts_with("POST /try-lock") {
                    let body = b"{\"record\":\"jobs\",\"name\":\"build\",\"holder\":\"#worker\"}";
                    socket.write_all(format!("HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",body.len()).as_bytes()).await.unwrap();
                    socket.write_all(body).await.unwrap();
                } else if request.starts_with("GET /consume") {
                    takes.fetch_add(1, Ordering::SeqCst);
                    // The message was taken but its response body is truncated.
                    socket.write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 100\r\nConnection: close\r\n\r\n{\"message_id\":").await.unwrap();
                } else if request.starts_with("POST ") {
                    writes.fetch_add(1, Ordering::SeqCst);
                    // Request body arrived in full; drop before confirming its outcome.
                }
            });
        }
    });
    let bus = Bus::builder("#worker")
        .transport(address)
        .credential("token-secret")
        .timeout(Duration::from_millis(300))
        .connect()
        .await
        .unwrap();
    (bus, writes, takes)
}
#[tokio::test]
async fn lost_write_and_consume_responses_are_uncertain_and_never_replayed() {
    let (bus, writes, takes) = loss_fixture().await;
    assert!(matches!(
        bus.send("#worker", "write", SendOptions::default()).await,
        Err(BusError::UncertainOutcome { .. })
    ));
    assert_eq!(writes.load(Ordering::SeqCst), 1);
    assert!(matches!(
        bus.consume(ConsumeOptions {
            wait: Duration::ZERO,
            ..Default::default()
        })
        .await,
        Err(BusError::UncertainOutcome { .. })
    ));
    assert_eq!(takes.load(Ordering::SeqCst), 1);
    assert!(matches!(
        bus.record("jobs").unwrap().lookup().await,
        Err(BusError::Transport(_))
    ));
    bus.close().await.unwrap();
}
#[tokio::test]
async fn leases_retire_after_uncertain_release_or_extend() {
    let (bus, writes, _) = loss_fixture().await;
    let locks = bus.record("jobs").unwrap().locks();
    let mut lease = locks
        .try_acquire("build", Duration::from_secs(2))
        .await
        .unwrap();
    assert!(matches!(
        lease.release().await,
        Err(BusError::UncertainOutcome { .. })
    ));
    assert!(matches!(
        lease.release().await,
        Err(BusError::Validation(_))
    ));
    assert_eq!(
        writes.load(Ordering::SeqCst),
        1,
        "lease replayed an uncertain release"
    );
    let mut lease = locks
        .try_acquire("build", Duration::from_secs(2))
        .await
        .unwrap();
    assert!(matches!(
        lease.extend(Duration::from_secs(1)).await,
        Err(BusError::UncertainOutcome { .. })
    ));
    assert!(matches!(
        lease.release().await,
        Err(BusError::Validation(_))
    ));
    assert_eq!(
        writes.load(Ordering::SeqCst),
        2,
        "lease mutated a hold after an uncertain extend"
    );
    bus.close().await.unwrap();
}
