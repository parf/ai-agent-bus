//! Async, credential-bound agent-bus client. Writes are never automatically retried.
/// Shared daemon release version, independent of unpublished Cargo metadata.
pub const VERSION: &str = env!("AGENT_BUS_VERSION");
pub use reqwest::Certificate;
mod storage;
mod tls;
use reqwest::{Client, Method};
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};
use std::{
    collections::HashMap,
    fmt,
    future::Future,
    path::PathBuf,
    sync::{Arc, Mutex, Weak},
    time::{Duration, Instant},
};
pub use storage::*;
use tokio::sync::{Notify, OwnedSemaphorePermit, Semaphore, mpsc};
use tokio::task::JoinSet;
pub use tokio_util::sync::CancellationToken;

#[derive(Debug, thiserror::Error)]
pub enum BusError {
    #[error("{0}")]
    Validation(String),
    #[error("expected caller {expected}, authenticated as {actual}")]
    IdentityMismatch { expected: String, actual: String },
    #[error("inbox reader is already in use")]
    ReaderInUse,
    #[error("client is closed")]
    Closed,
    #[error("deadline exceeded (acknowledged: {acked})")]
    Timeout { acked: bool },
    #[error("HTTP {status}: {message}")]
    Refusal { status: u16, message: String },
    #[error("{operation} outcome is unknown; do not replay: {message}")]
    UncertainOutcome { operation: String, message: String },
    #[error("transport failed: {0}")]
    Transport(#[from] reqwest::Error),
    #[error("invalid daemon response: {0}")]
    Protocol(String),
}
pub type Result<T> = std::result::Result<T, BusError>;

/// Received envelope. Fields remain private so server-stamped roles cannot be edited.
#[derive(Debug, Clone, Deserialize)]
pub struct Message {
    #[serde(default)]
    message_id: String,
    #[serde(default)]
    from: String,
    #[serde(default)]
    to: String,
    #[serde(default)]
    body: String,
    #[serde(default)]
    topic: String,
    #[serde(default)]
    tag: String,
    #[serde(default)]
    receipt: String,
    #[serde(default)]
    re: String,
    #[serde(default)]
    ask: String,
    #[serde(default)]
    roles: Vec<String>,
    #[serde(default)]
    original_to: String,
    #[serde(default)]
    reply_to: Option<ReplyTo>,
    #[serde(flatten)]
    metadata: HashMap<String, Value>,
}
impl Message {
    pub fn id(&self) -> &str {
        &self.message_id
    }
    pub fn from(&self) -> &str {
        &self.from
    }
    pub fn to(&self) -> &str {
        &self.to
    }
    pub fn body(&self) -> &str {
        &self.body
    }
    pub fn topic(&self) -> &str {
        &self.topic
    }
    pub fn tag(&self) -> &str {
        &self.tag
    }
    pub fn receipt(&self) -> &str {
        &self.receipt
    }
    pub fn re(&self) -> &str {
        &self.re
    }
    /// What the sender asks about the receiver itself, `help`, `help-config`, `help-secret`, `help-json`, `mcp` or `status`; empty for ordinary work.
    pub fn ask(&self) -> &str {
        &self.ask
    }
    pub fn roles(&self) -> &[String] {
        &self.roles
    }
    pub fn original_to(&self) -> &str {
        &self.original_to
    }
    pub fn reply_to(&self) -> Option<&ReplyTo> {
        self.reply_to.as_ref()
    }
    fn deadline_expired(&self) -> Result<bool> {
        let Some(value) = self.metadata.get("deadline") else {
            return Ok(false);
        };
        if value.is_null() {
            return Ok(false);
        }
        let text = value
            .as_str()
            .ok_or_else(|| BusError::Protocol("deadline must be RFC3339 text".into()))?;
        // Go's zero time denotes an absent deadline on the existing wire.
        if text.is_empty() || text == "0001-01-01T00:00:00Z" {
            return Ok(false);
        }
        let deadline =
            time::OffsetDateTime::parse(text, &time::format_description::well_known::Rfc3339)
                .map_err(|error| BusError::Protocol(error.to_string()))?;
        Ok(deadline <= time::OffsetDateTime::now_utc())
    }
    /// Remaining envelope fields, including at/expires/deadline/forwards.
    pub fn metadata(&self) -> &HashMap<String, Value> {
        &self.metadata
    }
}
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ReplyTo {
    pub name: String,
    #[serde(default)]
    pub topic: String,
    #[serde(default)]
    pub tag: String,
}
#[derive(Debug, Clone, Default)]
pub struct SendOptions {
    pub topic: Option<String>,
    pub tag: Option<String>,
    pub ttl: Option<Duration>,
    pub wait: Option<Duration>,
    pub reply_to: Option<ReplyTo>,
    /// Asks the receiver about itself, `help`, `help-config`, `help-secret`, `help-json`, `mcp` or `status`, instead of giving it work; the body must be empty.
    pub ask: Option<String>,
}
#[derive(Clone)]
pub struct CallOptions {
    pub timeout: Duration,
    pub topic: Option<String>,
    pub ttl: Option<Duration>,
    pub reply_inbox: Option<String>,
    pub ask: Option<String>,
    pub on_ack: Option<Arc<dyn Fn(&Message) + Send + Sync>>,
}
impl Default for CallOptions {
    fn default() -> Self {
        Self {
            timeout: Duration::from_secs(30),
            topic: None,
            ttl: None,
            reply_inbox: None,
            ask: None,
            on_ack: None,
        }
    }
}
#[derive(Debug, Clone)]
pub struct ConsumeOptions {
    pub inbox: Option<String>,
    pub topic: Option<String>,
    pub tag: Option<String>,
    pub wait: Duration,
    pub share: bool,
}
impl Default for ConsumeOptions {
    fn default() -> Self {
        Self {
            inbox: None,
            topic: None,
            tag: None,
            wait: Duration::from_secs(30),
            share: false,
        }
    }
}
#[derive(Debug)]
pub enum CallOutcome {
    Reply(Message),
    Done(Message),
}
#[derive(Debug)]
pub enum HandlerReply {
    Body(String),
    Done,
    Manual,
}
#[derive(Debug, Clone)]
pub struct ServeOptions {
    pub workers: usize,
    pub inbox: Option<String>,
    pub share: bool,
    pub drain_timeout: Duration,
}
impl Default for ServeOptions {
    fn default() -> Self {
        Self {
            workers: 1,
            inbox: None,
            share: false,
            drain_timeout: Duration::from_secs(30),
        }
    }
}
#[derive(Debug, Default)]
pub struct ServeReport {
    pub completed: usize,
    pub failed: usize,
    pub unfinished: usize,
}

#[derive(Clone)]
pub struct Bus {
    pub(crate) inner: Arc<Inner>,
}
pub struct BusBuilder {
    name: String,
    /// None: AGENT_BUS_ADDR, else the socket for the token finally sent (resolved at connect).
    addr: Option<String>,
    token: Option<String>,
    timeout: Duration,
    socket: Option<PathBuf>,
    roots: Vec<reqwest::Certificate>,
    fingerprint: Option<String>,
}
struct Pending {
    polling: bool,
    inbox: String,
    topic: String,
    tag: String,
    tx: mpsc::UnboundedSender<Result<Message>>,
}
struct Serving {
    inbox: String,
    share: bool,
    slots: Arc<Semaphore>,
    tx: mpsc::Sender<Result<Work>>,
}
struct Work {
    message: Message,
    _permit: OwnedSemaphorePermit,
}
#[derive(Default)]
struct ReaderState {
    pending: HashMap<String, Pending>,
    serving: Option<Serving>,
    raw: bool,
}
pub(crate) struct Inner {
    call_prefix: String,
    poll_gate: tokio::sync::Mutex<()>,
    client: Client,
    base: String,
    token: Option<String>,
    timeout: Duration,
    name: String,
    reader: Mutex<ReaderState>,
    notify: Notify,
    close: CancellationToken,
    records: Mutex<HashMap<String, Arc<RecordInner>>>,
}
impl fmt::Debug for Bus {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Bus")
            .field("name", &self.inner.name)
            .finish_non_exhaustive()
    }
}
impl BusBuilder {
    pub fn transport(mut self, address: impl Into<String>) -> Self {
        self.addr = Some(address.into());
        self.socket = None;
        self
    }
    pub fn unix_socket(mut self, path: impl Into<PathBuf>) -> Self {
        self.socket = Some(path.into());
        self
    }
    pub fn tls_fingerprint(mut self, fingerprint: impl Into<String>) -> Self {
        self.fingerprint = Some(fingerprint.into());
        self
    }
    pub fn credential(mut self, token: impl Into<String>) -> Self {
        self.token = Some(token.into());
        self
    }
    pub fn timeout(mut self, timeout: Duration) -> Self {
        self.timeout = timeout;
        self
    }
    /// Trust an additional CA while preserving identity, pin and retry policies.
    pub fn root_certificate(mut self, certificate: reqwest::Certificate) -> Self {
        self.roots.push(certificate);
        self
    }
    pub async fn connect(self) -> Result<Bus> {
        if self.name.is_empty() || self.timeout.is_zero() {
            return Err(BusError::Validation(
                "name and positive timeout required".into(),
            ));
        }
        let addr = self.addr.clone().unwrap_or_else(|| {
            default_socket(self.token.as_deref().is_some_and(|t| !t.is_empty()))
        });
        let socket = self.socket.or_else(|| {
            if addr.starts_with('/') {
                Some(PathBuf::from(&addr))
            } else {
                addr.strip_prefix("unix://").map(PathBuf::from)
            }
        });
        let base = if socket.is_some() {
            "http://localhost".to_owned()
        } else {
            addr.trim_end_matches('/').to_owned()
        };
        let mut builder = Client::builder()
            .redirect(reqwest::redirect::Policy::none())
            .retry(reqwest::retry::never());
        for root in self.roots {
            builder = builder.add_root_certificate(root);
        }
        if let Some(path) = socket {
            builder = builder.unix_socket(path);
        }
        if let Some(pin) = self.fingerprint {
            if !base.starts_with("https://") {
                return Err(BusError::Validation(
                    "TLS fingerprint requires HTTPS".into(),
                ));
            }
            builder = builder.use_preconfigured_tls(tls::pinned_config(&pin)?);
        }
        let client = builder.build()?;
        let bus = Bus {
            inner: Arc::new(Inner {
                call_prefix: format!("call-{}-", uuid::Uuid::new_v4()),
                poll_gate: tokio::sync::Mutex::new(()),
                client,
                base,
                token: self.token,
                timeout: self.timeout,
                name: self.name,
                reader: Mutex::new(ReaderState::default()),
                notify: Notify::new(),
                close: CancellationToken::new(),
                records: Mutex::new(HashMap::new()),
            }),
        };
        let status = bus.status().await?;
        let actual = status
            .get("you")
            .and_then(Value::as_str)
            .ok_or_else(|| BusError::Protocol("status lacks caller identity".into()))?;
        if actual != bus.inner.name {
            return Err(BusError::IdentityMismatch {
                expected: bus.inner.name.clone(),
                actual: actual.into(),
            });
        }
        tokio::spawn(dispatcher(Arc::downgrade(&bus.inner)));
        Ok(bus)
    }
}
impl Bus {
    pub fn builder(name: impl Into<String>) -> BusBuilder {
        BusBuilder {
            name: name.into(),
            addr: std::env::var("AGENT_BUS_ADDR").ok(),
            token: std::env::var("AGENT_BUS_TOKEN").ok(),
            timeout: Duration::from_secs(30),
            socket: None,
            roots: Vec::new(),
            fingerprint: std::env::var("AGENT_BUS_TLS_FINGERPRINT")
                .ok()
                .filter(|p| !p.trim().is_empty()),
        }
    }
    pub fn name(&self) -> &str {
        &self.inner.name
    }
    pub async fn identity(&self) -> Result<Value> {
        self.json(Method::GET, "/identity", &[], None, None).await
    }
    pub async fn status(&self) -> Result<Value> {
        self.json(Method::GET, "/status", &[], None, None).await
    }
    pub async fn records(&self, filters: &[(&str, &str)]) -> Result<Value> {
        self.json(Method::GET, "/ls", filters, None, None).await
    }
    pub(crate) fn ensure_open(&self) -> Result<()> {
        if self.inner.close.is_cancelled() {
            Err(BusError::Closed)
        } else {
            Ok(())
        }
    }
    pub async fn close(&self) -> Result<()> {
        self.inner.close.cancel();
        self.inner.notify.notify_waiters();
        Ok(())
    }
    pub fn record(&self, name: impl Into<String>) -> Result<Record> {
        let name = name.into();
        if name.is_empty() {
            return Err(BusError::Validation("record name required".into()));
        }
        let mut records = self.inner.records.lock().unwrap();
        let inner = records.get(&name).cloned().unwrap_or_else(|| {
            let r = Arc::new(RecordInner::new(name.clone()));
            records.insert(name, r.clone());
            r
        });
        Ok(Record {
            bus: self.clone(),
            inner,
        })
    }
    pub fn own_record(&self) -> Record {
        self.record(self.name()).expect("verified name")
    }
    pub fn kv(&self) -> RecordKv {
        self.own_record().kv()
    }
    pub fn locks(&self) -> RecordLocks {
        self.own_record().locks()
    }
    pub async fn config(&self) -> Result<Option<Arc<Value>>> {
        self.own_record().config().await
    }
    pub async fn secret(&self) -> Result<Arc<Secret>> {
        self.own_record().secret().await
    }
    pub fn invalidate_private(&self, selection: PrivateSelection) {
        self.own_record().invalidate_private(selection)
    }
    pub(crate) async fn bytes(
        &self,
        method: Method,
        path: &str,
        query: &[(&str, &str)],
        body: Option<Value>,
        timeout: Option<Duration>,
    ) -> Result<Option<Vec<u8>>> {
        if self.inner.close.is_cancelled() {
            return Err(BusError::Closed);
        }
        let uncertain = mutating(&method, path);
        let operation = format!("{method} {path}");
        let mut r = self
            .inner
            .client
            .request(method, format!("{}{}", self.inner.base, path))
            .query(query)
            .timeout(timeout.unwrap_or(self.inner.timeout));
        if let Some(token) = &self.inner.token {
            r = r.header("X-Agent-Bus-Token", token);
        }
        if let Some(body) = body {
            r = r.json(&body);
        }
        let response = tokio::select! {
            _ = self.inner.close.cancelled() => return Err(if uncertain {
                BusError::UncertainOutcome { operation, message: "client closed during request".into() }
            } else { BusError::Closed }),
            result = r.send() => result.map_err(|error| transport_failure(error, uncertain, &operation))?,
        };
        let status = response.status();
        if status.as_u16() == 204 {
            return Ok(None);
        }
        let bytes = tokio::select! {
            _ = self.inner.close.cancelled() => return Err(if uncertain {
                BusError::UncertainOutcome { operation, message: "client closed before response body completed".into() }
            } else { BusError::Closed }),
            result = response.bytes() => match result {
                Ok(bytes) => bytes,
                Err(error) if !status.is_success() => return Err(BusError::Refusal { status: status.as_u16(), message: error.to_string() }),
                Err(error) => return Err(transport_failure(error, uncertain, &operation)),
            },
        };
        if !status.is_success() {
            let msg = serde_json::from_slice::<Value>(&bytes)
                .ok()
                .and_then(|v| v["error"].as_str().map(str::to_owned))
                .unwrap_or_else(|| String::from_utf8_lossy(&bytes).into_owned());
            return Err(BusError::Refusal {
                status: status.as_u16(),
                message: msg,
            });
        }
        Ok(Some(bytes.to_vec()))
    }
    pub(crate) async fn json(
        &self,
        method: Method,
        path: &str,
        query: &[(&str, &str)],
        body: Option<Value>,
        timeout: Option<Duration>,
    ) -> Result<Value> {
        let uncertain = mutating(&method, path);
        let operation = format!("{method} {path}");
        match self.bytes(method, path, query, body, timeout).await? {
            None => Ok(Value::Null),
            Some(b) if b.is_empty() => Ok(Value::Null),
            Some(b) => serde_json::from_slice(&b).map_err(|error| {
                if uncertain {
                    BusError::UncertainOutcome {
                        operation,
                        message: error.to_string(),
                    }
                } else {
                    BusError::Protocol(error.to_string())
                }
            }),
        }
    }
    pub async fn send(&self, to: &str, body: &str, options: SendOptions) -> Result<Message> {
        let mut v = json!({"to":to,"body":body});
        if let Some(x) = options.topic {
            v["topic"] = json!(x);
        }
        if let Some(x) = options.tag {
            v["tag"] = json!(x);
        }
        if let Some(x) = options.ttl {
            v["ttl"] = json!(duration(x));
        }
        // A wait of nothing is no wait: the daemon refuses a zero one.
        if let Some(x) = options.wait.filter(|x| !x.is_zero()) {
            v["wait"] = json!(duration(x));
        }
        if let Some(x) = options.reply_to {
            v["reply_to"] = json!(x);
        }
        if let Some(x) = options.ask {
            v["ask"] = json!(x);
        }
        accepted_message(self.json(Method::POST, "/send", &[], Some(v), None).await?)
    }
    pub async fn publish(
        &self,
        channel: &str,
        body: &str,
        options: SendOptions,
    ) -> Result<Message> {
        self.send(channel, body, options).await
    }
    pub async fn unsubscribe(&self, channel: &str) -> Result<Value> {
        self.json(
            Method::POST,
            "/subscribe",
            &[],
            Some(json!({"channel":channel,"off":true})),
            None,
        )
        .await
    }
    pub async fn reply(&self, message: &Message, body: &str) -> Result<Message> {
        self.respond(message, body, None).await
    }
    pub async fn ack(&self, message: &Message) -> Result<Message> {
        self.respond(message, "", Some("ack")).await
    }
    pub async fn done(&self, message: &Message) -> Result<Message> {
        self.respond(message, "", Some("done")).await
    }
    async fn respond(&self, m: &Message, body: &str, receipt: Option<&str>) -> Result<Message> {
        if !m.receipt.is_empty() {
            return Err(BusError::Validation("cannot respond to a receipt".into()));
        }
        let (to, topic, tag) = match &m.reply_to {
            Some(r) => (r.name.as_str(), r.topic.as_str(), r.tag.as_str()),
            None => (m.from(), m.topic(), m.tag()),
        };
        let mut v = json!({"to":to,"body":body,"topic":topic,"tag":tag,"re":m.id()});
        if let Some(receipt) = receipt {
            v["receipt"] = json!(receipt);
            v["re"] = json!(m.id());
        }
        accepted_message(self.json(Method::POST, "/send", &[], Some(v), None).await?)
    }
    pub async fn call(&self, to: &str, body: &str, options: CallOptions) -> Result<CallOutcome> {
        if options.timeout.is_zero() {
            return Err(BusError::Validation(
                "positive call timeout required".into(),
            ));
        }
        self.ensure_open()?;
        let deadline = tokio::time::Instant::now() + options.timeout;
        let id = format!("{}{}", self.inner.call_prefix, uuid::Uuid::new_v4());
        let topic = options.topic.clone().unwrap_or_else(|| "call".into());
        let inbox = options
            .reply_inbox
            .clone()
            .unwrap_or_else(|| self.name().into());
        let (tx, mut rx) = mpsc::unbounded_channel();
        {
            let mut state = self.inner.reader.lock().unwrap();
            if state.raw {
                return Err(BusError::ReaderInUse);
            }
            state.pending.insert(
                id.clone(),
                Pending {
                    polling: true,
                    inbox: inbox.clone(),
                    topic: topic.clone(),
                    tag: id.clone(),
                    tx,
                },
            );
        }
        let _guard = PendingGuard {
            inner: self.inner.clone(),
            id: id.clone(),
        };
        self.inner.notify.notify_one();
        let request = SendOptions {
            topic: Some(topic.clone()),
            tag: Some(id.clone()),
            ttl: options.ttl,
            wait: Some(options.timeout),
            reply_to: options.reply_inbox.map(|name| ReplyTo {
                name,
                topic,
                tag: id,
            }),
            ask: options.ask,
        };
        let mut acked = false;
        let mut accepted = false;
        let exchange = async {
            let sent = self.send(to, body, request).await?;
            accepted = true;
            loop {
                let msg = tokio::select! {
                    biased;
                    _ = self.inner.close.cancelled() => return Err(BusError::Closed),
                    msg = rx.recv() => msg,
                };
                let Some(msg) = msg else {
                    return Err(BusError::Closed);
                };
                let msg = msg?;
                if tokio::time::Instant::now() >= deadline {
                    return Err(BusError::Timeout { acked });
                }
                if !msg.receipt().is_empty() && msg.re() != sent.id() {
                    continue;
                }
                match msg.receipt() {
                    "ack" => {
                        acked = true;
                        if let Some(f) = &options.on_ack {
                            f(&msg);
                        }
                    }
                    "done" => return Ok(CallOutcome::Done(msg)),
                    _ => return Ok(CallOutcome::Reply(msg)),
                }
            }
        };
        match tokio::time::timeout_at(deadline, exchange).await {
            Ok(r) => r,
            Err(_) if !accepted => Err(BusError::UncertainOutcome {
                operation: "POST /send".into(),
                message: "call deadline elapsed before acceptance was confirmed".into(),
            }),
            Err(_) => Err(BusError::Timeout { acked }),
        }
    }
    pub async fn consume(&self, options: ConsumeOptions) -> Result<Option<Message>> {
        {
            let mut s = self.inner.reader.lock().unwrap();
            if s.raw || s.serving.is_some() || !s.pending.is_empty() {
                return Err(BusError::ReaderInUse);
            }
            s.raw = true;
        }
        let _guard = RawGuard(self.inner.clone());
        self.consume_wire(&options).await
    }
    async fn consume_wire(&self, o: &ConsumeOptions) -> Result<Option<Message>> {
        let _poll = self.inner.poll_gate.lock().await;
        let wait = duration(o.wait.min(Duration::from_secs(60)));
        let mut q = vec![("wait", wait.as_str())];
        if let Some(x) = &o.inbox {
            q.push(("inbox", x));
        }
        if let Some(x) = &o.topic {
            q.push(("topic", x));
        }
        if let Some(x) = &o.tag {
            q.push(("tag", x));
        }
        if o.share {
            q.push(("share", "1"));
        }
        match self
            .bytes(
                Method::GET,
                "/consume",
                &q,
                None,
                Some(o.wait.min(Duration::from_secs(60)) + self.inner.timeout),
            )
            .await?
        {
            None => Ok(None),
            Some(b) => {
                serde_json::from_slice(&b)
                    .map(Some)
                    .map_err(|e| BusError::UncertainOutcome {
                        operation: "GET /consume".into(),
                        message: e.to_string(),
                    })
            }
        }
    }
    pub async fn serve<H, F>(
        &self,
        handler: H,
        options: ServeOptions,
        cancellation: CancellationToken,
    ) -> Result<ServeReport>
    where
        H: Fn(Message) -> F + Send + Sync + 'static,
        F: Future<Output = Result<HandlerReply>> + Send + 'static,
    {
        self.ensure_open()?;
        if options.workers == 0 {
            return Err(BusError::Validation("workers must be positive".into()));
        }
        let (tx, mut rx) = mpsc::channel(options.workers);
        {
            let mut s = self.inner.reader.lock().unwrap();
            if s.raw || s.serving.is_some() {
                return Err(BusError::ReaderInUse);
            }
            s.serving = Some(Serving {
                inbox: options.inbox.unwrap_or_else(|| self.name().into()),
                share: options.share,
                slots: Arc::new(Semaphore::new(options.workers)),
                tx,
            });
        }
        let guard = ServeGuard(self.inner.clone());
        self.inner.notify.notify_one();
        let handler = Arc::new(handler);
        let mut tasks = JoinSet::new();
        let mut report = ServeReport::default();
        let mut intake_error = None;
        loop {
            tokio::select! {
                biased;
                _ = cancellation.cancelled() => break,
                _ = self.inner.close.cancelled() => break,
                Some(joined) = tasks.join_next(), if !tasks.is_empty() => count(joined, &mut report),
                work = rx.recv() => match work {
                    Some(Ok(work)) => {
                        spawn_work(&mut tasks, work, self.clone(), handler.clone());
                    }
                    Some(Err(error)) => { intake_error = Some(error); break; }
                    None => break,
                }
            }
        }
        drop(guard); // stop unfiltered intake; pending calls keep polling while handlers drain
        let drain = async {
            // Sender clones held by reserved in-flight polls keep the channel
            // open until every known successful take reaches this drain.
            while let Some(work) = rx.recv().await {
                match work {
                    Ok(work) => spawn_work(&mut tasks, work, self.clone(), handler.clone()),
                    Err(error) => {
                        if intake_error.is_none() {
                            intake_error = Some(error)
                        }
                    }
                }
            }
            while let Some(r) = tasks.join_next().await {
                count(r, &mut report);
            }
        };
        if tokio::time::timeout(options.drain_timeout, drain)
            .await
            .is_err()
        {
            report.unfinished = tasks.len() + rx.len();
            tasks.abort_all();
        }
        if let Some(e) = intake_error {
            return Err(e);
        }
        Ok(report)
    }
}
fn spawn_work<H, F>(tasks: &mut JoinSet<Result<()>>, work: Work, bus: Bus, handler: Arc<H>)
where
    H: Fn(Message) -> F + Send + Sync + 'static,
    F: Future<Output = Result<HandlerReply>> + Send + 'static,
{
    tasks.spawn(async move {
        let _permit = work._permit;
        let message = work.message;
        if !message.receipt.is_empty()
            || !message.re.is_empty()
            || message.tag.starts_with(&bus.inner.call_prefix)
        {
            return Ok(());
        }
        if message.deadline_expired()? {
            return Ok(());
        }
        bus.ack(&message).await?;
        if message.deadline_expired()? {
            return Ok(());
        }
        match handler(message.clone()).await? {
            HandlerReply::Body(body) => {
                bus.reply(&message, &body).await?;
            }
            HandlerReply::Done => {
                bus.done(&message).await?;
            }
            HandlerReply::Manual => {}
        }
        Ok(())
    });
}
fn count(r: std::result::Result<Result<()>, tokio::task::JoinError>, report: &mut ServeReport) {
    match r {
        Ok(Ok(())) => report.completed += 1,
        _ => report.failed += 1,
    }
}
pub(crate) fn duration(d: Duration) -> String {
    format!("{}ns", d.as_nanos())
}
struct PendingGuard {
    inner: Arc<Inner>,
    id: String,
}
impl Drop for PendingGuard {
    fn drop(&mut self) {
        self.inner.reader.lock().unwrap().pending.remove(&self.id);
        self.inner.notify.notify_one();
    }
}
struct RawGuard(Arc<Inner>);
impl Drop for RawGuard {
    fn drop(&mut self) {
        self.0.reader.lock().unwrap().raw = false;
        self.0.notify.notify_one();
    }
}
struct ServeGuard(Arc<Inner>);
impl Drop for ServeGuard {
    fn drop(&mut self) {
        self.0.reader.lock().unwrap().serving = None;
        self.0.notify.notify_one();
    }
}
async fn dispatcher(weak: Weak<Inner>) {
    let mut turn = 0usize;
    loop {
        let Some(inner) = weak.upgrade() else {
            return;
        };
        if Arc::strong_count(&inner) == 1 {
            return;
        }
        if inner.close.is_cancelled() {
            return;
        }
        let bus = Bus {
            inner: inner.clone(),
        };
        let (pending, work) = {
            let s = inner.reader.lock().unwrap();
            let mut p = s
                .pending
                .iter()
                .filter(|(_, pending)| pending.polling)
                .map(|(id, p)| (id.clone(), p.inbox.clone(), p.topic.clone(), p.tag.clone()))
                .collect::<Vec<_>>();
            p.sort_by(|a, b| a.0.cmp(&b.0));
            let work = if turn % 2 == 1 || p.is_empty() {
                s.serving.as_ref().and_then(|s| {
                    s.slots
                        .clone()
                        .try_acquire_owned()
                        .ok()
                        .map(|permit| (s.inbox.clone(), s.share, s.tx.clone(), permit))
                })
            } else {
                None
            };
            (p, work)
        };
        turn = turn.wrapping_add(1);
        if let Some((inbox, share, tx, permit)) = work {
            let result = bus
                .consume_wire(&ConsumeOptions {
                    inbox: Some(inbox),
                    wait: Duration::from_millis(100),
                    share,
                    ..Default::default()
                })
                .await;
            match result {
                Ok(Some(message)) => {
                    let matched = {
                        let s = inner.reader.lock().unwrap();
                        s.pending
                            .values()
                            .find(|p| {
                                p.topic == message.topic
                                    && p.tag == message.tag
                                    && p.inbox == message.to
                            })
                            .map(|p| p.tx.clone())
                    };
                    if let Some(p) = matched {
                        let _ = p.send(Ok(message));
                    } else {
                        let _ = tx
                            .send(Ok(Work {
                                message,
                                _permit: permit,
                            }))
                            .await;
                    }
                }
                Ok(None) => {}
                Err(e) => {
                    let _ = tx.send(Err(e)).await;
                }
            }
        } else if !pending.is_empty() {
            let (id, inbox, topic, tag) = &pending[(turn / 2) % pending.len()];
            let result = bus
                .consume_wire(&ConsumeOptions {
                    inbox: Some(inbox.clone()),
                    topic: Some(topic.clone()),
                    tag: Some(tag.clone()),
                    wait: Duration::from_millis(100),
                    ..Default::default()
                })
                .await;
            if !matches!(result, Ok(None)) {
                let tx = {
                    let mut state = inner.reader.lock().unwrap();
                    state.pending.get_mut(id).map(|pending| {
                        if result.is_err() {
                            pending.polling = false;
                        }
                        pending.tx.clone()
                    })
                };
                if let Some(tx) = tx {
                    let _ = tx.send(result.and_then(|m| {
                        m.ok_or_else(|| BusError::Protocol("missing message".into()))
                    }));
                }
            }
        } else {
            tokio::select! {_=inner.close.cancelled()=>return,_=inner.notify.notified()=>{},_=tokio::time::sleep(Duration::from_millis(50))=>{}}
        }
    }
}

