//! Pin the certificate before any HTTP credentials are transmitted. Signature
//! verification stays with rustls's maintained crypto provider.
use crate::{BusError, Result};
use rustls::{
    DigitallySignedStruct, SignatureScheme,
    client::danger::{HandshakeSignatureValid, ServerCertVerified, ServerCertVerifier},
    pki_types::{CertificateDer, ServerName, UnixTime},
};
use sha2::{Digest, Sha256};
use std::sync::Arc;
#[derive(Debug)]
struct Pinned {
    digest: [u8; 32],
    provider: Arc<rustls::crypto::CryptoProvider>,
}
impl ServerCertVerifier for Pinned {
    fn verify_server_cert(
        &self,
        end: &CertificateDer<'_>,
        _: &[CertificateDer<'_>],
        _: &ServerName<'_>,
        _: &[u8],
        _: UnixTime,
    ) -> std::result::Result<ServerCertVerified, rustls::Error> {
        if Sha256::digest(end.as_ref()).as_slice() != self.digest {
            return Err(rustls::Error::General(
                "agent-bus TLS fingerprint mismatch".into(),
            ));
        }
        Ok(ServerCertVerified::assertion())
    }
    fn verify_tls12_signature(
        &self,
        message: &[u8],
        cert: &CertificateDer<'_>,
        signature: &DigitallySignedStruct,
    ) -> std::result::Result<HandshakeSignatureValid, rustls::Error> {
        rustls::crypto::verify_tls12_signature(
            message,
            cert,
            signature,
            &self.provider.signature_verification_algorithms,
        )
    }
    fn verify_tls13_signature(
        &self,
        message: &[u8],
        cert: &CertificateDer<'_>,
        signature: &DigitallySignedStruct,
    ) -> std::result::Result<HandshakeSignatureValid, rustls::Error> {
        rustls::crypto::verify_tls13_signature(
            message,
            cert,
            signature,
            &self.provider.signature_verification_algorithms,
        )
    }
    fn supported_verify_schemes(&self) -> Vec<SignatureScheme> {
        self.provider
            .signature_verification_algorithms
            .supported_schemes()
    }
}
pub(crate) fn pinned_config(pin: &str) -> Result<rustls::ClientConfig> {
    let pin = pin.trim().to_lowercase();
    let pin = pin.strip_prefix("sha256:").unwrap_or(&pin).replace(':', "");
    if pin.len() != 64 || !pin.bytes().all(|b| b.is_ascii_hexdigit()) {
        return Err(BusError::Validation(
            "TLS fingerprint must be 32 SHA256 hex bytes".into(),
        ));
    }
    let mut digest = [0; 32];
    for (i, b) in digest.iter_mut().enumerate() {
        *b = u8::from_str_radix(&pin[i * 2..i * 2 + 2], 16).unwrap();
    }
    let provider = Arc::new(rustls::crypto::ring::default_provider());
    Ok(
        rustls::ClientConfig::builder_with_provider(provider.clone())
            .with_safe_default_protocol_versions()
            .map_err(|e| BusError::Validation(e.to_string()))?
            .dangerous()
            .with_custom_certificate_verifier(Arc::new(Pinned { digest, provider }))
            .with_no_client_auth(),
    )
}
