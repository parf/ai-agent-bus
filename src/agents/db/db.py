#!/usr/bin/env python3
"""One bounded SQL statement per runner call against one MySQL, PostgreSQL or ClickHouse database.

`read` is enforced by the database (a read-only transaction or setting), never by reading the SQL.
`write` and `admin` are separated only by accounts: see tier_account.
"""
import base64
import datetime
import decimal
import http.client
import json
import math
import os
from pathlib import Path
import re
import signal
import sys
import urllib.parse

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE / 'support/agents/shared' if (HERE / 'support').exists() else HERE.parent / 'shared'))
if (HERE / 'support/lib').exists():
    sys.path.insert(0, str(HERE / 'support/lib'))
from metadata import dispatch
from provider_http import MAX_BYTES, ProxyError, arm_deadline

ENGINES = ('mysql', 'postgres', 'clickhouse')
TIERS = ('read', 'write', 'admin')
MAX_COLUMNS = 256
MAX_CELL = 1 << 16
CONFIG_KEYS = {'engine', 'host', 'port', 'socket', 'database', 'secure', 'max_rows'}
ENV_NAMES = ('DB_USER', 'DB_PASSWORD') + tuple(f'DB_{t.upper()}_{k}' for t in TIERS for k in ('USER', 'PASSWORD'))


def role_tier(roles):
    """The highest tier the delivered roles reach. Unknown labels, and no role, read.

    `#qdrant` gives a caller with no role write; here it is read (RD134). Do not copy that default.
    """
    held = {role for role in roles if isinstance(role, str)}
    if held & {'owner', 'maintainer', 'admin'}:
        return 'admin'
    return 'write' if 'write' in held else 'read'


def delivered_roles(environ):
    return {key[8:].lower() for key, value in environ.items() if key.startswith('AB_ROLE_') and value == '1'}


def tier_account(environ, tier):
    """(user, password) for the tier. Per-tier accounts are all or nothing: a tier without one is refused,
    never served by a stronger account. One account (DB_USER) serves every tier and only `read` is
    then enforced by the database."""
    tiered = any(environ.get(f'DB_{t.upper()}_USER') for t in TIERS)
    if tiered:
        user = environ.get(f'DB_{tier.upper()}_USER', '')
        if not user:
            raise ProxyError(f'no {tier} account is configured on this instance')
        return user, environ.get(f'DB_{tier.upper()}_PASSWORD', '')
    user = environ.get('DB_USER', '')
    if not user:
        raise ProxyError('DB_USER is not set')
    return user, environ.get('DB_PASSWORD', '')


def secrets_of(environ):
    return sorted({environ[name] for name in ENV_NAMES if environ.get(name)}, key=len, reverse=True)


def redact(text, secrets):
    for secret in secrets:
        text = text.replace(secret, '***')
    return text


def check_config(config):
    if not isinstance(config, dict) or set(config) - CONFIG_KEYS:
        raise ProxyError('Configuration accepts only ' + ', '.join(sorted(CONFIG_KEYS)))
    if config.get('engine') not in ENGINES:
        raise ProxyError('Configuration engine must be mysql, postgres or clickhouse')
    if not config.get('socket') and not isinstance(config.get('host'), str):
        raise ProxyError('Configuration needs host or socket')
    port = config.get('port')
    if port is not None and not (isinstance(port, int) and 0 < port < 65536):
        raise ProxyError('Configuration port must be 1..65535')
    rows = config.get('max_rows', 1000)
    if not isinstance(rows, int) or isinstance(rows, bool) or not 1 <= rows <= 10000:
        raise ProxyError('Configuration max_rows must be 1..10000')
    return rows


def statement_of(body):
    if not isinstance(body, str) or len(body.encode()) > MAX_BYTES:
        raise ProxyError('request exceeds 1048576 bytes or is not text')
    if body.lstrip().startswith('{'):
        value = json.loads(body)
        if not isinstance(value, dict) or value.get('method') != 'query' or set(value) != {'method', 'sql'}:
            raise ProxyError('expected a query object with sql')
        sql = value['sql']
    else:
        sql = body.removeprefix('query ') if body.startswith('query ') else body
    if not isinstance(sql, str) or not sql.strip():
        raise ProxyError('query needs a SQL statement')
    return sql


