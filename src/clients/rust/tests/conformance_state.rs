//! Native Rust adapter for the shared state, identity and refusal vectors in
//! src/testdata/client-conformance (kv, locks, identity, refusals, events; not bigfile).
//! Event cases: shared_event_vectors / shared_event_subscription_vectors.
//! Executed examples: native_event_example / native_event_subscription_example.
use agent_bus::*;
use serde_json::{json, Value};
use std::{
    io::{BufRead, BufReader},
    process::{Child, Command, Stdio},
    sync::{Arc, Mutex},
    time::Duration,
};
use tokio::io::{AsyncReadExt, AsyncWriteExt};

struct Fixture(Child);
impl Drop for Fixture {
    fn drop(&mut self) {
        drop(self.0.stdin.take());
        let _ = self.0.wait();
    }
}
// Every case gets a daemon of its own: the fixture is cheap and no case depends on another.
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

/// What the step's value is: repeated text, decoded bytes, or the text itself.
fn step_bytes(step: &Value) -> Vec<u8> {
    use base64::{engine::general_purpose::STANDARD, Engine};
    if let Some(r) = step.get("repeat") {
        return text(r, "text").repeat(r["count"].as_u64().unwrap() as usize).into_bytes();
    }
    if let Some(b) = step.get("value_b64") {
        return STANDARD.decode(b.as_str().unwrap()).unwrap();
    }
    text(step, "value").as_bytes().to_vec()
}

/// Checks a step's expected refusal. Some(result) when the step was meant to succeed.
fn refusal<T: std::fmt::Debug>(result: Result<T>, step: &Value, at: &str) -> Option<T> {
    let want = &step["expect"]["refused"];
    if want.is_null() {
        return Some(result.unwrap_or_else(|e| panic!("{at}: {e:?}")));
    }
    let error = result.expect_err(&format!("{at}: expected a refusal"));
    if want.is_object() {
        match error {
            BusError::Refusal { status, message, .. } => {
                assert_eq!(status as u64, want["status"].as_u64().unwrap(), "{at}: {message}");
                if let Some(has) = want["message_has"].as_str() {
                    assert!(message.contains(has), "{at}: {message:?} lacks {has:?}");
                }
            }
            other => panic!("{at}: not a refusal from the daemon: {other:?}"),
        }
    }
    None
}

#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn shared_kv_vectors() {
    for case in cases("kv") {
        let (_fixture, v) = fixture();
        let owner = connect(&v, "owner@fixture").await;
        let kv = owner.record("jobs@fixture").unwrap().kv();
        for (i, step) in case["steps"].as_array().unwrap().iter().enumerate() {
            let at = format!("{} step {i} {} {}", text(&case, "id"), text(step, "op"), text(step, "name"));
            let name = text(step, "name");
            let expect = &step["expect"];
            let kind = match step["kind"].as_str().unwrap_or("string") {
                "int" => KvKind::Int,
                "json" => KvKind::Json,
                _ => KvKind::String,
            };
            match text(step, "op") {
                "set" => {
                    let value = match kind {
                        KvKind::Int => KvValue::Int(text(step, "value").parse().unwrap()),
                        KvKind::Json => KvValue::Json(serde_json::from_str(text(step, "value")).unwrap()),
                        KvKind::String => KvValue::Bytes(step_bytes(step)),
                    };
                    let mode = match step["how"].as_str().unwrap_or("set") {
                        "add" => SetMode::Add,
                        "replace" => SetMode::Replace,
                        _ => SetMode::Set,
                    };
                    refusal(kv.set(name, value, mode).await, step, &at);
                }
                "get" => {
                    let Some(got) = refusal(kv.get(name, kind).await, step, &at) else { continue };
                    if let Some(b64) = expect["value_b64"].as_str() {
                        use base64::{engine::general_purpose::STANDARD, Engine};
                        let KvValue::Bytes(raw) = got else { panic!("{at}: not bytes") };
                        assert_eq!(STANDARD.encode(raw), b64, "{at}");
                    } else if let Some(n) = expect["length"].as_u64() {
                        let KvValue::Bytes(raw) = got else { panic!("{at}: not bytes") };
                        assert_eq!(raw.len() as u64, n, "{at}");
                    } else if expect.get("json").is_some() {
                        let KvValue::Json(j) = got else { panic!("{at}: not json") };
                        assert_eq!(j, expect["json"], "{at}");
                    } else if let KvKind::Int = kind {
                        let KvValue::Int(n) = got else { panic!("{at}: not int") };
                        assert_eq!(n.to_string(), text(expect, "value"), "{at}");
                    } else {
                        assert_eq!(got.utf8().unwrap(), text(expect, "value"), "{at}");
                    }
                }
                "fill" => {
                    for n in 0..step["count"].as_u64().unwrap() {
                        let key = format!("{}{n}", text(step, "prefix"));
                        kv.set(&key, KvValue::Bytes(b"v".to_vec()), SetMode::Add)
                            .await
                            .unwrap_or_else(|e| panic!("{at} {key}: {e:?}"));
                    }
                }
                "delete" => {
                    if let Some(got) = refusal(kv.delete(name, kind).await, step, &at) {
                        assert_eq!(got, expect["deleted"].as_bool().unwrap(), "{at}");
                    }
                }
                "inc" => {
                    let by: i64 = text(step, "by").parse().unwrap();
                    if let Some(got) = refusal(kv.inc(name, by).await, step, &at) {
                        if let Some(want) = expect["value"].as_str() {
                            assert_eq!(got.to_string(), want, "{at}");
                        }
                    }
                }
                "json" => {
                    let ops: Vec<JsonOp> = step["ops"]
                        .as_array()
                        .unwrap()
                        .iter()
                        .map(|o| JsonOp {
                            op: text(o, "op").into(),
                            key: text(o, "key").into(),
                            value: o.get("value").cloned(),
                        })
                        .collect();
                    if let Some(got) = refusal(kv.json(name, &ops).await, step, &at) {
                        if let Some(want) = expect.get("changed") {
                            let results = got.as_array().cloned().or_else(|| got["results"].as_array().cloned()).expect("json results");
                            let changed: Vec<Value> = results.iter().map(|r| r["changed"].clone()).collect();
                            assert_eq!(&Value::Array(changed), want, "{at}");
                        }
                    }
                }
                other => panic!("unknown vector op {other}"),
            }
        }
    }
}

