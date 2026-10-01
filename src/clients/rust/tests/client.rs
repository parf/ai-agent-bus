use agent_bus::*;
use axum::{
    Json, Router,
    body::Body,
    extract::{Query, State},
    http::{Request, StatusCode},
    response::{IntoResponse, Response},
    routing::{get, post},
};
use serde_json::{Value, json};
use std::{
    collections::{HashMap, VecDeque},
    sync::{
        Arc, Mutex,
        atomic::{AtomicUsize, Ordering},
    },
    time::Duration,
};
use tokio::sync::{Notify, Semaphore};
struct Fixture {
    queues: Mutex<HashMap<String, VecDeque<Value>>>,
    sent: Mutex<Vec<Value>>,
    configs: AtomicUsize,
    config_started: Notify,
    config_release: Semaphore,
    hold_config: AtomicUsize,
    secret_reads: AtomicUsize,
    consume_filters: Mutex<Vec<HashMap<String, String>>>,
    consume_stop: Mutex<Option<CancellationToken>>,
    notify: Notify,
}
impl Default for Fixture {
    fn default() -> Self {
        Self {
            queues: Mutex::default(),
            sent: Mutex::default(),
            configs: AtomicUsize::new(0),
            config_started: Notify::new(),
            config_release: Semaphore::new(0),
            hold_config: AtomicUsize::new(0),
            secret_reads: AtomicUsize::new(0),
            consume_filters: Mutex::default(),
            consume_stop: Mutex::default(),
            notify: Notify::new(),
        }
    }
}
impl Fixture {
    fn push(&self, name: &str, msg: Value) {
        self.queues
            .lock()
            .unwrap()
            .entry(name.into())
            .or_default()
            .push_back(msg);
        self.notify.notify_waiters();
    }
}
async fn status() -> Json<Value> {
    Json(json!({"you":"#worker"}))
}
async fn config(State(s): State<Arc<Fixture>>) -> Json<Value> {
    let n = s.configs.fetch_add(1, Ordering::SeqCst) + 1;
    if s.hold_config.load(Ordering::SeqCst) == n {
        s.config_started.notify_one();
        s.config_release.acquire().await.unwrap().forget();
    }
    Json(json!({"generation":n}))
}
async fn secret(State(s): State<Arc<Fixture>>) -> Response {
    if s.secret_reads.fetch_add(1, Ordering::SeqCst) == 0 {
        (StatusCode::FORBIDDEN, Json(json!({"error":"private"}))).into_response()
    } else {
        "TOKEN=shh\n".into_response()
    }
}
async fn send(State(s): State<Arc<Fixture>>, Json(mut v): Json<Value>) -> Json<Value> {
    s.sent.lock().unwrap().push(v.clone());
    v["message_id"] = json!(format!("m{}", s.sent.lock().unwrap().len()));
    v["from"] = json!("#worker");
    if v["to"] == "#echo" {
        let reply = v.get("reply_to");
        let inbox = reply.and_then(|r| r["name"].as_str()).unwrap_or("#worker");
        let topic = reply
            .and_then(|r| r["topic"].as_str())
            .or_else(|| v["topic"].as_str())
            .unwrap_or("");
        let tag = reply
            .and_then(|r| r["tag"].as_str())
            .or_else(|| v["tag"].as_str())
            .unwrap_or("");
        let base = json!({"message_id":"answer","from":"#echo","to":inbox,"topic":topic,"tag":tag,"body":"","re":v["message_id"]});
        let mut ack = base.clone();
        ack["receipt"] = json!("ack");
        s.push(inbox, ack);
        let mut response = base;
        if v["body"] == "done" {
            response["receipt"] = json!("done");
        } else {
            response["body"] = json!(v["body"].as_str().unwrap_or(""));
        }
        s.push(inbox, response);
    } else if v["to"] == "#ack-only" {
        s.push("#worker",json!({"message_id":"ack","from":"#ack-only","to":"#worker","topic":v["topic"],"tag":v["tag"],"body":"","receipt":"ack","re":v["message_id"]}));
    }
    if v["body"] == "delayed-post" {
        tokio::time::sleep(Duration::from_millis(180)).await;
    }
    Json(v)
}
async fn consume(
    State(s): State<Arc<Fixture>>,
    Query(q): Query<HashMap<String, String>>,
) -> Response {
    s.consume_filters.lock().unwrap().push(q.clone());
    let inbox = q.get("inbox").map(String::as_str).unwrap_or("#worker");
    for _ in 0..10 {
        let value = {
            let mut qs = s.queues.lock().unwrap();
            let queue = qs.entry(inbox.into()).or_default();
            let index = queue.iter().position(|v| {
                q.get("topic")
                    .is_none_or(|x| v["topic"].as_str().unwrap_or("") == x)
                    && q.get("tag")
                        .is_none_or(|x| v["tag"].as_str().unwrap_or("") == x)
            });
            index.and_then(|i| queue.remove(i))
        };
        if let Some(v) = value {
            if let Some(stop) = s.consume_stop.lock().unwrap().take() {
                stop.cancel();
            }
            return Json(v).into_response();
        }
        tokio::time::sleep(Duration::from_millis(5)).await;
    }
    StatusCode::NO_CONTENT.into_response()
}
async fn inc(Json(v): Json<Value>) -> Response {
    assert!(v.get("kind").is_none());
    assert_eq!(v["record"], "jobs");
    Json(json!({"value":i64::MAX})).into_response()
}
async fn kv(Query(q): Query<HashMap<String, String>>) -> Json<Value> {
    assert_eq!(q["record"], "jobs");
    Json(json!({"value_base64":"AP8="}))
}
async fn kv_set(Json(v): Json<Value>) -> Json<Value> {
    assert_eq!(v["value_base64"], "AP8=");
    assert_eq!(v["how"], "add");
    Json(json!({}))
}
async fn busy() -> Response {
    (StatusCode::CONFLICT, Json(json!({"error":"already held"}))).into_response()
}
async fn drop_write(State(s): State<Arc<Fixture>>, request: Request<Body>) -> Response {
    assert_eq!(request.headers()["x-agent-bus-token"], "test-token");
    s.sent.lock().unwrap().push(json!({"uncertain":true}));
    (
        StatusCode::SERVICE_UNAVAILABLE,
        Json(json!({"error":"gone"})),
    )
        .into_response()
}
async fn fixture() -> (Bus, Arc<Fixture>) {
    let s = Arc::new(Fixture::default());
    let app = Router::new()
        .route("/status", get(status))
        .route(
            "/identity",
            get(|| async { Json(json!({"node":"fixture"})) }),
        )
        .route("/config", get(config))
        .route("/secret", get(secret))
        .route("/send", post(send))
        .route("/consume", get(consume))
        .route("/kv", get(kv))
        .route("/kv/set", post(kv_set))
        .route("/kv/inc", post(inc))
        .route("/try-lock", post(busy))
        .route("/manage", post(drop_write))
        .with_state(s.clone());
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let addr = listener.local_addr().unwrap();
    tokio::spawn(async move {
        axum::serve(listener, app).await.unwrap();
    });
    let bus = Bus::builder("#worker")
        .transport(format!("http://{addr}"))
        .credential("test-token")
        .timeout(Duration::from_secs(1))
        .connect()
        .await
        .unwrap();
    (bus, s)
}
fn request(body: &str, tag: &str) -> Value {
    json!({"message_id":tag,"from":"alice","to":"#worker","topic":"work","tag":tag,"body":body,"roles":["owner"],"deadline":"2030-01-01T00:00:00Z"})
}
#[tokio::test]
async fn lazy_cache_and_failure_not_cached() {
    let (bus, s) = fixture().await;
    let mut tasks = Vec::new();
    for _ in 0..10 {
        let b = bus.clone();
        tasks.push(tokio::spawn(
            async move { b.config().await.unwrap().unwrap() },
        ));
    }
    for t in tasks {
        assert_eq!(t.await.unwrap()["generation"], 1);
    }
    assert_eq!(s.configs.load(Ordering::SeqCst), 1);
    assert!(matches!(
        bus.secret().await,
        Err(BusError::Refusal { status: 403, .. })
    ));
    let secret = bus.secret().await.unwrap();
    assert_eq!(secret.bytes(), b"TOKEN=shh\n");
    assert!(!format!("{secret:?}").contains("shh"));
    assert_eq!(s.secret_reads.load(Ordering::SeqCst), 2);
    bus.close().await.unwrap();
}
#[tokio::test]
async fn cache_invalidation_cannot_resurrect_old_generation() {
    let (bus, s) = fixture().await;
    s.hold_config.store(1, Ordering::SeqCst);
    let b = bus.clone();
    let old = tokio::spawn(async move { b.config().await.unwrap().unwrap() });
    s.config_started.notified().await;
    bus.invalidate_private(PrivateSelection::Config);
    assert_eq!(
        tokio::time::timeout(Duration::from_millis(100), bus.config())
            .await
            .expect("invalidated generation still waits for old fetch")
            .unwrap()
            .unwrap()["generation"],
        2
    );
    s.config_release.add_permits(1);
    assert_eq!(old.await.unwrap()["generation"], 1);
    assert_eq!(bus.config().await.unwrap().unwrap()["generation"], 2);
    assert_eq!(s.configs.load(Ordering::SeqCst), 2);
    bus.close().await.unwrap();
}
#[tokio::test]
async fn call_correlates_concurrent_answers_done_and_ack() {
    let (bus, _) = fixture().await;
    let ack = Arc::new(AtomicUsize::new(0));
    let mut tasks = vec![];
    for n in 0..8 {
        let b = bus.clone();
        let ack = ack.clone();
        tasks.push(tokio::spawn(async move {
            let o = CallOptions {
                on_ack: Some(Arc::new(move |_| {
                    ack.fetch_add(1, Ordering::SeqCst);
                })),
                ..Default::default()
            };
            match b.call("#echo", &format!("answer{n}"), o).await.unwrap() {
                CallOutcome::Reply(m) => assert_eq!(m.body(), format!("answer{n}")),
                _ => panic!("expected answer"),
            }
        }));
    }
    for t in tasks {
        t.await.unwrap();
    }
    assert_eq!(ack.load(Ordering::SeqCst), 8);
    assert!(matches!(
        bus.call("#echo", "done", Default::default()).await.unwrap(),
        CallOutcome::Done(_)
    ));
    assert!(matches!(
        bus.call(
            "#ack-only",
            "x",
            CallOptions {
                timeout: Duration::from_millis(200),
                ..Default::default()
            }
        )
        .await,
        Err(BusError::Timeout { acked: true })
    ));
    bus.close().await.unwrap();
}
#[tokio::test]
async fn cancelled_call_removes_pending_reader() {
    let (bus, _) = fixture().await;
    let b = bus.clone();
    let task = tokio::spawn(async move { b.call("#never", "x", Default::default()).await });
    tokio::time::sleep(Duration::from_millis(20)).await;
    assert!(matches!(
        bus.consume(Default::default()).await,
        Err(BusError::ReaderInUse)
    ));
    task.abort();
    let _ = task.await;
    assert!(
        bus.consume(ConsumeOptions {
            wait: Duration::from_millis(10),
            ..Default::default()
        })
        .await
        .unwrap()
        .is_none()
    );
    bus.close().await.unwrap();
}
#[tokio::test]
async fn serve_at_capacity_drains_nested_calls_without_prefetch() {
    let (bus, s) = fixture().await;
    let cancel = CancellationToken::new();
    let started = Arc::new(Notify::new());
    let release = Arc::new(Semaphore::new(0));
    let b = bus.clone();
    let c = cancel.clone();
    let hbus = bus.clone();
    let hs = started.clone();
    let hr = release.clone();
    let server = tokio::spawn(async move {
        b.serve(
            move |message| {
                let b = hbus.clone();
                let started = hs.clone();
                let release = hr.clone();
                async move {
                    assert_eq!(message.roles(), &["owner"]);
                    assert!(matches!(
                        b.call("#echo", "nested", Default::default()).await?,
                        CallOutcome::Reply(_)
                    ));
                    started.notify_one();
                    release.acquire().await.unwrap().forget();
                    Ok(HandlerReply::Body(String::new()))
                }
            },
            ServeOptions::default(),
            c,
        )
        .await
        .unwrap()
    });
    s.push("#worker", request("work", "first"));
    tokio::time::timeout(Duration::from_secs(2), started.notified())
        .await
        .expect("nested call deadlocked at worker capacity");
    s.push("#worker", request("queued", "second"));
    tokio::time::sleep(Duration::from_millis(150)).await;
    assert_eq!(
        s.queues.lock().unwrap()["#worker"].len(),
        1,
        "prefetched work at capacity"
    );
    assert!(matches!(
        bus.consume(Default::default()).await,
        Err(BusError::ReaderInUse)
    ));
    cancel.cancel();
    release.add_permits(1);
    let report = server.await.unwrap();
    assert_eq!(report.completed, 1);
    let sent = s.sent.lock().unwrap();
    let response = sent
        .iter()
        .find(|v| v["to"] == "alice" && v["body"] == "")
        .unwrap();
    assert!(response.get("receipt").is_some());
    assert!(
        sent.iter()
            .any(|v| v["to"] == "alice" && v["body"] == "" && v.get("receipt").is_none()),
        "empty answer converted to done"
    );
    drop(sent);
    bus.close().await.unwrap();
}
#[tokio::test]
async fn typed_storage_and_refusals_are_not_retried() {
    let (bus, s) = fixture().await;
    let jobs = bus.record("jobs").unwrap();
    assert_eq!(jobs.kv().inc("count", 1).await.unwrap(), i64::MAX);
    assert!(
        matches!(jobs.kv().get("bytes",KvKind::String).await.unwrap(),KvValue::Bytes(b) if b==vec![0,255])
    );
    jobs.kv()
        .set("bytes", KvValue::Bytes(vec![0, 255]), SetMode::Add)
        .await
        .unwrap();
    assert!(matches!(
        jobs.locks()
            .try_acquire("build", Duration::from_secs(1))
            .await,
        Err(BusError::Refusal { status: 409, .. })
    ));
    assert!(matches!(
        jobs.manage(json!({"description":"x"})).await,
        Err(BusError::Refusal { status: 503, .. })
    ));
    assert_eq!(s.sent.lock().unwrap().len(), 1, "write retried");
    bus.close().await.unwrap();
}

