"""Fixed network worker; requests arrive only on its private stdin pipe."""
import json
import base64
import sys
from policy import Refused,Policy,curl_fields,BODY_CURL
from transport import get,curl,ContentError,Uncertain
from extract import extract,WARNING,OUTPUT_MAX


def raw_answer(reply):
    try:text=reply.data.decode('utf-8');encoding='utf-8'
    except UnicodeError:text=base64.b64encode(reply.data).decode();encoding='base64'
    def output():return WARNING+json.dumps({'status':reply.status,'headers':reply.headers,'body':text,'body_encoding':encoding},ensure_ascii=False,separators=(',',':'),allow_nan=False)
    result=output()
    if len(result.encode())>OUTPUT_MAX and encoding=='utf-8':
        text=base64.b64encode(reply.data).decode();encoding='base64';result=output()
    if len(result.encode())>OUTPUT_MAX:raise Uncertain('request was sent; answer exceeds limits; do not retry automatically')
    return result


def perform(request):
    try:
        options={'policy':Policy({'allow':request['allow']})} if 'allow' in request else {}
        if request.get('method')=='curl':
            reply=curl(request['url'],verb=request['verb'],headers=request['headers'],body=base64.b64decode(request['body_base64'],validate=True),budget=request['timeout'],**options)
            return {'ok':True,'body':raw_answer(reply),'class':'ok','bytes':len(reply.data)}
        data,mime,final=get(request['url'],budget=request['timeout'],**options)
        body=extract(data,mime,request['format'],final)
        return {'ok':True,'body':body,'class':'ok','bytes':len(data)}
    except Uncertain as error:
        return {'ok':False,'body':'unknown: '+str(error),'class':'unknown','bytes':0}
    except Refused as error:
        reason=str(error)
        return {'ok':False,'body':'error: could not fetch that URL','class':reason if reason in ('url','port','address','dns','connection','protocol','downgrade','timeout','allowlist') else 'network','bytes':0}
    except ContentError as error:
        return {'ok':False,'body':'error: '+str(error),'class':'content','bytes':0}
    except Exception:
        if request.get('method')=='curl':return {'ok':False,'body':'unknown: request may have been sent; response unavailable; do not retry automatically','class':'unknown','bytes':0}
        return {'ok':False,'body':'error: could not fetch that URL','class':'network','bytes':0}


def main():
    try:
        raw=sys.stdin.buffer.read(2*1024*1024+1)
        if len(raw)>2*1024*1024:raise ValueError()
        request=json.loads(raw)
        if not isinstance(request,dict) or type(request.get('timeout')) not in (int,float) or not 0<request['timeout']<=60:raise ValueError()
        expected={'url','format','timeout'} if request.get('method')!='curl' else {'method','verb','url','headers','body_base64','timeout'}
        if 'allow' in request:Policy({'allow':request['allow']});expected.add('allow')
        if set(request)!=expected:raise ValueError()
        if request.get('method')=='curl':
            normalized=curl_fields({k:v for k,v in request.items() if k not in ('timeout','allow')})
            request.update(normalized)
        elif not isinstance(request.get('url'),str) or not 1<=len(request['url'].encode())<=4096 or request['format'] not in ('text','markdown'):raise ValueError()
        result=perform(request)
    except Exception:
        result={'ok':False,'body':'error: could not fetch that URL','class':'worker','bytes':0}
    print(json.dumps(result,ensure_ascii=False,allow_nan=False))


if __name__=='__main__':main()