def cell(value):
    """A deterministic, bounded text or JSON value for any driver type."""
    if value is None or isinstance(value, (bool, int, str)):
        pass
    elif isinstance(value, float):
        value = str(value) if not math.isfinite(value) else value
    elif isinstance(value, decimal.Decimal):
        value = str(value)
    elif isinstance(value, (datetime.datetime, datetime.date, datetime.time)):
        value = value.isoformat()
    elif isinstance(value, datetime.timedelta):
        value = str(value)
    elif isinstance(value, (bytes, bytearray, memoryview)):
        value = 'base64:' + base64.b64encode(bytes(value)[:MAX_CELL]).decode()
    elif isinstance(value, (list, tuple, dict)):
        value = json.dumps(value, ensure_ascii=False, default=str)
    else:
        value = str(value)
    if isinstance(value, str) and len(value) > MAX_CELL:
        value = value[:MAX_CELL]
    return value


class Collect:
    """Rows up to the row cap and the byte cap; remembers that it cut."""

    def __init__(self, cap):
        self.cap, self.rows, self.size, self.cut, self.columns = cap, [], 0, False, []

    def add(self, row):
        if len(self.rows) >= self.cap:
            self.cut = True
            return False
        row = [cell(v) for v in row]
        self.size += len(json.dumps(row, ensure_ascii=False).encode())
        if self.size > MAX_BYTES // 2:
            self.cut = True
            return False
        self.rows.append(row)
        return True

    def result(self, affected=None):
        if not self.columns:
            return {'affected': affected if isinstance(affected, int) and affected >= 0 else 0}
        return {'columns': self.columns, 'rows': self.rows, 'truncated': self.cut}


def run_mysql(config, tier, user, password, sql, cap, seconds):
    import pymysql
    from pymysql.cursors import SSCursor
    args = {'user': user, 'password': password, 'database': config.get('database'),
            'connect_timeout': seconds, 'read_timeout': seconds, 'write_timeout': seconds,
            'autocommit': True, 'cursorclass': SSCursor}
    if config.get('socket'):
        args['unix_socket'] = config['socket']
    else:
        args.update(host=config['host'], port=config.get('port', 3306))
    try:
        connection = pymysql.connect(**args)
    except Exception:
        raise ConnectionError() from None
    try:
        cursor = connection.cursor()
        # Both spellings are tried: MySQL and MariaDB name the same limits differently.
        for setting in (f'max_execution_time={int(seconds * 1000)}', f'max_statement_time={int(seconds)}'):
            try:
                cursor.execute('SET SESSION ' + setting)
                break
            except pymysql.err.Error:
                continue
        if tier == 'read':
            # The session flag survives an implicit commit; the transaction is the second wall.
            for flag in ('transaction_read_only', 'tx_read_only'):
                try:
                    cursor.execute(f'SET SESSION {flag}=ON')
                    break
                except pymysql.err.Error:
                    continue
            else:
                raise ProxyError('this server cannot start a read-only session')
            cursor.execute('START TRANSACTION READ ONLY')
        collect = Collect(cap)
        try:
            cursor.execute(sql)
            if cursor.description:
                collect.columns = [d[0] for d in cursor.description]
                while True:
                    batch = cursor.fetchmany(100)
                    if not batch or not all(collect.add(row) for row in batch):
                        break
            return collect.result(cursor.rowcount)
        finally:
            if tier == 'read':
                try:
                    connection.rollback()
                except Exception:
                    pass
    finally:
        try:
            connection.close()
        except Exception:
            pass


def run_postgres(config, tier, user, password, sql, cap, seconds):
    import psycopg
    options = f'-c statement_timeout={int(seconds * 1000)}'
    if tier == 'read':
        options += ' -c default_transaction_read_only=on'
    args = {'user': user, 'password': password, 'dbname': config.get('database') or user,
            'connect_timeout': max(1, int(seconds)), 'options': options}
    args.update(host=config['socket'] if config.get('socket') else config['host'])
    if not config.get('socket'):
        args['port'] = config.get('port', 5432)
    try:
        connection = psycopg.connect(**args, autocommit=(tier != 'read'))
    except Exception:
        raise ConnectionError() from None
    try:
        if tier == 'read':
            connection.read_only = True
        collect = Collect(cap)
        with connection.cursor() as cursor:
            # prepare=True forces the extended protocol: one statement per call, or the server refuses.
            cursor.execute(sql, prepare=True)
            if cursor.description:
                collect.columns = [d.name for d in cursor.description]
                while True:
                    batch = cursor.fetchmany(100)
                    if not batch or not all(collect.add(row) for row in batch):
                        break
            affected = cursor.rowcount
        if tier != 'read':
            connection.commit()
        return collect.result(affected)
    finally:
        try:
            connection.rollback()
            connection.close()
        except Exception:
            pass