#[tokio::test]
async fn handler_failure_and_receipts_do_not_send_done_or_loop() {
    let (bus, s) = fixture().await;
    let cancel = CancellationToken::new();
    let called = Arc::new(Notify::new());
    let b = bus.clone();
    let c = cancel.clone();
    let h = called.clone();
    let server = tokio::spawn(async move {
        b.serve(
            move |_| {
                let h = h.clone();
                async move {
                    h.notify_one();
                    Err(BusError::Validation("handler failed".into()))
                }
            },
            ServeOptions::default(),
            c,
        )
        .await
        .unwrap()
    });
    s.push("#worker", json!({"message_id":"receipt","from":"alice","to":"#worker","body":"","receipt":"done","re":"old"}));
    s.push("#worker", request("failure", "failed"));
    tokio::time::timeout(Duration::from_secs(1), called.notified())
        .await
        .unwrap();
    cancel.cancel();
    let report = server.await.unwrap();
    assert_eq!(report.failed, 1);
    let sent = s.sent.lock().unwrap();
    assert_eq!(sent.len(), 1);
    assert_eq!(sent[0]["receipt"], "ack");
    assert_eq!(sent[0]["re"], "failed");
    drop(sent);
    bus.close().await.unwrap();
}

#[tokio::test]
async fn drain_deadline_reports_unfinished_handler_and_close_cancels_calls() {
    let (bus, s) = fixture().await;
    let cancel = CancellationToken::new();
    let started = Arc::new(Notify::new());
    let b = bus.clone();
    let c = cancel.clone();
    let h = started.clone();
    let server = tokio::spawn(async move {
        b.serve(
            move |_| {
                let h = h.clone();
                async move {
                    h.notify_one();
                    std::future::pending::<Result<HandlerReply>>().await
                }
            },
            ServeOptions {
                drain_timeout: Duration::from_millis(20),
                ..Default::default()
            },
            c,
        )
        .await
        .unwrap()
    });
    s.push("#worker", request("forever", "drain"));
    tokio::time::timeout(Duration::from_secs(1), started.notified())
        .await
        .unwrap();
    cancel.cancel();
    assert_eq!(server.await.unwrap().unfinished, 1);
    let b = bus.clone();
    let acknowledged = Arc::new(Notify::new());
    let observer = acknowledged.clone();
    let call = tokio::spawn(async move {
        b.call(
            "#ack-only",
            "x",
            CallOptions {
                on_ack: Some(Arc::new(move |_| observer.notify_one())),
                ..Default::default()
            },
        )
        .await
    });
    tokio::time::timeout(Duration::from_secs(1), acknowledged.notified())
        .await
        .unwrap();
    bus.close().await.unwrap();
    assert!(matches!(
        tokio::time::timeout(Duration::from_millis(100), call)
            .await
            .unwrap()
            .unwrap(),
        Err(BusError::Closed)
    ));
}

