use crate::*;
use base64::{Engine, engine::general_purpose::STANDARD};
use tokio::sync::OnceCell;

#[derive(Clone)]
pub struct Record {
    pub(crate) bus: Bus,
    pub(crate) inner: Arc<RecordInner>,
}
pub(crate) struct RecordInner {
    name: String,
    config: Mutex<Arc<OnceCell<Option<Arc<Value>>>>>,
    secret: Mutex<Arc<OnceCell<Arc<Secret>>>>,
}
impl RecordInner {
    pub(crate) fn new(name: String) -> Self {
        Self {
            name,
            config: Mutex::new(Arc::new(OnceCell::new())),
            secret: Mutex::new(Arc::new(OnceCell::new())),
        }
    }
}
#[derive(Clone, Copy, Debug)]
pub enum PrivateSelection {
    Config,
    Secret,
    Both,
}
pub struct Secret(Vec<u8>);
impl Secret {
    pub fn bytes(&self) -> &[u8] {
        &self.0
    }
}
impl fmt::Debug for Secret {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str("Secret([redacted])")
    }
}
impl fmt::Display for Secret {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str("[redacted]")
    }
}
impl Record {
    pub fn name(&self) -> &str {
        &self.inner.name
    }
    pub async fn lookup(&self) -> Result<Value> {
        self.bus
            .json(Method::GET, "/lookup", &[("name", self.name())], None, None)
            .await
    }
    pub async fn register(&self, metadata: Value) -> Result<Value> {
        self.edit("/register", metadata).await
    }
    pub async fn manage(&self, changes: Value) -> Result<Value> {
        self.edit("/manage", changes).await
    }
    async fn edit(&self, path: &str, mut body: Value) -> Result<Value> {
        if !body.is_object() {
            return Err(BusError::Validation(
                "record metadata must be an object".into(),
            ));
        }
        body["name"] = json!(self.name());
        self.bus
            .json(Method::POST, path, &[], Some(body), None)
            .await
    }
    pub async fn unregister(&self) -> Result<()> {
        self.bus
            .json(
                Method::POST,
                "/unregister",
                &[],
                Some(json!({"name":self.name()})),
                None,
            )
            .await?;
        Ok(())
    }
    pub fn kv(&self) -> RecordKv {
        RecordKv(self.clone())
    }
    pub fn locks(&self) -> RecordLocks {
        RecordLocks(self.clone())
    }
    pub async fn config(&self) -> Result<Option<Arc<Value>>> {
        let cell = self.inner.config.lock().unwrap().clone();
        cell.get_or_try_init(|| async {
            let v = self
                .bus
                .json(Method::GET, "/config", &[("name", self.name())], None, None)
                .await?;
            Ok(if v.is_null() { None } else { Some(Arc::new(v)) })
        })
        .await
        .cloned()
    }
    pub async fn secret(&self) -> Result<Arc<Secret>> {
        let cell = self.inner.secret.lock().unwrap().clone();
        cell.get_or_try_init(|| async {
            let bytes = self
                .bus
                .bytes(Method::GET, "/secret", &[("name", self.name())], None, None)
                .await?
                .ok_or_else(|| BusError::Protocol("empty secret response".into()))?;
            Ok(Arc::new(Secret(bytes)))
        })
        .await
        .cloned()
    }
    pub fn invalidate_private(&self, selection: PrivateSelection) {
        if matches!(selection, PrivateSelection::Config | PrivateSelection::Both) {
            *self.inner.config.lock().unwrap() = Arc::new(OnceCell::new());
        }
        if matches!(selection, PrivateSelection::Secret | PrivateSelection::Both) {
            *self.inner.secret.lock().unwrap() = Arc::new(OnceCell::new());
        }
    }
}
#[derive(Debug, Clone, Copy, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum KvKind {
    String,
    Int,
    Json,
}
impl KvKind {
    fn text(self) -> &'static str {
        match self {
            Self::String => "string",
            Self::Int => "int",
            Self::Json => "json",
        }
    }
}
#[derive(Debug, Clone)]
pub enum KvValue {
    Bytes(Vec<u8>),
    Int(i64),
    Json(Value),
}
impl KvValue {
    pub fn utf8(&self) -> Result<&str> {
        match self {
            Self::Bytes(b) => {
                std::str::from_utf8(b).map_err(|e| BusError::Validation(e.to_string()))
            }
            _ => Err(BusError::Validation("value is not bytes".into())),
        }
    }
}
#[derive(Debug, Clone, Copy, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum SetMode {
    Set,
    Add,
    Replace,
}
#[derive(Debug, Clone, Serialize)]
pub struct JsonOp {
    pub op: String,
    pub key: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub value: Option<Value>,
}
#[derive(Clone)]
pub struct RecordKv(Record);
impl RecordKv {
    pub async fn get(&self, key: &str, kind: KvKind) -> Result<KvValue> {
        let v = self
            .0
            .bus
            .json(
                Method::GET,
                "/kv",
                &[
                    ("record", self.0.name()),
                    ("name", key),
                    ("kind", kind.text()),
                ],
                None,
                None,
            )
            .await?;
        match kind {
            KvKind::Int => v["value"]
                .as_i64()
                .map(KvValue::Int)
                .ok_or_else(|| BusError::Protocol("expected int64".into())),
            KvKind::Json => Ok(KvValue::Json(v["value"].clone())),
            KvKind::String => {
                if let Some(s) = v["value_base64"].as_str() {
                    STANDARD
                        .decode(s)
                        .map(KvValue::Bytes)
                        .map_err(|e| BusError::Protocol(e.to_string()))
                } else {
                    v["value"]
                        .as_str()
                        .map(|s| KvValue::Bytes(s.as_bytes().to_vec()))
                        .ok_or_else(|| BusError::Protocol("expected string bytes".into()))
                }
            }
        }
    }
    pub async fn set(&self, key: &str, value: KvValue, mode: SetMode) -> Result<()> {
        let mut v = json!({"record":self.0.name(),"name":key,"how":mode});
        match value {
            KvValue::Int(n) => {
                v["kind"] = json!("int");
                v["value"] = json!(n);
            }
            KvValue::Json(j) => {
                if !j.is_object() {
                    return Err(BusError::Validation("stored JSON must be an object".into()));
                }
                v["kind"] = json!("json");
                v["value"] = j;
            }
            KvValue::Bytes(b) => {
                v["kind"] = json!("string");
                match String::from_utf8(b) {
                    Ok(s) => v["value"] = json!(s),
                    Err(e) => v["value_base64"] = json!(STANDARD.encode(e.into_bytes())),
                }
            }
        }
        self.0
            .bus
            .json(Method::POST, "/kv/set", &[], Some(v), None)
            .await?;
        Ok(())
    }
    pub async fn delete(&self, key: &str, kind: KvKind) -> Result<bool> {
        let v = self
            .0
            .bus
            .json(
                Method::POST,
                "/kv/delete",
                &[],
                Some(json!({"record":self.0.name(),"name":key,"kind":kind})),
                None,
            )
            .await?;
        v["deleted"]
            .as_bool()
            .ok_or_else(|| BusError::Protocol("missing deleted flag".into()))
    }
    pub async fn list(&self) -> Result<Value> {
        self.0
            .bus
            .json(
                Method::GET,
                "/kv/list",
                &[("record", self.0.name())],
                None,
                None,
            )
            .await
    }
    pub async fn inc(&self, key: &str, by: i64) -> Result<i64> {
        let v = self
            .0
            .bus
            .json(
                Method::POST,
                "/kv/inc",
                &[],
                Some(json!({"record":self.0.name(),"name":key,"n":by})),
                None,
            )
            .await?;
        v["value"]
            .as_i64()
            .ok_or_else(|| BusError::Protocol("expected int64 increment".into()))
    }
    pub async fn json(&self, key: &str, operations: &[JsonOp]) -> Result<Value> {
        if operations.len() > 100 {
            return Err(BusError::Validation("at most 100 JSON operations".into()));
        }
        self.0
            .bus
            .json(
                Method::POST,
                "/kv/json",
                &[],
                Some(json!({"record":self.0.name(),"name":key,"ops":operations})),
                None,
            )
            .await
    }
}
#[derive(Clone, Debug)]
pub struct LockOptions {
    pub ttl: Duration,
    pub wait: Duration,
}
impl Default for LockOptions {
    fn default() -> Self {
        Self {
            ttl: Duration::from_secs(30),
            wait: Duration::ZERO,
        }
    }
}
#[derive(Clone)]
pub struct RecordLocks(Record);
pub struct Lease {
    locks: RecordLocks,
    name: String,
    expires: Instant,
    released: bool,
}
impl fmt::Debug for Lease {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("Lease")
            .field("name", &self.name)
            .field("released", &self.released)
            .finish()
    }
}
impl RecordLocks {
    fn ttl(ttl: Duration) -> Result<()> {
        if ttl.is_zero() || ttl > Duration::from_secs(86400) {
            Err(BusError::Validation(
                "lock TTL must be positive and at most 24h".into(),
            ))
        } else {
            Ok(())
        }
    }
    async fn take(&self, path: &str, name: &str, options: LockOptions) -> Result<Lease> {
        Self::ttl(options.ttl)?;
        if options.wait > Duration::from_secs(60) {
            return Err(BusError::Validation("lock wait capped at 60s".into()));
        }
        let started = Instant::now();
        let wait = duration(options.wait);
        self.0
            .bus
            .json(
                Method::POST,
                path,
                &[("wait", &wait)],
                Some(json!({"record":self.0.name(),"name":name,"ttl":duration(options.ttl)})),
                Some(options.wait + self.0.bus.inner.timeout),
            )
            .await?;
        Ok(Lease {
            locks: self.clone(),
            name: name.into(),
            expires: started + options.ttl,
            released: false,
        })
    }
    pub async fn acquire(&self, name: &str, options: LockOptions) -> Result<Lease> {
        self.take("/lock", name, options).await
    }
    pub async fn try_acquire(&self, name: &str, ttl: Duration) -> Result<Lease> {
        self.take(
            "/try-lock",
            name,
            LockOptions {
                ttl,
                wait: Duration::ZERO,
            },
        )
        .await
    }
    pub async fn extend(&self, name: &str, ttl: Duration) -> Result<()> {
        Self::ttl(ttl)?;
        self.0
            .bus
            .json(
                Method::POST,
                "/extend",
                &[],
                Some(json!({"record":self.0.name(),"name":name,"ttl":duration(ttl)})),
                None,
            )
            .await?;
        Ok(())
    }
    pub async fn release(&self, name: &str) -> Result<()> {
        self.0
            .bus
            .json(
                Method::POST,
                "/release",
                &[],
                Some(json!({"record":self.0.name(),"name":name})),
                None,
            )
            .await?;
        Ok(())
    }
    pub async fn force_release(&self, name: &str) -> Result<Option<String>> {
        let v = self
            .0
            .bus
            .json(
                Method::POST,
                "/release-force",
                &[],
                Some(json!({"record":self.0.name(),"name":name})),
                None,
            )
            .await?;
        Ok(v["holder"]
            .as_str()
            .filter(|s| !s.is_empty())
            .map(str::to_owned))
    }
    pub async fn holders(&self) -> Result<Value> {
        self.0
            .bus
            .json(
                Method::GET,
                "/holders",
                &[("record", self.0.name())],
                None,
                None,
            )
            .await
    }
}
impl Lease {
    pub fn name(&self) -> &str {
        &self.name
    }
    fn live(&self) -> Result<()> {
        if self.released || Instant::now() >= self.expires {
            Err(BusError::Validation(
                "lease already released or locally expired".into(),
            ))
        } else {
            Ok(())
        }
    }
    pub async fn extend(&mut self, ttl: Duration) -> Result<()> {
        self.live()?;
        let started = Instant::now();
        self.locks.extend(&self.name, ttl).await?;
        self.expires = started + ttl;
        Ok(())
    }
    pub async fn release(&mut self) -> Result<()> {
        self.live()?;
        self.locks.release(&self.name).await?;
        self.released = true;
        Ok(())
    }
}
