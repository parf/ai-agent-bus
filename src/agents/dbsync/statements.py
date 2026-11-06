"""The only SQL DBSync sends: five SELECT shapes built from checked names and checked cursor values.

There are no bind parameters in a `#db` call, so nothing reaches the text unless it is a plain identifier (quoted
for its dialect) or a cursor value that is a number or a timestamp in a strict form. A value is never escaped to
make it fit: one that does not match is refused. Quotes around a timestamp literal are ours, never the value's.
"""
from datetime import datetime
from decimal import Decimal, InvalidOperation
import re

ENGINES=('mysql','postgres')
IDENT=re.compile(r'[A-Za-z_][A-Za-z0-9_]{0,62}\Z')   # 63: PostgreSQL truncates a longer name, and MySQL allows 64
NUMBER=re.compile(r'-?[0-9]{1,40}(\.[0-9]{1,40})?([eE][+-]?[0-9]{1,3})?\Z')
STAMP=re.compile(r'[0-9]{4}-[0-9]{2}-[0-9]{2}[T ][0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,9})?(Z|[+-][0-9]{2}:[0-9]{2})?\Z')
NUMERIC_TYPES={'tinyint','smallint','mediumint','int','integer','bigint','decimal','numeric','float','double','real','double precision'}
TIMESTAMP_TYPES={'datetime','timestamp','timestamp without time zone','timestamp with time zone'}


class StatementError(ValueError):
    """A name or value that cannot reach a statement; `code` is stable."""
    def __init__(self,code,text):self.code=code;super().__init__(text)


def name(value,what='name'):
    if not isinstance(value,str) or not IDENT.match(value):raise StatementError('invalid_name',f'{what} must be a plain identifier')
    return value


def quote(engine,value):
    name(value)
    return f'`{value}`' if engine=='mysql' else f'"{value}"'


def kind_of(data_type):
    """'numeric', 'timestamp' or None for a database data_type."""
    text=str(data_type).lower().split('(')[0].strip()
    if text.startswith('unsigned '):text=text[9:]
    text=text.replace(' unsigned','')
    if text in NUMERIC_TYPES:return 'numeric'
    if text in TIMESTAMP_TYPES:return 'timestamp'
    return None


def text_of(value):
    """A cursor value as the database returned it, as text; never through a float when it came as text."""
    if isinstance(value,bool) or value is None:raise StatementError('invalid_cursor','a cursor value must be a number or a timestamp')
    if isinstance(value,int):return str(value)
    if isinstance(value,float):return repr(value)
    if isinstance(value,str):return value
    raise StatementError('invalid_cursor','a cursor value must be a number or a timestamp')


def literal(kind,value):
    """The SQL text of one cursor value of a column of `kind`."""
    text=text_of(value)
    if kind=='numeric' and NUMBER.match(text):return text
    if kind=='timestamp' and STAMP.match(text):return "'"+text+"'"
    raise StatementError('invalid_cursor',f'a cursor value is not a {kind}')


def order_key(kind,value):
    """A comparable form of a cursor value, so Python orders pairs the way the database does."""
    text=text_of(value)
    try:
        if kind=='numeric':
            if not NUMBER.match(text):raise ValueError()
            return Decimal(text)
        if kind=='timestamp' and STAMP.match(text):return datetime.fromisoformat(text.replace(' ','T',1))
    except (InvalidOperation,ValueError):pass
    raise StatementError('invalid_cursor',f'a cursor value is not a {kind}')


class Builder:
    """Statements for one table. `kinds` holds the type of the key and `updated` columns once the type guard has run."""

    def __init__(self,engine,table,key,updated,columns=(),schema=None):
        if engine not in ENGINES:raise StatementError('invalid_engine','engine must be mysql or postgres')
        self.engine=engine;self.table=name(table,'table');self.key=name(key,'key');self.updated=name(updated,'updated')
        self.schema=None if schema is None else name(schema,'schema')
        self.columns=[name(c,'column') for c in columns]
        self.kinds={};self.zoned=False   # zoned: the updated column carries a time zone (set by the type guard)

    def _q(self,value):return quote(self.engine,value)

    def _from(self):
        return (self._q(self.schema)+'.' if self.schema else '')+self._q(self.table)

    def selected(self):
        """Column names in answer order: key, updated, then the chosen columns not already there."""
        out=[self.key,self.updated]
        for c in self.columns:
            if c not in out:out.append(c)
        return out

    def page(self,after,limit,since=None):
        """Rows after the pair `after`, or from `updated` value `since`, or from the start; ordered by (updated, key), at most `limit`."""
        if type(limit) is not int or not 1<=limit<=10000:raise StatementError('invalid_limit','a page holds 1 to 10000 rows')
        u,k=self._q(self.updated),self._q(self.key)
        where=f'{u} IS NOT NULL'
        if since is not None:where+=f' AND {u} >= {literal(self.kinds["updated"],since)}'
        if after is not None:
            au,ak=literal(self.kinds['updated'],after[0]),literal(self.kinds['key'],after[1])
            where+=f' AND ({u} > {au} OR ({u} = {au} AND {k} > {ak}))'
        cols=', '.join(self._q(c) for c in self.selected())
        return f'SELECT {cols} FROM {self._from()} WHERE {where} ORDER BY {u}, {k} LIMIT {limit}'

    def cutoff(self,seconds,unit=None):
        """The database's own now() minus `seconds`, in the form of the `updated` column: epoch s or ms, or timestamp text."""
        if type(seconds) is not int or not 0<seconds<=86400:raise StatementError('invalid_lookback','a lookback is 1 second to 1 day')
        if unit in ('s','ms'):
            m=1000 if unit=='ms' else 1
            now='UNIX_TIMESTAMP(NOW(6))' if self.engine=='mysql' else 'EXTRACT(EPOCH FROM CURRENT_TIMESTAMP)'
            return f'SELECT FLOOR({now} * {m}) - {seconds*m}'
        if self.engine=='mysql':return f"SELECT DATE_FORMAT(NOW(6) - INTERVAL {seconds} SECOND, '%Y-%m-%d %H:%i:%s.%f')"
        if self.zoned:return f"SELECT to_char(CURRENT_TIMESTAMP - interval '{seconds} seconds', 'YYYY-MM-DD HH24:MI:SS.USTZH:TZM')"
        return f"SELECT to_char(LOCALTIMESTAMP - interval '{seconds} seconds', 'YYYY-MM-DD HH24:MI:SS.US')"

    def newest(self):
        """The highest (updated, key) pair, where a subscriber that wants only later changes starts."""
        u,k=self._q(self.updated),self._q(self.key)
        return f'SELECT {k}, {u} FROM {self._from()} WHERE {u} IS NOT NULL ORDER BY {u} DESC, {k} DESC LIMIT 1'

    def row(self,key):
        """The one row with this key, for a body that is no longer in memory."""
        cols=', '.join(self._q(c) for c in self.selected())
        return f'SELECT {cols} FROM {self._from()} WHERE {self._q(self.key)} = {literal(self.kinds["key"],key)}'

    def types(self):
        """The data types of the key and `updated` columns, from information_schema; names are pattern-checked literals."""
        schema=f"'{self.schema}'" if self.schema else ('DATABASE()' if self.engine=='mysql' else 'current_schema()')
        wanted=', '.join(f"'{c}'" for c in (self.key,self.updated))
        return ('SELECT column_name, data_type FROM information_schema.columns'
                f" WHERE table_schema = {schema} AND table_name = '{self.table}' AND column_name IN ({wanted})")