#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn shared_lock_vectors() {
    for case in cases("locks") {
        let (_fixture, v) = fixture();
        let owner = connect(&v, "owner@fixture").await;
        let worker = connect(&v, "#worker@fixture").await;
        for setup in case["setup"].as_array().cloned().unwrap_or_default() {
            assert_eq!(setup, "worker-maintains-jobs");
            owner
                .record("jobs@fixture")
                .unwrap()
                .manage(json!({"maintainers": ["#worker@fixture"]}))
                .await
                .unwrap();
        }
        for (i, step) in case["steps"].as_array().unwrap().iter().enumerate() {
            let who = text(step, "as");
            let at = format!("{} step {i} {who} {} {}", text(&case, "id"), text(step, "op"), text(step, "name"));
            let bus = if who == "owner" { &owner } else { &worker };
            let locks = bus.record("jobs@fixture").unwrap().locks();
            let name = text(step, "name");
            let ttl = Duration::from_secs(step["ttl_s"].as_u64().unwrap_or(0));
            let expect = &step["expect"];
            match text(step, "op") {
                "try" => {
                    refusal(locks.try_acquire(name, ttl).await, step, &at);
                }
                "acquire" => {
                    let options = LockOptions { ttl, wait: Duration::from_secs(step["wait_s"].as_u64().unwrap_or(0)) };
                    refusal(locks.acquire(name, options).await, step, &at);
                }
                "extend" => {
                    refusal(locks.extend(name, ttl).await, step, &at);
                }
                "release" => {
                    refusal(locks.release(name).await, step, &at);
                }
                "force" => {
                    if let Some(got) = refusal(locks.force_release(name).await, step, &at) {
                        assert_eq!(got.as_deref(), expect["displaced"].as_str(), "{at}");
                    }
                }
                "holders" => {
                    if let Some(got) = refusal(locks.holders().await, step, &at) {
                        // The SDK returns the daemon's answer: {"record":…,"locks":{name:{holder,…}}}.
                        let held: serde_json::Map<String, Value> = got["locks"]
                            .as_object()
                            .cloned()
                            .unwrap_or_default()
                            .into_iter()
                            .map(|(k, h)| (k, h["holder"].clone()))
                            .collect();
                        assert_eq!(Value::Object(held), expect["holders"], "{at}");
                    }
                }
                other => panic!("unknown vector op {other}"),
            }
        }
    }
}

