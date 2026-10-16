import fs from 'node:fs';
const ref=name=>({$ref:'#/components/schemas/'+name});
const json=schema=>({'application/json':{schema}});
const response=(schema=ref('Success'),status='200')=>({[status]:{description:'Success',content:json(schema)},'400':{description:'Validation failed',content:json(ref('Error'))},'401':{description:'Authentication required',content:json(ref('Error'))},'403':{description:'Ownership or role denied',content:json(ref('Error'))},'404':{description:'Missing or not publicly available',content:json(ref('Error'))},'409':{description:'Pending, stale or already reviewed',content:json(ref('Error'))},'429':{description:'Rate limited',content:json(ref('Error'))}});
const integer={type:'integer',format:'int64',minimum:0};
const string={type:'string'};
const array=schema=>({type:'array',items:schema});
const object=(properties,required=[])=>({type:'object',properties,required});
const payload=object({name:{...string,maxLength:200},name_en:{...string,maxLength:200},main_products:{...string,maxLength:500},keywords:{...string,maxLength:500},region_id:integer,region_name:string,category_id:integer,description:string,description_en:string,phone:string,wechat:string,address:string,parameters:string,media_ids:array(integer)},['name']);
const view=object({id:integer,kind:{type:'string',enum:['vendor','product']},vendor_id:integer,visible:{type:'boolean'},public_visible:{type:'boolean',description:'Actual public access, including the parent booth visibility gate'},featured:{type:'boolean'},sort_order:{type:'integer'},revision_id:integer,status:{type:'string',enum:['draft','pending','approved','rejected']},review_note:string,data:ref('Payload'),matching_products:array(ref('View'))});
const page=object({items:array(ref('View')),total:integer,page:integer,page_size:integer});
const taxonomy=object({halls:array(ref('Hall')),categories:array(ref('Category')),regions:array(ref('Region'))});
const schemas={Payload:payload,View:view,Page:page,Success:object({ok:{type:'boolean'}}),Error:object({error:object({code:string})}),Account:object({id:integer,username:string,role:{type:'string',enum:['merchant','admin']}}),Session:object({token:string,account:ref('Account')}),Taxonomy:taxonomy,Hall:object({id:integer,name:string,name_en:string,slug:string}),Category:object({id:integer,hall_id:integer,parent_id:integer,name:string,name_en:string,sort_order:{type:'integer'},enabled:{type:'boolean'}},['name']),Region:object({id:integer,name:string,name_en:string,enabled:{type:'boolean'}},['name']),Draft:object({revision_id:integer,data:ref('Payload')},['revision_id','data']),Create:object({kind:{type:'string',enum:['vendor','product']},data:ref('Payload')},['kind','data']),RevisionRequest:object({revision_id:integer},['revision_id']),ReviewRequest:object({decision:{type:'string',enum:['approve','reject']},note:{...string,maxLength:1000}},['decision']),ReviewLog:object({id:integer,content_id:integer,revision_id:integer,actor_id:integer,action:string,note:string,created_at:{type:'string',format:'date-time'}})};
const spec={openapi:'3.0.3',info:{title:'中企展馆 · 大陆农机',version:'1.4.0',description:'Published snapshots only. Draft and pending content and media require ownership or administrator access. Error messages are localized by clients using error.code.'},servers:[{url:'http://localhost:8080/api/v1'}],paths:{},components:{securitySchemes:{bearerAuth:{type:'http',scheme:'bearer',description:'Opaque session token; 30 day expiration; revoked on logout'}},schemas}};
function endpoint(method,path,summary,{role,request,result,status='200',query=[]}={}){
 const params=[];if(path.includes('{id}'))params.push({name:'id',in:'path',required:true,schema:integer});for(const name of query)params.push({name,in:'query',schema:['page','page_size','category_id','region_id','vendor_id'].includes(name)?integer:string});
 const op={summary,tags:[role || 'public'],responses:response(result,status)};
 if(role)op.security=[{bearerAuth:[]}];if(params.length)op.parameters=params;if(request)op.requestBody={required:true,content:json(request)};
 (spec.paths[path]??={})[method]=op;
}
endpoint('get','/health','Database health',{result:object({status:string,sms_provider:string})});
endpoint('post','/auth/sms','Send a 5-minute code (60-second cooldown, 20 requests/IP/hour)',{request:object({phone:string},['phone']),result:object({sent:{type:'boolean'},expires_in:integer,development_code:{type:'string',description:'Only development mode and development provider'}})});
endpoint('post','/auth/phone','Consume single-use code (at most 5 attempts)',{request:object({phone:string,code:string},['phone','code']),result:ref('Session')});
endpoint('post','/auth/admin','Administrator login',{request:object({username:string,password:string},['username','password']),result:ref('Session')});
endpoint('post','/auth/merchant','Named manufacturer account login (10 requests/IP/minute); keeps merchant ownership and permissions',{request:object({username:string,password:string},['username','password']),result:ref('Session')});
endpoint('get','/auth/me','Current account',{role:'authenticated',result:ref('Account')});endpoint('post','/auth/logout','Revoke current session',{role:'authenticated'});
endpoint('get','/public/home','Featured public content only',{result:object({vendors:array(ref('View')),products:array(ref('View')),vendor_count:integer})});
endpoint('get','/public/taxonomy','Enabled categories and regions having public vendors',{result:ref('Taxonomy')});
endpoint('get','/public/vendors','Deduplicated manufacturer search',{result:ref('Page'),query:['q','category_id','region_id','page','page_size']});
endpoint('get','/public/products','Published products of public manufacturers',{result:ref('Page'),query:['q','category_id','vendor_id','page','page_size']});
endpoint('get','/public/vendors/{id}','Published booth with paginated products',{result:object({item:ref('View'),products:ref('Page')}),query:['page','page_size']});
endpoint('get','/public/products/{id}','Published product and manufacturer',{result:object({item:ref('View'),vendor:ref('View')})});
spec.paths['/media/{id}']={get:{summary:'Image or thumbnail; anonymous access only if referenced by a currently public snapshot',parameters:[{name:'id',in:'path',required:true,schema:integer},{name:'thumb',in:'query',schema:{type:'string',enum:['1']}}],responses:{'200':{description:'Re-encoded image; Cache-Control: no-store',content:{'image/jpeg':{schema:{type:'string',format:'binary'}},'image/png':{schema:{type:'string',format:'binary'}}}},'404':{description:'Unavailable or private; authenticated owners and administrators may read it'}}}};
for(const role of ['merchant','admin']){
 endpoint('get',`/${role}/taxonomy`,'All taxonomy for management',{role,result:ref('Taxonomy')});
 endpoint('get',`/${role}/contents`,'Current private draft or submission, paginated',{role,result:ref('Page'),query:['kind','status','page','page_size']});
 endpoint('get',`/${role}/contents/{id}`,'Read private draft with ownership verification',{role,result:ref('View')});
 endpoint('get',`/${role}/contents/{id}/history`,'Last 100 review events',{role,result:array(ref('ReviewLog'))});
 endpoint('post',`/${role}/contents/{id}/offline`,'Immediately hide public content and reject pending submissions',{role});
}
endpoint('post','/merchant/contents','Create booth or product draft; max 1000 products per manufacturer including drafts and offline products. Overflow returns 409 product_limit_reached.',{role:'merchant',request:ref('Create'),result:ref('View'),status:'201'});
endpoint('put','/merchant/contents/{id}/draft','Create new draft with optimistic revision guard; pending revisions are frozen',{role:'merchant',request:ref('Draft'),result:ref('View')});
endpoint('post','/merchant/contents/{id}/submit','Freeze draft for review',{role:'merchant',request:ref('RevisionRequest')});
endpoint('post','/admin/revisions/{id}/review','Approve atomically or reject with a required reason; stale review is refused',{role:'admin',request:ref('ReviewRequest')});
endpoint('put','/admin/contents/{id}/featured','Set featured flag and ordering',{role:'admin',request:object({featured:{type:'boolean'},sort_order:{type:'integer'}})});
for(const [resource,schema] of [['categories','Category'],['regions','Region']]){endpoint('post','/admin/'+resource,'Add taxonomy',{role:'admin',request:ref(schema),result:ref(schema)});endpoint('put',`/admin/${resource}/{id}`,'Edit or disable taxonomy (category cycles rejected)',{role:'admin',request:ref(schema),result:ref(schema)})}
spec.paths['/merchant/media']={post:{summary:'Private upload; JPEG/PNG/WebP up to 10 MB and 36 MP, at most 12 attached images/content',security:[{bearerAuth:[]}],requestBody:{required:true,content:{'multipart/form-data':{schema:object({file:{type:'string',format:'binary'}},['file'])}}},responses:response(object({id:integer,url:string,thumb_url:string}),'201')}};
fs.mkdirSync('docs',{recursive:true});fs.writeFileSync('docs/openapi.json',JSON.stringify(spec,null,2));