#[tokio::test]
async fn cached_private_reads_and_new_serve_refuse_after_close() {
    let (bus, _) = fixture().await;
    let record = bus.own_record();
    let config = record.config().await.unwrap().unwrap();
    assert!(record.secret().await.is_err());
    let secret = record.secret().await.unwrap();
    bus.close().await.unwrap();
    assert!(matches!(record.config().await, Err(BusError::Closed)));
    assert!(matches!(record.secret().await, Err(BusError::Closed)));
    assert!(matches!(
        bus.serve(
            |_| async { Ok(HandlerReply::Done) },
            ServeOptions::default(),
            CancellationToken::new()
        )
        .await,
        Err(BusError::Closed)
    ));
    assert_eq!(config["generation"], 1); // Already returned copies remain caller-owned.
    assert_eq!(secret.bytes(), b"TOKEN=shh\n");
}

#[tokio::test]
async fn receipts_without_matching_original_id_cannot_complete_a_call() {
    let (bus, s) = fixture().await;
    let acked = Arc::new(AtomicUsize::new(0));
    let observer = acked.clone();
    let b = bus.clone();
    let call = tokio::spawn(async move {
        b.call(
            "#never",
            "request",
            CallOptions {
                on_ack: Some(Arc::new(move |_| {
                    observer.fetch_add(1, Ordering::SeqCst);
                })),
                timeout: Duration::from_secs(1),
                ..Default::default()
            },
        )
        .await
    });
    loop {
        if !s.sent.lock().unwrap().is_empty() {
            break;
        }
        tokio::time::sleep(Duration::from_millis(1)).await;
    }
    let sent = s.sent.lock().unwrap()[0].clone();
    let base = json!({"message_id":"receipt","from":"#never","to":"#worker","topic":sent["topic"],"tag":sent["tag"],"body":""});
    let mut wrong = base.clone();
    wrong["receipt"] = json!("ack");
    wrong["re"] = json!("other-original");
    s.push("#worker", wrong);
    let mut missing = base.clone();
    missing["receipt"] = json!("done");
    s.push("#worker", missing);
    let mut answer = base;
    answer["body"] = json!("real answer");
    s.push("#worker", answer);
    assert!(
        matches!(call.await.unwrap().unwrap(), CallOutcome::Reply(m) if m.body()=="real answer")
    );
    assert_eq!(acked.load(Ordering::SeqCst), 0);
    bus.close().await.unwrap();
}

