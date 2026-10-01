use agent_bus::{
    Bus, BusError, CallOptions, CallOutcome, HandlerReply, KvKind, KvValue, LockOptions,
    ServeOptions, SetMode,
};
use serde_json::{Value, json};
use sha2::{Digest, Sha256};
use std::{
    collections::HashMap,
    path::{Path, PathBuf},
    sync::Arc,
    time::Duration,
};
use tokio::sync::Mutex;
type Gates = Arc<Mutex<HashMap<String, Arc<Mutex<()>>>>>;
fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}
async fn read(root: &Path, arg: &Value) -> Result<(String, Vec<u8>), String> {
    let input = arg
        .get("path")
        .and_then(Value::as_str)
        .ok_or("path is required")?;
    let candidate = Path::new(input);
    let path = tokio::fs::canonicalize(if candidate.is_absolute() {
        candidate.to_path_buf()
    } else {
        root.join(candidate)
    })
    .await
    .map_err(|e| e.to_string())?;
    if !path.starts_with(root) {
        return Err("path is outside STATS_ROOT".into());
    };
    let bytes = tokio::fs::read(&path).await.map_err(|e| e.to_string())?;
    Ok((path.to_string_lossy().into_owned(), bytes))
}
async fn answer(
    bus: &Bus,
    root: &Path,
    gates: &Gates,
    topic: &str,
    body: &str,
) -> Result<Value, String> {
    let arg: Value = serde_json::from_str(body).map_err(|e| e.to_string())?;
    if topic == "relay" {
        let mut value = arg;
        let next = value
            .get_mut("route")
            .and_then(Value::as_array_mut)
            .and_then(|route| {
                if route.is_empty() {
                    None
                } else {
                    Some(route.remove(0))
                }
            });
        let hops = value
            .as_object_mut()
            .ok_or("relay needs an object")?
            .entry("hops")
            .or_insert_with(|| json!([]));
        hops.as_array_mut()
            .ok_or("hops needs an array")?
            .push(json!("rust"));
        return if let Some(next) = next {
            let name = next.as_str().ok_or("route entries must be names")?;
            call(bus, name, "relay", &value).await
        } else {
            Ok(value)
        };
    }
    let (path, bytes) = read(root, &arg).await?;
    let digest = hex(&Sha256::digest(&bytes));
    if topic == "stats" {
        bus.kv().inc("files", 1).await.map_err(|e| e.to_string())?;
        let lines = bytes.iter().filter(|b| **b == b'\n').count()
            + usize::from(!bytes.is_empty() && bytes.last() != Some(&b'\n'));
        let todos = bytes.windows(4).filter(|window| *window == b"TODO").count();
        return Ok(
            json!({"worker":"rust","path":path,"bytes":bytes.len(),"lines":lines,"todos":todos,"sha256":digest}),
        );
    }
    if topic != "digest" {
        return Err("supported topics: digest, stats, relay".into());
    }
    // Serialize same-content requests within this principal; daemon locks name
    // principals, so reacquisition by sibling tasks would otherwise be refused.
    let gate = {
        let mut gates = gates.lock().await;
        gates.entry(digest.clone()).or_default().clone()
    };
    let _local = gate.lock().await;
    let key = format!("digest-{digest}");
    let mut lease = bus
        .locks()
        .acquire(
            &key,
            LockOptions {
                ttl: Duration::from_secs(30),
                wait: Duration::ZERO,
            },
        )
        .await
        .map_err(|e| e.to_string())?;
    let result=async {
  let cached=match bus.kv().get(&key,KvKind::Json).await{Ok(KvValue::Json(value))=>Some(value),Err(BusError::Refusal{status:404,..})=>None,Ok(_)=>return Err("cached digest has wrong kind".into()),Err(e)=>return Err(e.to_string())};
  let java=if let Some(cached)=cached.as_ref(){cached["java"].clone()}else{let value=call(bus,"#stats-java@localhost","stats",&json!({"path":path})).await?;if value.get("sha256").and_then(Value::as_str)!=Some(digest.as_str()){return Err("Java SHA256 differs from Rust".into())};bus.kv().set(&key,KvValue::Json(json!({"sha256":digest,"java":value})),SetMode::Set).await.map_err(|e|e.to_string())?;bus.kv().inc("files",1).await.map_err(|e|e.to_string())?;value};
  Ok(json!({"worker":"rust","path":path,"bytes":bytes.len(),"sha256":digest,"cached":cached.is_some(),"java_verified":true,"java":java}))
 }.await;
    let released = lease.release().await.map_err(|e| e.to_string());
    match (result, released) {
        (Ok(value), Ok(())) => Ok(value),
        (Err(e), _) => Err(e),
        (_, Err(e)) => Err(e),
    }
}
async fn call(bus: &Bus, to: &str, topic: &str, value: &Value) -> Result<Value, String> {
    match bus
        .call(
            to,
            &value.to_string(),
            CallOptions {
                topic: Some(topic.into()),
                timeout: Duration::from_secs(20),
                ..Default::default()
            },
        )
        .await
        .map_err(|e| e.to_string())?
    {
        CallOutcome::Reply(message) => {
            serde_json::from_str(message.body()).map_err(|e| e.to_string())
        }
        CallOutcome::Done(_) => Err("peer completed without an answer".into()),
    }
}
#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let name = std::env::var("AGENT_BUS_NAME").unwrap_or_else(|_| "#stats-rust@localhost".into());
    let root = tokio::fs::canonicalize(PathBuf::from(std::env::var("STATS_ROOT")?)).await?;
    let bus = Bus::builder(name).connect().await?;
    let stop = agent_bus::CancellationToken::new();
    let cancel = stop.clone();
    tokio::spawn(async move {
        let mut term = tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
            .expect("SIGTERM handler");
        tokio::select! {_=tokio::signal::ctrl_c()=>{},_=term.recv()=>{}};
        cancel.cancel();
    });
    let handler_bus = bus.clone();
    let gates: Gates = Arc::default();
    let report = bus
        .serve(
            move |message| {
                let bus = handler_bus.clone();
                let root = root.clone();
                let gates = gates.clone();
                async move {
                    let value = answer(&bus, &root, &gates, message.topic(), message.body())
                        .await
                        .unwrap_or_else(|error| json!({"worker":"rust","error":error}));
                    Ok(HandlerReply::Body(value.to_string()))
                }
            },
            ServeOptions {
                workers: 2,
                drain_timeout: Duration::from_secs(25),
                ..Default::default()
            },
            stop,
        )
        .await?;
    bus.close().await?;
    if report.failed > 0 || report.unfinished > 0 {
        return Err(format!(
            "{} failed, {} unfinished handlers",
            report.failed, report.unfinished
        )
        .into());
    };
    Ok(())
}
