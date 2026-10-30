//! Native Rust adapter for the shared protocol cases in src/testdata/client-conformance.
use agent_bus::*;
use serde_json::Value;
use std::{
    io::{BufRead, BufReader},
    process::{Child, Command, Stdio},
    sync::{Arc, Mutex},
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
    (Fixture(child), serde_json::from_str(&line).expect("fixture did not start"))
}
fn cases(name: &str) -> Vec<Value> {
    let path = std::path::PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../../testdata/client-conformance")
        .join(format!("{name}.json"));
    let cases: Vec<Value> = serde_json::from_slice(&std::fs::read(&path).unwrap()).unwrap();
    assert!(!cases.is_empty(), "{} has no cases", path.display());
    cases
}
fn text<'a>(v: &'a Value, key: &str) -> &'a str {
    v[key].as_str().unwrap_or("")
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

fn same_envelope(m: &Message, expected: &Value, id: &str) {
    for (key, value) in expected.as_object().unwrap() {
        let got = match key.as_str() {
            "from" => m.from(),
            "to" => m.to(),
            "body" => m.body(),
            "topic" => m.topic(),
            "tag" => m.tag(),
            "ask" => m.ask(),
            other => panic!("{id}: no adapter for expected field {other}"),
        };
        assert_eq!(got, value.as_str().unwrap(), "{id}: {key}");
    }
}

// Each send is received once, field for field, under the ID the send returned.
#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn shared_send_cases_arrive_as_sent() {
    let (_fixture, v) = fixture();
    let caller = connect(&v, "#caller@fixture").await;
    let worker = connect(&v, "#worker@fixture").await;
    for case in cases("send") {
        let request = &case["request"];
        let id = text(&case, "id");
        let sent = caller
            .send(
                text(request, "to"),
                text(request, "body"),
                SendOptions {
                    topic: request["topic"].as_str().map(Into::into),
                    tag: request["tag"].as_str().map(Into::into),
                    ask: request["ask"].as_str().map(Into::into),
                    ..Default::default()
                },
            )
            .await
            .unwrap_or_else(|e| panic!("{id}: {e:?}"));
        let got = worker
            .consume(ConsumeOptions {
                wait: Duration::from_secs(2),
                ..Default::default()
            })
            .await
            .unwrap()
            .unwrap_or_else(|| panic!("{id}: nothing arrived"));
        same_envelope(&got, &case["expected"], id);
        assert_eq!(got.id(), sent.id(), "{id}: the accepted ID");
    }
}

// The worker's handler runs once per call, and the caller gets its answer,
// empty ones included, naming the request.
#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn shared_call_cases_return_the_handler_answer() {
    let (_fixture, v) = fixture();
    let caller = connect(&v, "#caller@fixture").await;
    let worker = connect(&v, "#worker@fixture").await;
    let rows = cases("call");
    let handled = Arc::new(Mutex::new(Vec::<(String, String)>::new()));
    let cancel = CancellationToken::new();
    let server = {
        let (b, c, handled, rows) = (worker.clone(), cancel.clone(), handled.clone(), rows.clone());
        tokio::spawn(async move {
            b.serve(
                move |m| {
                    let (handled, rows) = (handled.clone(), rows.clone());
                    async move {
                        let row = rows
                            .iter()
                            .find(|r| text(&r["request"], "body") == m.body())
                            .unwrap_or_else(|| panic!("no case for {:?}", m.body()));
                        handled
                            .lock()
                            .unwrap()
                            .push((m.body().to_string(), m.id().to_string()));
                        Ok(HandlerReply::Body(text(row, "answer").to_string()))
                    }
                },
                ServeOptions {
                    workers: 2,
                    ..Default::default()
                },
                c,
            )
            .await
            .unwrap()
        })
    };
    for case in &rows {
        let request = &case["request"];
        let id = text(case, "id");
        let outcome = caller
            .call(
                text(request, "to"),
                text(request, "body"),
                CallOptions {
                    timeout: Duration::from_secs(5),
                    topic: request["topic"].as_str().map(Into::into),
                    ..Default::default()
                },
            )
            .await
            .unwrap_or_else(|e| panic!("{id}: {e:?}"));
        let CallOutcome::Reply(m) = outcome else {
            panic!("{id}: done instead of an answer");
        };
        same_envelope(&m, &case["expected"], id);
        let seen = handled.lock().unwrap().clone();
        let mine: Vec<_> = seen.iter().filter(|(b, _)| b == text(request, "body")).collect();
        assert_eq!(mine.len(), 1, "{id}: the handler runs once");
        assert_eq!(m.re(), mine[0].1, "{id}: the answer names its request");
    }
    cancel.cancel();
    assert_eq!(server.await.unwrap().failed, 0);
    assert_eq!(handled.lock().unwrap().len(), rows.len());
}