#[tokio::test]
async fn late_call_answers_never_become_new_work() {
    let (bus, s) = fixture().await;
    let cancel = CancellationToken::new();
    let finished = Arc::new(Notify::new());
    let calls = Arc::new(AtomicUsize::new(0));
    let b = bus.clone();
    let handler_bus = bus.clone();
    let c = cancel.clone();
    let f = finished.clone();
    let count = calls.clone();
    let server = tokio::spawn(async move {
        b.serve(
            move |_| {
                let b = handler_bus.clone();
                let f = f.clone();
                let count = count.clone();
                async move {
                    count.fetch_add(1, Ordering::SeqCst);
                    let _ = b
                        .call(
                            "#never",
                            "request",
                            CallOptions {
                                timeout: Duration::from_millis(20),
                                ..Default::default()
                            },
                        )
                        .await;
                    f.notify_one();
                    Ok(HandlerReply::Done)
                }
            },
            ServeOptions::default(),
            c,
        )
        .await
        .unwrap()
    });
    s.push("#worker", request("work", "initial"));
    tokio::time::timeout(Duration::from_secs(1), finished.notified())
        .await
        .unwrap();
    let sent = s
        .sent
        .lock()
        .unwrap()
        .iter()
        .find(|v| v["to"] == "#never")
        .unwrap()
        .clone();
    // Wait for the old filtered poll to finish: its late result can otherwise
    // be discarded by the vanished waiter before serving sees the response.
    let intake_count = s.consume_filters.lock().unwrap().len();
    tokio::time::timeout(Duration::from_secs(1), async {
        loop {
            let unfiltered = s.consume_filters.lock().unwrap()[intake_count..]
                .iter()
                .any(|filter| !filter.contains_key("tag"));
            if unfiltered {
                break;
            }
            tokio::time::sleep(Duration::from_millis(2)).await;
        }
    })
    .await
    .unwrap();
    for re in [None, Some("old-original")] {
        let mut answer = json!({"message_id":"late","from":"#never","to":"#worker","topic":sent["topic"],"tag":sent["tag"],"body":"late answer"});
        if let Some(re) = re {
            answer["re"] = json!(re);
        }
        s.push("#worker", answer);
    }
    tokio::time::sleep(Duration::from_millis(180)).await;
    cancel.cancel();
    server.await.unwrap();
    assert_eq!(
        calls.load(Ordering::SeqCst),
        1,
        "late answer was run as new work"
    );
    bus.close().await.unwrap();
}

