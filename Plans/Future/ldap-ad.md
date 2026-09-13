# LDAP / Active Directory — future

**Not in MVP.** MVP registration is manual (`username + person name + pubkey +
optional details`) plus GitHub — see
[identity § registration](../../docs/01-identity.md#registration). This file records
the design so it does not have to be rediscovered.

## One source, not two

AD is an LDAP v3 server, so **one protocol and one code path** cover both;
write it `LDAP/AD`.

Query it by **shelling out to `ldapsearch`** (openldap-clients), as with every
other external tool ([modules § external tools](../../docs/10-modules.md#external-tools)):
no LDAP library to depend on, and TLS, GSSAPI/Kerberos and the whole bind story are the system's
problem, configured once by whoever runs the directory. A source is then a
saved `ldapsearch` invocation plus the attribute map.

It slots into the existing registration table as one more row: fills in the
same fields, re-check compares the stored stable id.

## Only the attribute names differ

| | OpenLDAP | Active Directory |
|---|---|---|
| Login | `uid` | `sAMAccountName`, or `userPrincipalName` (`parf@corp.example`) |
| **Stable id** (what a re-check compares) | `entryUUID` | **`objectGUID`** — AD has no `entryUUID`; binary, escape it in filters |
| Person name | `displayName` | `displayName` |
| Mail | `mail` | `mail` |
| Photo | `jpegPhoto` | `thumbnailPhoto` |
| Public key | `sshPublicKey` (OpenSSH-LPK schema) | **none by default** — schema extension, or the key comes from normal registration |

## AD specifics before writing the invocation

- **Ports**: 389 / 636, plus 3268 / 3269 for the Global Catalog.
- **Bind**: GSSAPI/Kerberos preferred (`ldapsearch -Y GSSAPI`); simple bind
  over cleartext is usually refused by policy — expect LDAPS or StartTLS.
- **Paging**: results are capped at 1000; the paged-results control is required
  for anything larger.
- **Nested groups** need the matching rule `1.2.840.113556.1.4.1941`.
- **No SSH keys** is a non-issue: AD supplies the name and the stable id, the
  public key comes from the normal registration path (or from a schema
  extension where the org already made one, e.g. what
  `sss_ssh_authorizedkeys` reads).
