"""Google refresh-token exchange, callable only inside the mail operation deadline."""
import http.client
import json
import ssl
import urllib.parse

from mail import MailError

MAX_TOKEN_BYTES = 65536
TOKEN_HOST = 'oauth2.googleapis.com'
TOKEN_PATH = '/token'
ERRORS = {'invalid_grant', 'invalid_client', 'invalid_request', 'unauthorized_client',
          'unsupported_grant_type', 'invalid_scope', 'temporarily_unavailable'}


def token_connection(timeout):
    context = ssl.create_default_context()
    context.minimum_version = ssl.TLSVersion.TLSv1_2
    return http.client.HTTPSConnection(TOKEN_HOST, timeout=timeout, context=context)


def google_access_token(env, timeout, prefix='SMTP'):
    if prefix not in {'SMTP', 'IMAP'}:
        raise MailError('invalid mail credential prefix')
    names = tuple(prefix + suffix for suffix in ('_CLIENT_ID', '_CLIENT_SECRET', '_REFRESH_TOKEN'))
    if any(not isinstance(env.get(name), str) or not env[name] for name in names):
        raise MailError('XOAUTH2 requires ' + ', '.join(names) + ' in this instance host env')
    if any(len(env[name]) > 16384 or not env[name].isascii() or any(ord(c) < 32 or ord(c) == 127 for c in env[name]) for name in names):
        raise MailError('OAuth credentials must be bounded ASCII strings without control characters')
    payload = urllib.parse.urlencode({'client_id': env[names[0]], 'client_secret': env[names[1]],
                                     'refresh_token': env[names[2]], 'grant_type': 'refresh_token'}).encode()
    connection = token_connection(timeout)
    try:
        connection.request('POST', TOKEN_PATH, payload,
                           {'Content-Type': 'application/x-www-form-urlencoded', 'Accept-Encoding': 'identity'})
        response = connection.getresponse()
        if response.getheader('Content-Encoding', 'identity').lower() != 'identity':
            raise MailError('OAuth response encoding is unsupported')
        declared = response.getheader('Content-Length')
        if declared is not None and (not declared.isdigit() or int(declared) > MAX_TOKEN_BYTES):
            raise MailError('OAuth response exceeds 65536 bytes or has invalid length')
        raw = response.read(MAX_TOKEN_BYTES + 1)
        if len(raw) > MAX_TOKEN_BYTES:
            raise MailError('OAuth response exceeds 65536 bytes')
        if declared is not None and len(raw) != int(declared):
            raise MailError('OAuth response was incomplete')
        if response.getheader('Content-Type', '').split(';')[0].strip().lower() != 'application/json':
            raise MailError('OAuth response is not JSON')
        try:
            value = json.loads(raw)
        except (ValueError, UnicodeError):
            raise MailError('OAuth response is invalid JSON') from None
        if not isinstance(value, dict):
            raise MailError('OAuth response is invalid JSON')
        if response.status != 200:
            kind = value.get('error')
            raise MailError('OAuth refresh refused: ' + (kind if isinstance(kind, str) and kind in ERRORS else 'provider_error'))
        token = value.get('access_token')
        if not isinstance(token, str) or not token or len(token) > 16384 or not token.isascii() or any(ord(c) < 33 or ord(c) == 127 for c in token):
            raise MailError('OAuth access token is missing or invalid')
        if not isinstance(value.get('token_type'), str):
            raise MailError('OAuth response is invalid: token_type must be a string')
        if value['token_type'].lower() != 'bearer':
            raise MailError('OAuth token_type must be Bearer')
        return token
    finally:
        connection.close()