/// A daemon whose /identity answers the vector's raw body, whose GET /ls answers
/// `refuse` (status, body, Allow) when given, and anything else `{"you":...}`.
/// It records the path of every request.
async fn fake_daemon(identity: &str, refuse: Option<(u16, String, Option<String>)>) -> (String, Arc<Mutex<Vec<String>>>) {
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = format!("http://{}", listener.local_addr().unwrap());
    let calls = Arc::new(Mutex::new(Vec::new()));
    let (identity, seen) = (identity.to_owned(), calls.clone());
    tokio::spawn(async move {
        loop {
            let (mut socket, _) = listener.accept().await.unwrap();
            let (identity, refuse, seen) = (identity.clone(), refuse.clone(), seen.clone());
            tokio::spawn(async move {
                let mut buffer = [0; 8192];
                let n = socket.read(&mut buffer).await.unwrap_or(0);
                let request = String::from_utf8_lossy(&buffer[..n]).into_owned();
                let path = request.split_whitespace().nth(1).unwrap_or("").to_owned();
                seen.lock().unwrap().push(path.clone());
                let bare = path.split('?').next().unwrap_or("");
                let (mut code, mut allow, mut body) = (200, None, "{\"you\":\"owner@fixture\"}".to_owned());
                if bare == "/identity" {
                    body = identity;
                } else if let (Some((status, text, header)), "/ls") = (refuse, bare) {
                    (code, allow, body) = (status, header, text);
                }
                let extra = allow.map(|a| format!("Allow: {a}\r\n")).unwrap_or_default();
                let head = format!(
                    "HTTP/1.1 {code} X\r\nContent-Type: application/json\r\n{extra}Content-Length: {}\r\nConnection: close\r\n\r\n",
                    body.len()
                );
                let _ = socket.write_all(head.as_bytes()).await;
                let _ = socket.write_all(body.as_bytes()).await;
            });
        }
    });
    (address, calls)
}
async fn open(address: &str) -> Result<Bus> {
    Bus::builder("owner@fixture")
        .transport(address)
        .credential("fixture")
        .timeout(Duration::from_secs(2))
        .connect()
        .await
}

#[tokio::test]
async fn shared_identity_vectors() {
    for case in cases("identity") {
        let id = text(&case, "id");
        let (address, calls) = fake_daemon(text(&case, "identity"), None).await;
        let expect = &case["expect"];
        // The version rule runs at connect or at the first call: either is the client's.
        let outcome = async {
            let bus = open(&address).await?;
            bus.status().await?;
            Ok::<Bus, BusError>(bus)
        }
        .await;
        if let Some(fatal) = expect["fatal"].as_str() {
            let error = outcome.err().unwrap_or_else(|| panic!("{id}: expected a fatal handshake"));
            assert!(error.to_string().contains(fatal), "{id}: {error}");
            assert_eq!(*calls.lock().unwrap(), vec!["/identity".to_owned()], "{id}: an application request followed a fatal handshake");
        } else {
            let _bus = outcome.unwrap_or_else(|e| panic!("{id}: {e:?}"));
            assert_eq!(calls.lock().unwrap()[0], "/identity", "{id}");

        }
    }
}

#[tokio::test]
async fn shared_refusal_vectors() {
    for case in cases("refusals") {
        let id = text(&case, "id");
        let current = r#"{"protocol":1,"protocol_min":1}"#;
        if let Some(call) = case.get("call") {
            // The client refuses these itself: no daemon refusal, and the daemon never saw the path.
            let (address, calls) = fake_daemon(current, None).await;
            let bus = open(&address).await.unwrap();
            let locks = bus.record("jobs@fixture").unwrap().locks();
            let name = text(call, "name");
            // A negative ttl cannot be written as a Duration: the type refuses it before any call,
            // and zero is the nearest value the SDK can be asked for.
            let ttl = Duration::from_secs(call["ttl_s"].as_i64().unwrap().max(0) as u64);
            let error = match text(call, "op") {
                "acquire" => locks
                    .acquire(name, LockOptions { ttl, wait: Duration::from_secs(call["wait_s"].as_u64().unwrap()) })
                    .await
                    .err(),
                "try" => locks.try_acquire(name, ttl).await.err(),
                "extend" => locks.extend(name, ttl).await.err(),
                other => panic!("{id}: unknown call op {other}"),
            };
            let error = error.unwrap_or_else(|| panic!("{id}: the client accepted it"));
            assert!(!matches!(error, BusError::Refusal { .. }), "{id}: {error:?}");
            let not = text(&case["expect"], "not_requested");
            assert!(!calls.lock().unwrap().iter().any(|p| p.split('?').next() == Some(not)), "{id}: {not} reached the daemon");
            continue;
        }
        let identity = if text(&case, "identity") == "current" { current } else { "{}" };
        let daemon = &case["daemon"];
        let (address, _) = fake_daemon(
            identity,
            Some((
                daemon["status"].as_u64().unwrap() as u16,
                text(daemon, "body").to_owned(),
                daemon["allow"].as_str().map(Into::into),
            )),
        )
        .await;
        let bus = open(&address).await.unwrap();
        let error = bus.records(&[]).await.expect_err(id);
        let want = &case["expect"];
        match error {
            BusError::Refusal { status, kind, message } => {
                assert_eq!(status as u64, want["status"].as_u64().unwrap(), "{id}");
                assert_eq!(kind.unwrap_or_default(), text(want, "kind"), "{id}");
                assert_eq!(message, text(want, "message"), "{id}");
            }
            other => panic!("{id}: not a daemon refusal: {other:?}"),
        }
    }
}


