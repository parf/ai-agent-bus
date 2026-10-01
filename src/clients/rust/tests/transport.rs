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
    let directory =
        std::path::PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../tmp/rust-client-tests");
    std::fs::create_dir_all(&directory).unwrap();
    let path = directory
        .canonicalize()
        .unwrap()
        .join(format!("{}.sock", &uuid::Uuid::new_v4().to_string()[..12]));
    let listener = tokio::net::UnixListener::bind(&path).unwrap();
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
    std::fs::remove_file(path).unwrap();
}
