"""FileScan's Configuration: strict types and ranges, defaults from the plan."""
import math
import re

KEYS={'dir','include','exclude','mode','every','settle','namespace','table','batch','max_file_size',
      'max_per_scan','baseline','delete_pct','delete_max','state_max','bigfile_ttl'}
DURATION=re.compile(r'(\d+)(s|m|h)\Z')
SIZE=re.compile(r'(\d+)(B|KiB|MiB|GiB)?\Z')
UNIT={'B':1,'KiB':1024,'MiB':1024**2,'GiB':1024**3}
MODES=('poll','watch','both','demand')   # demand: no watch, no timer, no subscriptions; only `sync` sends


class SettingsError(ValueError):
    def __init__(self,key,why):self.key=key;super().__init__(f'{key}: {why}')


def duration(key,value,minimum=0):
    if isinstance(value,bool) or isinstance(value,(int,float)):
        seconds=value
    elif isinstance(value,str) and DURATION.fullmatch(value):
        n,unit=DURATION.fullmatch(value).groups();seconds=int(n)*{'s':1,'m':60,'h':3600}[unit]
    else:raise SettingsError(key,'a duration such as 30s, 5m or 1h')
    if isinstance(seconds,bool) or not isinstance(seconds,(int,float)) or not math.isfinite(seconds) or not minimum<=seconds<=7*86400:
        raise SettingsError(key,f'between {minimum} and 7 days')
    return float(seconds)


def size(key,value,low,high):
    if isinstance(value,int) and not isinstance(value,bool):n=value
    elif isinstance(value,str) and SIZE.fullmatch(value):
        digits,unit=SIZE.fullmatch(value).groups();n=int(digits)*UNIT[unit or 'B']
    else:raise SettingsError(key,'a byte count or a size such as 32KiB')
    if not low<=n<=high:raise SettingsError(key,f'between {low} and {high} bytes')
    return n


def count(key,value,low,high):
    if type(value) is not int or not low<=value<=high:raise SettingsError(key,f'a whole number from {low} to {high}')
    return value


def globs(key,value,default):
    if value is None:return list(default)
    if not isinstance(value,list) or len(value)>64 or not all(isinstance(v,str) and v and len(v)<=256 and '\0' not in v for v in value):
        raise SettingsError(key,'a list of at most 64 glob patterns')
    return list(value)


def parse(config):
    """The agent's settings from its Configuration (null is the defaults), refusing an unknown key by name."""
    if config is None:config={}
    if not isinstance(config,dict):raise SettingsError('configuration','a JSON object')
    unknown=sorted(set(config)-KEYS)
    if unknown:raise SettingsError(unknown[0],'unknown setting')
    mode=config.get('mode','both')
    if mode not in MODES:raise SettingsError('mode','poll, watch, both or demand')
    default_every=300.0 if mode=='poll' else 1800.0
    s={
        'mode':mode,
        'every':duration('every',config['every'],1) if 'every' in config else default_every,
        'settle':duration('settle',config.get('settle',0),0),
        'include':globs('include',config.get('include'),['*']),
        'exclude':globs('exclude',config.get('exclude'),['.*','*/.*']),
        'namespace':config.get('namespace'),'table':config.get('table'),
        'batch':count('batch',config.get('batch',20),1,500),
        'max_file_size':size('max_file_size',config.get('max_file_size','32KiB'),0,64*1024*1024),
        'bigfile_ttl':duration('bigfile_ttl',config.get('bigfile_ttl','1h'),1),
        'max_per_scan':count('max_per_scan',config.get('max_per_scan',200),1,100000),
        'baseline':config.get('baseline',False),
        'delete_pct':count('delete_pct',config.get('delete_pct',20),1,100),
        'delete_max':count('delete_max',config.get('delete_max',500),1,10**7),
        'state_max':size('state_max',config.get('state_max','64MiB'),1024*1024,1024**3),
    }
    for key in ('namespace','table'):
        if s[key] is not None and (not isinstance(s[key],str) or not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]{0,63}',s[key])):
            raise SettingsError(key,'a name of letters, digits, dot, underscore or hyphen')
    if not isinstance(s['baseline'],bool):raise SettingsError('baseline','true or false')
    if 'dir' in config and not isinstance(config['dir'],str):raise SettingsError('dir','a path')
    return s