// Existing send/call/consume only: no new event SDK or subscription surface.
async fn event_call(bus: &Bus, body: &str) -> String {
    match bus.call("#bus", body, CallOptions {timeout: Duration::from_secs(5), ..Default::default()}).await.expect("event call failed") {
        CallOutcome::Reply(message) => message.body().to_owned(),
        CallOutcome::Done(_) => panic!("event call finished without an answer"),
    }
}
fn event_path<'a>(mut value: &'a Value, path: &str) -> &'a Value {
    for key in path.split('.') {
        value = if value.is_array() { &value[key.parse::<usize>().expect("array index")] } else { &value[key] };
    }
    value
}
#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn shared_event_vectors() { run_event_vectors("events").await; }
#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn shared_event_subscription_vectors() { run_event_vectors("event-subscriptions").await; }

async fn run_event_vectors(file: &str) {
    for case in cases(file) {
        assert_eq!(case["fresh"], true);
        let (_fixture, v) = fixture();
        let owner = connect(&v, "owner@fixture").await;
        let agent = connect(&v, "#caller@fixture").await;
        let third = connect(&v, "#outsider@fixture").await;
        let worker = connect(&v, "#worker@fixture").await;
        let mut subscription = String::new();
        let mut answers = std::collections::HashMap::new();
        for (i, step) in case["steps"].as_array().unwrap().iter().enumerate() {
            let at = format!("{} step {i} {}", text(&case,"id"), text(step,"op"));
            let bus = match text(step,"as") {"owner"=>&owner,"agent"=>&agent,"third"=>&third,"worker"=>&worker,other=>panic!("unknown identity {other}")};
            match text(step,"op") {
                "call" => {
                    let raw = event_call(bus, &text(step,"body").replace("$subscription", &subscription)).await;
                    let decoded: Value = serde_json::from_str(&raw).expect("event JSON reply");
                    if text(step,"body").starts_with("sub add ") && decoded.get("error").is_none() {
                        subscription = decoded["id"].as_str().expect("subscription id").to_owned();
                        assert!(!subscription.is_empty(), "{at}: subscription id");
                    }
                    if let Some(expected) = step["expect"].as_object() {
                        for (path,want) in expected {
                            let want = if let Some(value) = want.as_str() { json!(value.replace("$subscription", &subscription)) } else { want.clone() };
                            assert_eq!(event_path(&decoded,path), &want, "{at}: {path}");
                        }
                    }
                    if let Some(prior) = step["same_answer_as"].as_u64() {
                        assert_eq!(Some(&raw), answers.get(&(prior as usize)), "{at}: private refusal bytes");
                    }
                    answers.insert(i,raw);
                }
                "send" => {bus.send("#bus",text(step,"body"),SendOptions::default()).await.unwrap();}
                "manage" => { bus.record(text(step,"name")).unwrap().manage(step["changes"].clone()).await.unwrap(); }
                "inbox" => {
                    let inbox = step["inbox"].as_str().map(str::to_owned);
                    let message = bus.consume(ConsumeOptions {inbox,wait:Duration::from_millis(300),..Default::default()}).await.unwrap();
                    if step["expect"]["empty"] == true {
                        assert!(message.is_none(),"{at}: inbox not empty");
                        continue;
                    }
                    let message = message.expect(&format!("{at}: missing subscription copy"));
                    let value = event_copy(&message);
                    for (path,want) in step["expect"].as_object().unwrap() {
                        assert_eq!(event_path(&value,path),want,"{at}: copy {path}");
                    }
                    // Non-waiting notices get no ack/done.
                }
                other=>panic!("{at}: unknown operation {other}"),
            }
        }
        for bus in [&owner,&agent,&third,&worker] {bus.close().await.unwrap();}
    }
}

