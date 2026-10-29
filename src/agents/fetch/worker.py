"""Fixed network worker; requests arrive only on its private stdin pipe."""
import json
import sys
from policy import Refused
from transport import get,ContentError
from extract import extract


def perform(request):
    try:
        data,mime,final=get(request['url'],budget=request['timeout'])
        body=extract(data,mime,request['format'],final)
        return {'ok':True,'body':body,'class':'ok','bytes':len(data)}
    except Refused as error:
        reason=str(error)
        return {'ok':False,'body':'error: could not fetch that URL','class':reason if reason in ('url','port','address','dns','connection','protocol','downgrade','timeout') else 'network','bytes':0}
    except ContentError as error:
        return {'ok':False,'body':'error: '+str(error),'class':'content','bytes':0}
    except Exception:
        return {'ok':False,'body':'error: could not fetch that URL','class':'network','bytes':0}


def main():
    try:
        raw=sys.stdin.buffer.read(32769)
        if len(raw)>32768:raise ValueError()
        request=json.loads(raw)
        if not isinstance(request,dict) or set(request)!={'url','format','timeout'} or not isinstance(request['url'],str) or len(request['url'].encode())>4096 or request['format'] not in ('text','markdown') or type(request['timeout']) not in (int,float) or not 0<request['timeout']<=60:raise ValueError()
        result=perform(request)
    except Exception:
        result={'ok':False,'body':'error: could not fetch that URL','class':'worker','bytes':0}
    print(json.dumps(result,ensure_ascii=False,allow_nan=False))


if __name__=='__main__':main()