class DatabaseError(ProxyError):
    """The database's own answer; it goes back to the caller."""


def run_clickhouse(config, tier, user, password, sql, cap, seconds):
    settings = {'default_format': 'JSONCompact', 'max_execution_time': int(seconds),
                'max_result_rows': cap + 1, 'result_overflow_mode': 'break'}
    if tier == 'read':
        settings['readonly'] = 1
    if config.get('database'):
        settings['database'] = config['database']
    transport = http.client.HTTPSConnection if config.get('secure') else http.client.HTTPConnection
    connection = transport(config['host'], config.get('port', 8443 if config.get('secure') else 8123), timeout=seconds)
    try:
        connection.request('POST', '/?' + urllib.parse.urlencode(settings), sql.encode(),
                           {'X-ClickHouse-User': user, 'X-ClickHouse-Key': password})
        response = connection.getresponse()
        raw = response.read(MAX_BYTES + 1)
    except (OSError, http.client.HTTPException):
        raise ConnectionError() from None
    finally:
        connection.close()
    if len(raw) > MAX_BYTES:
        raise ProxyError('database answer exceeds 1048576 bytes')
    text = raw.decode('utf-8', errors='replace')
    if response.status == 401 or response.status == 403 and 'Authentication' in text:
        raise ConnectionError()
    if response.status >= 300:
        raise DatabaseError(text.strip())
    collect = Collect(cap)
    if text.strip():
        data = json.loads(text)
        collect.columns = [c['name'] for c in data.get('meta', [])]
        for row in data.get('data', []):
            if not collect.add(row):
                break
    return collect.result(0)


RUNNERS = {'mysql': run_mysql, 'postgres': run_postgres, 'clickhouse': run_clickhouse}


def answer(envelope, config, environ):
    secrets = secrets_of(environ)
    timer = None
    status, tier, engine, sent = 'invalid', '-', '-', 0
    try:
        cap = check_config(config)
        engine = config['engine']
        sql = statement_of(envelope['body'])
        sent = len(sql.encode())
        tier = role_tier(delivered_roles(environ))
        user, password = tier_account(environ, tier)
        timer, remaining = arm_deadline(envelope)
        result = RUNNERS[engine](config, tier, user, password, sql, cap, remaining)
        output = json.dumps(result, ensure_ascii=False, default=str)
        if len(output.encode()) > MAX_BYTES:
            raise ProxyError('database answer exceeds 1048576 bytes')
        status = 'ok'
        return redact(output, secrets), 0
    except TimeoutError:
        status = 'timeout'
        return 'error: database request timed out', 0
    except ProxyError as error:
        status = 'refused'
        return 'error: ' + redact(str(error), secrets), 0
    except ConnectionError:
        status = 'unavailable'
        return 'error: database connection or authentication failed', 0
    except (ValueError, TypeError, KeyError, AttributeError):
        return 'error: invalid request or Configuration', 0
    except ImportError as error:
        status = 'missing driver'
        return 'error: ' + redact(f'the {error.name or "database"} driver is not installed', secrets), 0
    except Exception as error:
        # A driver exception is the database's own answer; its text may name the caller's SQL, never the stderr line.
        status = 'database error'
        return 'error: ' + redact(' '.join(str(error).split()) or type(error).__name__, secrets), 0
    finally:
        if timer is not None:
            signal.setitimer(signal.ITIMER_REAL, 0)
            signal.signal(signal.SIGALRM, timer)
        print(json.dumps({'engine': engine, 'tier': tier, 'request_bytes': sent, 'status': status}), file=sys.stderr)


def main():
    result = dispatch(HERE, os.environ.get('AGENT_BUS_ASK'))
    if result is not None:
        print(result)
        return 0
    try:
        raw = sys.stdin.buffer.read(8 * MAX_BYTES + 1)
        if len(raw) > 8 * MAX_BYTES:
            raise ValueError()
        output, code = answer(json.loads(raw), json.loads(os.environ.get('AGENT_BUS_CONFIG', '{}')), os.environ)
    except (ValueError, TypeError):
        output, code = 'error: invalid message or Configuration', 0
    print(output)
    return code


if __name__ == '__main__':
    sys.exit(main())
