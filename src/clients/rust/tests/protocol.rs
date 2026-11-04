//! The version rule and the refusal `kind`, against a daemon that answers
//! whatever each case says (docs/09-daemon-api.md, `GET /identity`).
use agent_bus::*;
use std::sync::Arc;
use tokio::io::{AsyncReadExt, AsyncWriteExt};

/// Serves `identity` for GET /identity, `{"you":"#worker"}` for GET /status
/// and `other` (status, body) for everything else.
async fn daemon(identity: &str, other: (u16, &str)) -> String {
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = format!("http://{}", listener.local_addr().unwrap());
    let identity = identity.to_owned();
    let other = (other.0, other.1.to_owned());
    tokio::spawn(async move {
        loop {
            let (mut socket, _) = listener.accept().await.unwrap();
            let identity = identity.clone();
            let other = other.clone();
            tokio::spawn(async move {
                let mut buffer = [0; 4096];
                let n = socket.read(&mut buffer).await.unwrap_or(0);
                let request = String::from_utf8_lossy(&buffer[..n]).into_owned();
                let (code, body) = if request.starts_with("GET /status ") {
                    (200, "{\"you\":\"#worker\"}".to_owned())
                } else if request.starts_with("GET /identity ") {
                    (200, identity)
                } else {
                    other
                };
                let head = format!(
                    "HTTP/1.1 {code} X\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    body.len()
                );
                let _ = socket.write_all(head.as_bytes()).await;
                let _ = socket.write_all(body.as_bytes()).await;
            });
        }
    });
    address
}

async fn connect(address: &str) -> Result<Bus> {
    Bus::builder("#worker")
        .transport(address)
        .credential("t")
        .connect()
        .await
}

