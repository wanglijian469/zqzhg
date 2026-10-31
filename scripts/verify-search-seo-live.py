"""Read-only production SEO audit. Does not submit URLs or change server settings."""
import html.parser
import json
from pathlib import Path
import socket
import ssl
import urllib.error
import urllib.request
import xml.etree.ElementTree as ET


class Tags(html.parser.HTMLParser):
    def __init__(self):
        super().__init__(); self.result={'title':'','description':'','h1':[],'canonical':'','robots':'','structured_types':[]};self.collect='';self.text=''
    def handle_starttag(self,tag,attrs):
        a=dict(attrs)
        if tag in ['title','h1'] or tag=='script' and a.get('type')=='application/ld+json': self.collect=tag;self.text=''
        if tag=='meta' and a.get('name') in ['description','robots']:self.result[a['name']]=a.get('content','')
        if tag=='link' and a.get('rel')=='canonical':self.result['canonical']=a.get('href','')
    def handle_data(self,data):
        if self.collect:self.text+=data
    def handle_endtag(self,tag):
        if tag!=self.collect:return
        if tag=='h1':self.result['h1'].append(self.text)
        elif tag=='title':self.result['title']=self.text
        elif tag=='script':
            try:self.result['structured_types'].append(json.loads(self.text).get('@type'))
            except (ValueError,AttributeError):pass
        self.collect=''


opener=urllib.request.build_opener(urllib.request.ProxyHandler({}))
base='https://zqzhg.com'
def request(path,ua='ZQZH-readonly-SEO-audit/1.0'):
    try:r=opener.open(urllib.request.Request(base+path,headers={'User-Agent':ua}),timeout=25)
    except urllib.error.HTTPError as e:r=e
    with r:return r.status,dict(r.headers),r.read().decode('utf-8')

receipt={'checked_at':'2026-10-31','canonical_origin':base,'dns':{},'tls':{},'pages':{},'crawler_probes':[],'real_submissions':0,'new_submission_module_deployed':False}
class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self,req,fp,code,msg,headers,newurl):return None
direct=urllib.request.build_opener(urllib.request.ProxyHandler({}),NoRedirect())
receipt['redirects']=[]
for origin in ['http://zqzhg.com','http://www.zqzhg.com','https://www.zqzhg.com']:
    target=origin+'/regions?hall=agriculture&region_code=130528'
    try:response=direct.open(urllib.request.Request(target,method='HEAD'),timeout=25)
    except urllib.error.HTTPError as e:response=e
    with response:
        receipt['redirects'].append({'from':target,'status':response.status,'location':response.headers.get('Location'),'passed':response.status==301 and response.headers.get('Location')==base+'/regions?hall=agriculture&region_code=130528'})
for host in ['zqzhg.com','www.zqzhg.com']:
    receipt['dns'][host]=sorted({a[4][0] for a in socket.getaddrinfo(host,443,type=socket.SOCK_STREAM)})
    with socket.create_connection((host,443),timeout=10) as connection:
        with ssl.create_default_context().wrap_socket(connection,server_hostname=host) as s:
            cert=s.getpeercert();receipt['tls'][host]={'valid':True,'domains':[v for k,v in cert['subjectAltName'] if k=='DNS'],'expires':cert['notAfter']}
status,_,robots=request('/robots.txt');receipt['robots']={'status':status,'body':robots}
status,_,sitemap=request('/sitemap.xml');receipt['sitemap_index']={'status':status,'shards':[]}
ns={'s':'http://www.sitemaps.org/schemas/sitemap/0.9'}
sample=[]
for node in ET.fromstring(sitemap).findall('s:sitemap/s:loc',ns):
    url=node.text;status,_,text=request(url.removeprefix(base));entries=[n.text for n in ET.fromstring(text).findall('s:url/s:loc',ns)]
    receipt['sitemap_index']['shards'].append({'url':url,'status':status,'urls':len(entries)})
    if ('vendors-' in url or 'products-' in url) and entries:sample.append(entries[0].removeprefix(base))
for path in ['/','/?hall=agriculture','/?hall=cable','/login','/products/999999999']+sample:
    status,headers,text=request(path);p=Tags();p.feed(text);receipt['pages'][path]={'status':status,'x_robots_tag':headers.get('X-Robots-Tag'),**p.result}
for agent in ['Baiduspider/2.0','bingbot/2.0','Googlebot/2.1']:
    for path in ['/','/robots.txt','/sitemap.xml']:
        status,_,_=request(path,agent);receipt['crawler_probes'].append({'agent':agent,'path':path,'status':status})
target=Path(__file__).resolve().parents[1]/'docs/test-output/search-seo-live-audit-20261031.json'
target.write_bytes((json.dumps(receipt,ensure_ascii=False,indent=2)+'\n').encode())
print(json.dumps({'dns':receipt['dns'],'sample_page_statuses':{k:v['status'] for k,v in receipt['pages'].items()},'crawler_probes_passed':all(x['status']==200 for x in receipt['crawler_probes']),'real_submissions':0}))
