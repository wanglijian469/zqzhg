"""Export the explicitly selected local PUBLIC catalogue, never users or drafts."""
import argparse, hashlib, json, mimetypes, time, urllib.error, urllib.request
from pathlib import Path
from urllib.parse import urlparse

def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--source',default='http://127.0.0.1:8080')
    parser.add_argument('--output',required=True)
    args=parser.parse_args()
    source=args.source.rstrip('/')
    parsed=urlparse(source)
    if parsed.scheme!='http' or parsed.hostname not in ('127.0.0.1','localhost') or parsed.username or parsed.password:
        raise SystemExit('Source must be the local public application')
    root=Path(args.output).resolve()
    root.mkdir(parents=True,exist_ok=False)
    (root/'media').mkdir()
    opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
    def get(path):
        for attempt in range(4):
            try:
                return opener.open(urllib.request.Request(source+path,headers={'User-Agent':'ZQZHG-Public-Demo-Migration/1.0'}),timeout=30)
            except urllib.error.HTTPError as error:
                if error.code!=429 or attempt==3:raise
                time.sleep(min(60,max(1,int(error.headers.get('Retry-After','10')))))
    def listing(kind):
        rows=[]
        for page in range(1,201):
            with get('/api/v1/public/'+kind+'?page_size=100&page='+str(page)) as response: data=json.load(response)
            rows.extend(data.get('items',[]))
            if len(rows)>=data.get('total',len(rows)):return rows
        raise RuntimeError('catalogue exceeds bounded export size')
    def public_snapshot(row):
        return {key:row[key] for key in ('id','kind','hall_id','vendor_id','visible','published_revision_id','showroom_slug','data','status') if key in row}
    vendors,products=([public_snapshot(row) for row in listing('vendors')],[public_snapshot(row) for row in listing('products')])
    with get('/api/v1/public/taxonomy?hall_id=1') as response: taxonomy=json.load(response)
    wanted=set()
    def media_refs(value,key=''):
        if isinstance(value,dict):
            for k,v in value.items():media_refs(v,k)
        elif isinstance(value,list):
            if key=='media_ids':wanted.update(int(v) for v in value if isinstance(v,int) and v>0)
            else:
                for v in value:media_refs(v,key)
        elif key in ('media_id','logo_media_id','cover_media_id') and isinstance(value,int) and value>0:wanted.add(value)
    for row in vendors+products:media_refs(row['data'])
    media={}
    for media_id in sorted(wanted):
        with get('/api/v1/media/'+str(media_id)) as response:
            content_type=response.headers.get('Content-Type','').split(';')[0]
            if content_type not in ('image/jpeg','image/png','image/webp'):raise RuntimeError('non-public-image response refused')
            raw=response.read(10*1024*1024+1)
        if not raw or len(raw)>10*1024*1024:raise RuntimeError('public image exceeds import limit')
        extension={'image/jpeg':'.jpg','image/png':'.png','image/webp':'.webp'}[content_type]
        relative='media/'+str(media_id)+extension
        (root/relative).write_bytes(raw)
        media[str(media_id)]={'path':relative,'sha256':hashlib.sha256(raw).hexdigest()}
        time.sleep(.1)
    bundle={'format':1,'usage':'development-demonstration','source':source,'vendors':vendors,'products':products,'categories':taxonomy.get('categories',[]),'media':media}
    (root/'catalog.json').write_text(json.dumps(bundle,ensure_ascii=False,separators=(',',':')),encoding='utf-8')
    print(json.dumps({'vendors':len(vendors),'products':len(products),'public_images':len(media),'output':str(root)},ensure_ascii=False))

if __name__=='__main__':main()