#[tokio::test]
async fn current_protocol_connects() {
    connect(&daemon(r#"{"protocol":1,"protocol_min":1}"#, (200, "{}")).await).await.unwrap();
}

#[tokio::test]
async fn unsupported_protocol_is_refused() {
    for identity in [r#"{"protocol":0,"protocol_min":0}"#, r#"{}"#, r#"{"protocol":2,"protocol_min":1}"#, r#"{"protocol":1}"#, r#"{"protocol":1,"protocol_min":0}"#] {
        let error = connect(&daemon(identity, (200, "{}")).await).await.unwrap_err();
        assert!(matches!(error, BusError::Protocol(_)), "{identity}: {error:?}");
    }
}

#[tokio::test]
async fn a_client_older_than_the_daemons_minimum_is_refused_at_connect() {
    let address = daemon(r#"{"protocol":3,"protocol_min":2}"#, (200, "{}")).await;
    let error = connect(&address).await.unwrap_err();
    assert!(matches!(error, BusError::DaemonTooNew { protocol_min: 2, client: 1 }), "{error:?}");
    assert_eq!(error.to_string(), "daemon too new for this client");
}

#[tokio::test]
async fn a_malformed_protocol_answer_is_fatal_and_names_the_member() {
    for (identity, member) in [
        (r#"{"protocol":"1","protocol_min":1}"#, "protocol"),
        (r#"{"protocol":1.5,"protocol_min":1}"#, "protocol"),
        (r#"{"protocol":-1,"protocol_min":0}"#, "protocol"),
        (r#"{"protocol":true,"protocol_min":0}"#, "protocol"),
        (r#"{"protocol":null,"protocol_min":0}"#, "protocol"),
        (r#"{"protocol":2,"protocol_min":"x"}"#, "protocol_min"),
        (r#"{"protocol":2,"protocol_min":-1}"#, "protocol_min"),
        (r#"{"protocol":1,"protocol_min":2}"#, "protocol_min"),
    ] {
        let error = connect(&daemon(identity, (200, "{}")).await).await.unwrap_err();
        assert!(matches!(error, BusError::Protocol(_)), "{identity}: {error:?}");
        assert!(error.to_string().contains(member), "{identity}: {error}");
    }
}

#[tokio::test]
async fn a_refusal_carries_the_kind_the_daemon_sent() {
    for kind in ["acl", "authority", "busy", "credential", "enrolment", "full", "malformed", "name-taken", "second-reader", "suspended", "unavailable", "not_found", "no_route"] {
        let body = format!(r#"{{"error":"no","kind":"{kind}"}}"#);
        let bus = connect(&daemon(r#"{"protocol":1,"protocol_min":1}"#, (403, &body)).await)
            .await
            .unwrap();
        match bus.records(&[]).await {
            Err(BusError::Refusal { status: 403, kind: got, message }) => {
                assert_eq!(got.as_deref(), Some(kind));
                assert_eq!(message, "no");
            }
            other => panic!("{kind}: {other:?}"),
        }
    }
}

#[tokio::test]
async fn a_missing_route_preserves_its_cause() {
    let body = r#"{"error":"Not Found","kind":"no_route"}"#;
    let current = connect(&daemon(r#"{"protocol":1,"protocol_min":1}"#, (404, body)).await).await.unwrap();
    match current.records(&[]).await {
        Err(BusError::Refusal { kind, message, .. }) => {
            assert_eq!(kind.as_deref(), Some("no_route"));
            assert_eq!(message, "Not Found");
        }
        other => panic!("{other:?}"),
    }
    // Missing data is not a missing route.
    let data = r#"{"error":"no such name","kind":"not_found"}"#;
    let old = connect(&daemon(r#"{"protocol":1,"protocol_min":1}"#, (404, data)).await).await.unwrap();
    match old.records(&[]).await {
        Err(BusError::Refusal { kind, message, .. }) => {
            assert_eq!(kind.as_deref(), Some("not_found"));
            assert_eq!(message, "no such name");
        }
        other => panic!("{other:?}"),
    }
}

#[tokio::test]
async fn a_server_error_without_a_kind_is_an_internal_daemon_error() {
    let bus = connect(&daemon(r#"{"protocol":1,"protocol_min":1}"#, (500, r#"{"error":"boom: secret path /var/x"}"#)).await)
        .await
        .unwrap();
    match bus.records(&[]).await {
        Err(BusError::Refusal { status: 500, kind: None, message }) => {
            assert_eq!(message, "internal daemon error");
        }
        other => panic!("{other:?}"),
    }
    // A 503 that names its kind is a refusal, not an internal error.
    let bus = connect(&daemon(r#"{"protocol":1,"protocol_min":1}"#, (503, r#"{"error":"store down","kind":"unavailable"}"#)).await)
        .await
        .unwrap();
    match bus.records(&[]).await {
        Err(BusError::Refusal { status: 503, kind, message }) => {
            assert_eq!(kind.as_deref(), Some("unavailable"));
            assert_eq!(message, "store down");
        }
        other => panic!("{other:?}"),
    }
}

// The version rule runs before anything else is asked of the daemon: a daemon
// this client cannot speak is never sent a /status, and the first request
// of every connection is GET /identity (docs/09-daemon-api.md#the-client-contract).
#[tokio::test]
async fn the_first_request_is_the_identity_and_a_refused_daemon_sees_nothing_else() {
    let seen = std::sync::Arc::new(std::sync::Mutex::new(Vec::<String>::new()));
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = format!("http://{}", listener.local_addr().unwrap());
    let identity = Arc::new(std::sync::Mutex::new(String::new()));
    let (log, answer) = (seen.clone(), identity.clone());
    tokio::spawn(async move {
        loop {
            let (mut socket, _) = listener.accept().await.unwrap();
            let (log, answer) = (log.clone(), answer.clone());
            tokio::spawn(async move {
                let mut buffer = [0; 4096];
                let n = socket.read(&mut buffer).await.unwrap_or(0);
                let request = String::from_utf8_lossy(&buffer[..n]).into_owned();
                let path = request.split_whitespace().nth(1).unwrap_or("").to_owned();
                log.lock().unwrap().push(path.clone());
                let body = if path == "/identity" { answer.lock().unwrap().clone() } else { "{\"you\":\"#worker\"}".to_owned() };
                let head = format!("HTTP/1.1 200 X\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len());
                let _ = socket.write_all(head.as_bytes()).await;
                let _ = socket.write_all(body.as_bytes()).await;
            });
        }
    });
    *identity.lock().unwrap() = r#"{"protocol":2,"protocol_min":2}"#.to_owned();
    assert!(connect(&address).await.is_err());
    assert_eq!(*seen.lock().unwrap(), vec!["/identity".to_owned()], "a daemon this client refuses was sent another request");
    seen.lock().unwrap().clear();
    *identity.lock().unwrap() = r#"{"protocol":1,"protocol_min":1}"#.to_owned();
    connect(&address).await.unwrap();
    assert_eq!(seen.lock().unwrap()[0], "/identity", "the first request of a connection is /identity");
}
