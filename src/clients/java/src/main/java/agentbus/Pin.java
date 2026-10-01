package agentbus;

import java.net.Socket;
import java.security.MessageDigest;
import java.security.cert.CertificateEncodingException;
import java.security.cert.CertificateException;
import java.security.cert.X509Certificate;
import java.util.HexFormat;
import java.util.Locale;
import javax.net.ssl.SSLContext;
import javax.net.ssl.SSLEngine;
import javax.net.ssl.TrustManager;
import javax.net.ssl.X509ExtendedTrustManager;

/**
 * A pinned https:// daemon (docs/02-access-remote.md#over-https): its leaf
 * certificate's SHA-256 is the only trust. The check runs inside the TLS
 * handshake of the very connection the request then uses, so a wrong
 * certificate ends it before any header, token included, is written. The pin is
 * compared as the Go and TypeScript clients compare it: case, colons and a
 * {@code sha256:} prefix do not matter, and a blank pin matches nothing.
 */
final class Pin extends X509ExtendedTrustManager {
    private final String want;

    private Pin(String pin) { this.want = norm(pin); }

    static String norm(String s) {
        return s.strip().toLowerCase(Locale.ROOT).replaceFirst("^sha256:", "").replace(":", "");
    }

    static String fingerprint(byte[] der) {
        try {
            return "sha256:" + HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(der));
        } catch (java.security.NoSuchAlgorithmException e) {
            throw new IllegalStateException(e);
        }
    }

    static SSLContext context(String pin) {
        if (norm(pin).isEmpty()) throw new IllegalArgumentException("a blank pin matches nothing");
        try {
            SSLContext c = SSLContext.getInstance("TLS");
            c.init(null, new TrustManager[] {new Pin(pin)}, null);
            return c;
        } catch (java.security.GeneralSecurityException e) {
            throw new IllegalStateException(e);
        }
    }

    private void check(X509Certificate[] chain) throws CertificateException {
        if (chain == null || chain.length == 0) throw new CertificateException("the daemon showed no certificate");
        String got;
        try {
            got = fingerprint(chain[0].getEncoded());
        } catch (CertificateEncodingException e) {
            throw new CertificateException(e);
        }
        if (!norm(got).equals(want)) throw new CertificateException("the daemon's certificate is " + got + ", not the pinned one");
    }

    // The pin is the identity: a host name check would refuse the address the
    // pin was taken for, so none is made — this is an X509ExtendedTrustManager,
    // which JSSE leaves the whole decision to.
    @Override public void checkServerTrusted(X509Certificate[] chain, String auth) throws CertificateException { check(chain); }
    @Override public void checkServerTrusted(X509Certificate[] chain, String auth, Socket s) throws CertificateException { check(chain); }
    @Override public void checkServerTrusted(X509Certificate[] chain, String auth, SSLEngine e) throws CertificateException { check(chain); }

    @Override public void checkClientTrusted(X509Certificate[] chain, String auth) throws CertificateException { throw new CertificateException("not a server"); }
    @Override public void checkClientTrusted(X509Certificate[] chain, String auth, Socket s) throws CertificateException { throw new CertificateException("not a server"); }
    @Override public void checkClientTrusted(X509Certificate[] chain, String auth, SSLEngine e) throws CertificateException { throw new CertificateException("not a server"); }

    @Override public X509Certificate[] getAcceptedIssuers() { return new X509Certificate[0]; }
}