// A self-call is answered by the caller's own handler; the request it sent,
// carrying the same inbox, topic and tag, is work and never the answer.
#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn a_self_call_returns_its_handler_answer() {
    let (_fixture, v) = fixture();
    for case in cases("self-call") {
        self_call(&v, &case, None).await;
    }
}

// With another reply inbox only the serving read can take the request, whose
// tag still carries this caller's call prefix.
#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn a_self_call_with_an_alternate_reply_inbox_runs_its_handler() {
    let (_fixture, v) = fixture();
    for case in cases("self-call") {
        self_call(&v, &case, Some("jobs@fixture")).await;
    }
}

async fn self_call(v: &Value, case: &Value, reply_inbox: Option<&str>) {
    let request = &case["request"];
    let expected = &case["expected"];
    let name = text(request, "to");
    let bus = Bus::builder(name)
        .transport(v["addr"].as_str().unwrap())
        .credential(v["tokens"][name].as_str().unwrap())
        .timeout(Duration::from_secs(2))
        .connect()
        .await
        .unwrap();
    let handled = Arc::new(Mutex::new(Vec::new()));
    let cancel = CancellationToken::new();
    let server = {
        let (b, c, handled) = (bus.clone(), cancel.clone(), handled.clone());
        let (want, answer) = (
            text(request, "body").to_string(),
            text(case, "answer").to_string(),
        );
        tokio::spawn(async move {
            b.serve(
                move |m| {
                    let (want, answer, handled) = (want.clone(), answer.clone(), handled.clone());
                    async move {
                        assert_eq!(m.body(), want);
                        handled.lock().unwrap().push(m.id().to_string());
                        Ok(HandlerReply::Body(answer))
                    }
                },
                ServeOptions {
                    workers: 2,
                    ..Default::default()
                },
                c,
            )
            .await
            .unwrap()
        })
    };
    let outcome = bus
        .call(
            name,
            text(request, "body"),
            CallOptions {
                timeout: Duration::from_secs(5),
                topic: Some(text(request, "topic").into()),
                reply_inbox: reply_inbox.map(Into::into),
                ..Default::default()
            },
        )
        .await
        .unwrap_or_else(|e| panic!("{}: {e:?}", case["id"]));
    let CallOutcome::Reply(m) = outcome else {
        panic!("{}: done instead of an answer", case["id"]);
    };
    assert_eq!(m.body(), text(expected, "body"), "{}", case["id"]);
    assert_eq!(m.from(), text(expected, "from"), "{}", case["id"]);
    assert_eq!(m.to(), reply_inbox.unwrap_or(text(expected, "to")), "{}", case["id"]);
    assert_eq!(m.topic(), text(expected, "topic"), "{}", case["id"]);
    cancel.cancel();
    let report = server.await.unwrap();
    assert_eq!(report.failed, 0, "{}", case["id"]);
    let handled = handled.lock().unwrap().clone();
    assert_eq!(handled.len(), 1, "{}: handler runs", case["id"]);
    assert_eq!(m.re(), handled[0], "{}: the answer names its request", case["id"]);
    bus.close().await.unwrap();
}