// Executed native README example; distinct credentials keep the authority honest.
async fn example_event_log(agent: &Bus, owner: &Bus, third: &Bus) -> String {
    let write: Value = serde_json::from_str(&event_call(agent,"write #caller@fixture alert disk full").await).unwrap();
    let read: Value = serde_json::from_str(&event_call(owner,"read #caller@fixture").await).unwrap();
    let list: Value = serde_json::from_str(&event_call(owner,"list").await).unwrap();
    let private: Value = serde_json::from_str(&event_call(third,"read #caller@fixture").await).unwrap();
    let own: Value = serde_json::from_str(&event_call(agent,"read #caller@fixture").await).unwrap();
    format!("written: {} {}\nowner reads: {} alert\nowner lists: {}\na third party: {}\nthe writer reading its own log: {}",
        text(&write["entry"],"level"),text(&write["entry"],"text"),read["rings"]["alert"].as_array().unwrap().len(),text(&list["logs"][0],"log"),text(&private["error"],"code"),text(&own["error"],"code"))
}
#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn native_event_example() {
    let (_fixture,v)=fixture();
    let owner=connect(&v,"owner@fixture").await;
    let agent=connect(&v,"#caller@fixture").await;
    let third=connect(&v,"#outsider@fixture").await;
    assert_eq!(example_event_log(&agent,&owner,&third).await,"written: alert disk full\nowner reads: 1 alert\nowner lists: #caller@fixture\na third party: forbidden\nthe writer reading its own log: forbidden");
    for bus in [&owner,&agent,&third] {bus.close().await.unwrap();}
}


fn event_copy(message: &Message) -> Value {
    json!({"from":message.from(),"to":message.to(),"topic":message.topic(),
        "tag":message.tag(),"receipt":message.receipt(),"re":message.re(),
        "wait":message.metadata().get("wait").cloned().unwrap_or(json!("")),
        "body":serde_json::from_str::<Value>(message.body()).expect("copy body JSON")})
}

// Executed native subscription example, without an extra SDK abstraction.
async fn example_event_subscription(agent: &Bus, owner: &Bus) -> String {
    async fn call(bus: &Bus, body: &str) -> Value {
        let reply: Value = serde_json::from_str(&event_call(bus,body).await).unwrap();
        assert!(reply.get("error").is_none(),"subscription example refused: {reply}");
        reply
    }
    let row=call(owner,"sub add jobs@fixture #caller@fixture alert").await;
    let id=row["id"].as_str().expect("subscription id");
    let listed=call(owner,"sub list #caller@fixture").await;
    assert_eq!(listed["subscriptions"].as_array().unwrap().len(),1,"subscription listing");
    assert_eq!(listed["subscriptions"][0]["id"],id,"listed id");
    assert_eq!(call(agent,"write #caller@fixture alert disk full").await["deliveries"]["sent"],1,"one copy sent");
    let options=ConsumeOptions {inbox:Some("jobs@fixture".into()),wait:Duration::from_millis(300),..Default::default()};
    let copy=owner.consume(options.clone()).await.unwrap().expect("missing subscription copy");
    let value=event_copy(&copy);
    assert_eq!((copy.from(),copy.topic(),copy.tag()),("#bus","#caller@fixture","alert"),"copy route");
    assert_eq!(value["body"]["level"],"alert");assert_eq!(value["body"]["text"],"disk full");
    assert_eq!((copy.receipt(),copy.re(),value["wait"].as_str().unwrap()),("","",""),"copy is a notice");
    // No ack/done for a non-waiting notice.
    assert!(owner.consume(options.clone()).await.unwrap().is_none(),"duplicate copy");
    assert_eq!(call(owner,&format!("sub delete {id}")).await["deleted"],true,"subscription deletion");
    assert_eq!(call(agent,"write #caller@fixture alert after cancellation").await["deliveries"]["matched"],0,"cancelled subscription matched");
    assert!(owner.consume(options).await.unwrap().is_none(),"copy after cancellation");
    "listed: 1 subscription; copy: alert disk full; receipt: none for a notice; cancelled: no further copies".into()
}
#[tokio::test(flavor = "multi_thread", worker_threads = 2)]
async fn native_event_subscription_example() {
    let (_fixture,v)=fixture();
    let owner=connect(&v,"owner@fixture").await;
    let agent=connect(&v,"#caller@fixture").await;
    let result=tokio::time::timeout(Duration::from_secs(30),example_event_subscription(&agent,&owner)).await.expect("subscription example deadline");
    assert_eq!(result,"listed: 1 subscription; copy: alert disk full; receipt: none for a notice; cancelled: no further copies");
    for bus in [&owner,&agent] {bus.close().await.unwrap();}
}
