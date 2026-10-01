//! Conformance against the project's real Go core and HTTP face.
use agent_bus::*;
use serde_json::{Value, json};
use std::{
    io::{BufRead, BufReader},
    process::{Child, Command, Stdio},
    time::Duration,
};
struct Fixture(Child);
impl Drop for Fixture {
    fn drop(&mut self) {
        drop(self.0.stdin.take());
        let _ = self.0.wait();
    }
}
fn fixture() -> (Fixture, Value) {
    let source = std::path::PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../..");
    let mut command = if let Ok(binary) = std::env::var("AGENT_BUS_FIXTURE_BINARY") {
        Command::new(binary)
    } else {
        let mut c = Command::new("go");
        c.args(["run", "./clients/fixture"]);
        c.current_dir(source);
        c
    };
    let mut child = command
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::inherit())
        .spawn()
        .expect("Go fixture requires the repository's Go toolchain");
    let mut line = String::new();
    BufReader::new(child.stdout.take().unwrap())
        .read_line(&mut line)
        .unwrap();
    let v = serde_json::from_str(&line).expect("fixture did not start");
    (Fixture(child), v)
}
async fn connect(v: &Value, name: &str) -> Bus {
    Bus::builder(name)
        .transport(v["addr"].as_str().unwrap())
        .credential(v["tokens"][name].as_str().unwrap())
        .timeout(Duration::from_secs(2))
        .connect()
        .await
        .unwrap()
}
#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn real_daemon_registry_private_storage_and_nested_serving() {
    let (_fixture, v) = fixture();
    let owner = connect(&v, "owner@fixture").await;
    let worker = connect(&v, "#worker@fixture").await;
    let helper = connect(&v, "#helper@fixture").await;
    let caller = connect(&v, "#caller@fixture").await;
    let outsider = connect(&v, "#outsider@fixture").await;
    assert_eq!(worker.config().await.unwrap().unwrap()["label"], "fixture");
    assert_eq!(
        worker.secret().await.unwrap().bytes(),
        b"TOKEN=dummy-fixture-only\n"
    );
    assert!(matches!(
        outsider.record("#worker@fixture").unwrap().secret().await,
        Err(BusError::Refusal { status: 403, .. })
    ));
    let jobs = owner.record("jobs@fixture").unwrap();
    assert_eq!(jobs.lookup().await.unwrap()["kind"], "queue");
    jobs.kv()
        .set("n", KvValue::Int(i64::MAX - 1), SetMode::Set)
        .await
        .unwrap();
    assert_eq!(jobs.kv().inc("n", 1).await.unwrap(), i64::MAX);
    jobs.kv()
        .set("raw", KvValue::Bytes(vec![0, 255, 2]), SetMode::Set)
        .await
        .unwrap();
    assert!(
        matches!(jobs.kv().get("raw",KvKind::String).await.unwrap(),KvValue::Bytes(b) if b==vec![0,255,2])
    );
    jobs.kv()
        .set("document", KvValue::Json(json!({"n":1})), SetMode::Set)
        .await
        .unwrap();
    let result = jobs
        .kv()
        .json(
            "document",
            &[JsonOp {
                op: "inc".into(),
                key: "n".into(),
                value: Some(json!(2)),
            }],
        )
        .await
        .unwrap();
    assert_eq!(result["results"][0]["value"], 3);
    assert!(jobs.kv().delete("raw", KvKind::String).await.unwrap());
    assert!(jobs.kv().list().await.unwrap()["values"].is_object());
    let mut lease = jobs
        .locks()
        .try_acquire("build", Duration::from_secs(3))
        .await
        .unwrap();
    assert!(matches!(
        jobs.locks()
            .try_acquire("build", Duration::from_secs(3))
            .await,
        Err(BusError::Refusal { status: 409, .. })
    ));
    lease.extend(Duration::from_secs(3)).await.unwrap();
    assert_eq!(
        jobs.locks().holders().await.unwrap()["locks"]["build"]["holder"],
        "owner@fixture"
    );
    lease.release().await.unwrap();
    jobs.manage(json!({"maintainers":["#worker@fixture"]}))
        .await
        .unwrap();
    let _held = jobs
        .locks()
        .try_acquire("force", Duration::from_secs(3))
        .await
        .unwrap();
    assert_eq!(
        worker
            .record("jobs@fixture")
            .unwrap()
            .locks()
            .force_release("force")
            .await
            .unwrap()
            .as_deref(),
        Some("owner@fixture")
    );
    caller
        .publish("jobs@fixture", "published", SendOptions::default())
        .await
        .unwrap();
    let published = caller
        .consume(ConsumeOptions {
            inbox: Some("jobs@fixture".into()),
            wait: Duration::from_secs(1),
            ..Default::default()
        })
        .await
        .unwrap()
        .unwrap();
    assert_eq!(published.body(), "published");
    let record = owner.record("temporary@fixture").unwrap();
    record
        .register(json!({"kind":"queue","allow":["*"]}))
        .await
        .unwrap();
    record
        .manage(json!({"descr":"Rust fixture"}))
        .await
        .unwrap();
    assert_eq!(record.lookup().await.unwrap()["descr"], "Rust fixture");
    record.unregister().await.unwrap();
    assert!(matches!(
        record.lookup().await,
        Err(BusError::Refusal { status: 404, .. })
    ));
    let cancel = CancellationToken::new();
    let helper_task = {
        let h = helper.clone();
        let c = cancel.clone();
        tokio::spawn(async move {
            h.serve(
                |m| async move {
                    Ok(if m.body() == "done" {
                        HandlerReply::Done
                    } else {
                        HandlerReply::Body(m.body().into())
                    })
                },
                ServeOptions::default(),
                c,
            )
            .await
            .unwrap()
        })
    };
    let worker_task = {
        let w = worker.clone();
        let h = w.clone();
        let c = cancel.clone();
        tokio::spawn(async move {
            w.serve(
                move |m| {
                    let h = h.clone();
                    async move {
                        match h
                            .call(
                                "#helper@fixture",
                                m.body(),
                                CallOptions {
                                    timeout: Duration::from_secs(3),
                                    ..Default::default()
                                },
                            )
                            .await?
                        {
                            CallOutcome::Reply(answer) => {
                                Ok(HandlerReply::Body(answer.body().into()))
                            }
                            CallOutcome::Done(_) => Ok(HandlerReply::Done),
                        }
                    }
                },
                ServeOptions::default(),
                c,
            )
            .await
            .unwrap()
        })
    };
    for text in ["round-trip", "", "done"] {
        let outcome = caller
            .call(
                "#worker@fixture",
                text,
                CallOptions {
                    timeout: Duration::from_secs(3),
                    ..Default::default()
                },
            )
            .await
            .unwrap();
        match outcome {
            CallOutcome::Reply(m) => {
                assert_ne!(text, "done");
                assert_eq!(m.body(), text);
            }
            CallOutcome::Done(m) => {
                assert_eq!(text, "done");
                assert!(!m.re().is_empty());
            }
        }
    }
    cancel.cancel();
    assert_eq!(helper_task.await.unwrap().failed, 0);
    assert_eq!(worker_task.await.unwrap().failed, 0);
    for b in [owner, worker, helper, caller, outsider] {
        b.close().await.unwrap();
    }
}
