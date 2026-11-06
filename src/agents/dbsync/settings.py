"""DBSync's Configuration: an exact allowlist, strict types and ranges. It names what to read, never how to log in."""
import math
import re

import statements

KEYS={'db','engine','schema','table','key','updated','updated_unit','columns','deleted','deleted_value','every','max_rows','batch',
      'window_max','park_after','state_max'}
REQUIRED=('db','engine','table','key','updated')
DURATION=re.compile(r'([0-9]{1,7})(s|m|h)\Z')
SIZE=re.compile(r'([0-9]{1,13})(B|KiB|MiB|GiB)?\Z')
UNIT={'B':1,'KiB':1024,'MiB':1024**2,'GiB':1024**3}
AGENT=re.compile(r'#[a-z0-9][a-z0-9._-]{0,63}(/[a-z0-9][a-z0-9._-]{0,63})?(@[a-z0-9][a-z0-9.-]{0,252})?\Z')


class SettingsError(ValueError):
    def __init__(self,key,why):self.key=key;super().__init__(f'{key}: {why}')


def duration(key,value,low,high):
    if type(value) in (int,float):seconds=value
    elif isinstance(value,str) and DURATION.match(value):
        n,unit=DURATION.match(value).groups();seconds=int(n)*{'s':1,'m':60,'h':3600}[unit]
    else:raise SettingsError(key,'a duration such as 30s, 5m or 1h')
    if not math.isfinite(seconds) or not low<=seconds<=high:raise SettingsError(key,f'between {low} and {high} seconds')
    return float(seconds)


def size(key,value,low,high):
    if type(value) is int:n=value
    elif isinstance(value,str) and SIZE.match(value):
        digits,unit=SIZE.match(value).groups();n=int(digits)*UNIT[unit or 'B']
    else:raise SettingsError(key,'a byte count or a size such as 64MiB')
    if not low<=n<=high:raise SettingsError(key,f'between {low} and {high} bytes')
    return n


def count(key,value,low,high):
    if type(value) is not int or not low<=value<=high:raise SettingsError(key,f'a whole number from {low} to {high}')
    return value


def identifier(key,value):
    try:return statements.name(value,key)
    except statements.StatementError:raise SettingsError(key,'a plain identifier: a letter or underscore, then letters, digits or underscores, at most 64') from None


def parse(config):
    """The settings from the Configuration; an unknown or missing key is refused by name."""
    if not isinstance(config,dict):raise SettingsError('configuration','a JSON object naming db, engine, table, key and updated')
    unknown=sorted(set(config)-KEYS)
    if unknown:raise SettingsError(unknown[0],'unknown setting')
    for key in REQUIRED:
        if key not in config:raise SettingsError(key,'required')
    db=config['db']
    if not isinstance(db,str) or not AGENT.match(db.lower()):raise SettingsError('db','the DB agent, such as #db/orders@node.example')
    if config['engine'] not in statements.ENGINES:raise SettingsError('engine','mysql or postgres')
    columns=config.get('columns',[])
    if not isinstance(columns,list) or len(columns)>64:raise SettingsError('columns','a list of at most 64 column names')
    unit=config.get('updated_unit')
    if unit not in (None,'s','ms'):raise SettingsError('updated_unit','s or ms, for an updated column that holds epoch time')
    deleted=None if config.get('deleted') is None else identifier('deleted',config['deleted'])
    marker=config.get('deleted_value')
    if deleted is None and 'deleted_value' in config:raise SettingsError('deleted_value','only with deleted, the soft-delete column')
    if deleted is not None and (type(marker) not in (str,int,bool) or isinstance(marker,str) and len(marker)>200):
        raise SettingsError('deleted_value','the value that marks a row deleted: text, a whole number, true or false')
    s={'db':db.lower(),'deleted':deleted,'deleted_value':marker,'engine':config['engine'],'schema':None if config.get('schema') is None else identifier('schema',config['schema']),
       'table':identifier('table',config['table']),'key':identifier('key',config['key']),'updated':identifier('updated',config['updated']),
       'updated_unit':unit,'columns':[identifier('columns',c) for c in columns],
       'every':duration('every',config.get('every','60s'),5,86400),
       'max_rows':count('max_rows',config.get('max_rows',1000),1,10000),
       'batch':count('batch',config.get('batch',20),1,500),
       'window_max':count('window_max',config.get('window_max',10000),1,100000),
       'park_after':duration('park_after',config.get('park_after','10m'),1,7*86400),
       'state_max':size('state_max',config.get('state_max','64MiB'),1024*1024,1024**3)}
    return s


def page_size(max_rows):
    """A page is max_rows minus a tenth (rounded up), so the DB agent's cut is never the page's own size; 0 means refuse."""
    return max_rows-math.ceil(max_rows/10)