fn default_socket(token: bool) -> String {
    let account = users::get_current_username().map(|s| s.to_string_lossy().into_owned());
    let mut dirs = Vec::new();
    if let Ok(dir) = std::env::var("XDG_RUNTIME_DIR") {
        dirs.push(PathBuf::from(dir).join("agent-bus"));
    }
    dirs.push(PathBuf::from("/run/agent-bus"));
    for dir in dirs {
        let name = if !token {
            account
                .as_ref()
                .map(|a| format!("user-{a}.sock"))
                .unwrap_or_else(|| "bus.sock".into())
        } else {
            "bus.sock".into()
        };
        let p = dir.join(name);
        if p.exists() {
            return p.to_string_lossy().into_owned();
        }
    }
    if let Ok(runtime) = std::env::var("XDG_RUNTIME_DIR") {
        return PathBuf::from(runtime)
            .join("agent-bus/bus.sock")
            .to_string_lossy()
            .into_owned();
    }
    std::env::temp_dir()
        .join(format!("agent-bus-{}.sock", users::get_current_uid()))
        .to_string_lossy()
        .into_owned()
}

fn mutating(method: &Method, path: &str) -> bool {
    *method != Method::GET || path == "/consume"
}
fn transport_failure(error: reqwest::Error, uncertain: bool, operation: &str) -> BusError {
    if error.is_connect() || error.is_builder() {
        return BusError::Transport(error);
    }
    if uncertain {
        return BusError::UncertainOutcome {
            operation: operation.into(),
            message: error.to_string(),
        };
    }
    if error.is_timeout() {
        return BusError::Timeout { acked: false };
    }
    BusError::Transport(error)
}
fn accepted_message(value: Value) -> Result<Message> {
    let message: Message =
        serde_json::from_value(value).map_err(|error| BusError::UncertainOutcome {
            operation: "POST /send".into(),
            message: error.to_string(),
        })?;
    if message.id().is_empty() {
        return Err(BusError::UncertainOutcome {
            operation: "POST /send".into(),
            message: "response lacks accepted message id".into(),
        });
    }
    Ok(message)
}