#[tokio::test]
async fn call_deadline_bounds_the_send_phase_without_replaying() {
    let (bus, s) = fixture().await;
    let start = std::time::Instant::now();
    let outcome = bus
        .call(
            "#never",
            "delayed-post",
            CallOptions {
                timeout: Duration::from_millis(40),
                ..Default::default()
            },
        )
        .await;
    assert!(matches!(outcome, Err(BusError::UncertainOutcome { .. })));
    assert!(
        start.elapsed() < Duration::from_millis(120),
        "call deadline started after the delayed POST"
    );
    assert_eq!(
        s.sent.lock().unwrap().len(),
        1,
        "timed-out write was replayed"
    );
    bus.close().await.unwrap();
}

#[tokio::test]
async fn serve_drains_known_take_when_intake_stops() {
    let (bus, s) = fixture().await;
    let stop = CancellationToken::new();
    *s.consume_stop.lock().unwrap() = Some(stop.clone());
    s.push("#worker", request("work", "taken"));
    let calls = Arc::new(AtomicUsize::new(0));
    let count = calls.clone();
    let report = bus
        .serve(
            move |_| {
                let count = count.clone();
                async move {
                    count.fetch_add(1, Ordering::SeqCst);
                    Ok(HandlerReply::Done)
                }
            },
            ServeOptions {
                drain_timeout: Duration::from_secs(1),
                ..Default::default()
            },
            stop,
        )
        .await
        .unwrap();
    assert_eq!(
        calls.load(Ordering::SeqCst),
        1,
        "successful taken message was discarded at intake stop"
    );
    assert_eq!(report.completed, 1);
    let sent = s.sent.lock().unwrap();
    assert_eq!(sent.iter().filter(|v| v["receipt"] == "ack").count(), 1);
    assert_eq!(sent.iter().filter(|v| v["receipt"] == "done").count(), 1);
    drop(sent);
    bus.close().await.unwrap();
}
#[tokio::test]
async fn serve_skips_expired_deadlines_before_acknowledging() {
    let (bus, s) = fixture().await;
    let mut expired = request("stale", "expired");
    expired["deadline"] = json!("2000-01-01T00:00:00Z");
    s.push("#worker", expired);
    s.push("#worker", request("normal", "fresh"));
    let stop = CancellationToken::new();
    let cancel = stop.clone();
    let count = Arc::new(AtomicUsize::new(0));
    let calls = count.clone();
    bus.serve(
        move |m| {
            let count = count.clone();
            let stop = cancel.clone();
            async move {
                count.fetch_add(1, Ordering::SeqCst);
                stop.cancel();
                assert_eq!(m.body(), "normal", "expired message entered handler");
                Ok(HandlerReply::Done)
            }
        },
        ServeOptions {
            drain_timeout: Duration::from_secs(1),
            ..Default::default()
        },
        stop,
    )
    .await
    .unwrap();
    assert_eq!(calls.load(Ordering::SeqCst), 1);
    assert!(
        !s.sent.lock().unwrap().iter().any(|v| v["re"] == "expired"),
        "expired message was acknowledged"
    );
    bus.close().await.unwrap();
}

#[tokio::test]
async fn node_identity_is_separate_from_authenticated_caller() {
    let (bus, _) = fixture().await;
    let identity = json!(bus.identity().await.unwrap());
    assert_eq!(
        identity["node"], "fixture",
        "identity returned caller status instead of node facts"
    );
    assert_eq!(bus.name(), "#worker");
    bus.close().await.unwrap();
}
