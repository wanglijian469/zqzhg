import fs from 'node:fs';
const file=new URL('../docs/openapi.json',import.meta.url);const doc=JSON.parse(fs.readFileSync(file,'utf8'));
const ref=name=>({$ref:'#/components/schemas/'+name});
const str={type:'string'},id={type:'integer',format:'int64',minimum:1},date={type:'string',format:'date-time'},decimal={type:'string',pattern:'^(0|[1-9][0-9]*)(\\.[0-9]{1,4})?$',description:'Decimal string, never a floating-point JSON number'};
const object=properties=>({type:'object',properties});
Object.assign(doc.components.schemas,{
 ProcurementDemandInput:object({audience:str,vendor_ids:{type:"array",maxItems:20,items:id},version:{type:"integer",minimum:0},hall_id:id,source_product_id:id,title:{...str,maxLength:200},delivery_region:{...str,description:'Existing administrative code, province/city/district'},expected_delivery:str,deadline:date,notes:{...str,maxLength:10000},contact_name:str,items:{type:'array',maxItems:30,items:ref('ProcurementDemandItem')},attachment_ids:{type:'array',maxItems:10,items:id}}),
 ProcurementDemandItem:object({id,category_id:id,name:str,specification:str,quantity:decimal,unit:str}),
 ProcurementAction:object({version:id,audience:{type:'string',enum:['direct','matched','public','designated']},vendor_ids:{type:'array',maxItems:20,items:id},state:{type:'string',enum:['completed','closed','cancelled']},reason:str}),
 ProcurementQuoteInput:object({version:{type:'integer',minimum:0},demand_revision_id:id,submit:{type:'boolean'},tax_mode:{type:'string',enum:['included','excluded','negotiable']},freight_mode:{type:'string',enum:['included','fixed','collect','negotiable']},freight:decimal,valid_until:date,notes:str,items:{type:'array',maxItems:30,items:ref('ProcurementQuoteItem')},attachment_ids:{type:'array',maxItems:10,items:id}}),
 ProcurementQuoteItem:object({item_id:id,status:{type:'string',enum:['quoted','unquoted','unavailable']},unit_price:decimal,minimum_quantity:decimal,delivery_days:{type:'integer',minimum:0,maximum:3650},notes:str}),
 ProcurementPublicSummary:object({id,hall_id:id,title:str,delivery_region:str,deadline:date,state:str,items:{type:'array',items:object({name:str,category_id:id,quantity:decimal,unit:str})}}),
 ProcurementPrivateProject:object({id,hall_id:id,state:str,audience:str,version:id,published_revision_id:id,draft_revision_id:id,revision:object({id,status:str,title:str,delivery_region:str,expected_delivery:str,deadline:date,notes:str}),items:{type:'array',items:ref('ProcurementDemandItem')},attachments:{type:'array',items:ref('ProcurementAttachment')},contact:{...object({name:str,phone:str}),nullable:true,description:'Owner/platform only, or supplier after shortlist. Never present in public summaries.'},owner:{type:'boolean'},participant:{type:'object'},participants:{type:'array',items:{type:'object'},description:'Owner/platform only'},reviews:{type:'array',items:{type:'object'},description:'Owner/platform only'},versions:{type:'array',items:{type:'object'},description:'Owner/platform only'}}),
 ProcurementAttachment:object({id,name:str,content_type:str,size:{type:'integer',format:'int64'},scope:str,scope_id:id,project_id:id}),
 ProcurementBuyerProfile:object({company:{...str,maxLength:200},contact_name:{...str,maxLength:100},region_code:str,version:{type:'integer',minimum:0}}),
 ProcurementReviewInput:object({approved:{type:'boolean'},revision_id:id,reason:{...str,maxLength:1000}}),
 ProcurementMessageInput:object({body:{...str,maxLength:5000},attachment_ids:{type:'array',maxItems:10,items:id}}),
 ProcurementModerationInput:object({version:id,suspended:{type:'boolean'},quote_id:id,vendor_id:id,excluded:{type:'boolean'},reason:{...str,minLength:1,maxLength:1000}}),
 ProcurementParticipantInput:object({version:id,intent:{type:'boolean'},state:{type:'string',enum:['participating','declined']}}),
 ProcurementReadInput:object({ids:{type:'array',items:id},scope:{type:'string',enum:['buyer','supplier'],description:'Optional identity scope; omitted preserves all-message reads'}}),
 ProcurementRestrictionInput:object({supplier:{type:'boolean'},suspended:{type:'boolean'},reason:str}),
 ProcurementPreferenceInput:object({opportunity_mode:{type:'string',enum:['instant','daily','off']}}),
});
const responses={200:{description:'Success. Private projections depend on ownership and participant scope.'},400:{description:'Invalid input or missing Idempotency-Key'},401:{description:'Authentication required'},403:{description:'Capability, verified phone, supplier eligibility or administrative permission required'},404:{description:'Unavailable or outside the caller’s scope'},409:{description:'Stale version, closed project, passed deadline or review required'},429:{description:'Rate limited'},503:{description:'Feature disabled'}};
function add(path,method,summary,schema,publicRead=false,responseSchema){
 const parameters=[];for(const match of path.matchAll(/\{([^}]+)\}/g))parameters.push({name:match[1],in:'path',required:true,schema:id});
 if(!['get','head'].includes(method)&&schema!=='multipart')parameters.push({name:'Idempotency-Key',in:'header',required:true,schema:{type:'string',minLength:8,maxLength:80},description:'Reuse after uncertain network failure; reuse with different input returns 409.'});
 const operation={tags:['procurement'],summary,parameters,security:publicRead?[]:[{bearerAuth:[]}],responses:structuredClone(responses)};
 if(schema)operation.requestBody={required:true,content:schema==='multipart'?{'multipart/form-data':{schema:object({file:{type:'string',format:'binary',description:'JPG/PNG/WebP/PDF/DOCX/XLSX, <=20 MiB; no macros, CAD or archives'}})}}:{'application/json':{schema:ref(schema)}}};
 if(responseSchema)operation.responses['200'].content={'application/json':{schema:ref(responseSchema)}};
 doc.paths[path]||={};doc.paths[path][method]=operation;
}
add('/public/procurements','get','Active approved public summaries; no contact, notes, specification or attachment fields',null,true);
add('/public/procurement-vendors','get','Approved supplier selector',null,true);
for(const [path,method,summary,input,output] of [
 ['/profile','get','Current account buyer profile and capabilities'],['/profile','put','Register/update buyer profile without changing the account role','ProcurementBuyerProfile'],
 ['/projects','get','Buyer projects; scope=supplier returns only the caller’s invitations'],['/projects','post','Create one project; source_product_id starts a product inquiry','ProcurementDemandInput'],
 ['/projects/{id}','get','Authorized project; draft=1 for owner, revision_id for authorized version history',null,'ProcurementPrivateProject'],
 ['/projects/{id}/draft','put','Save a new immutable demand draft and supersede pending review','ProcurementDemandInput'],
 ['/projects/{id}/submit','post','Submit publication; first product inquiry reaches its source immediately, wider scope is reviewed on the same project','ProcurementAction'],
 ['/projects/{id}/expand','post','Review expansion of the SAME published project without invalidating quotes','ProcurementAction'],
 ['/projects/{id}/state','post','Complete, close or cancel while retaining history','ProcurementAction'],
 ['/projects/{id}/invite','post','Invite <=20 suppliers per call; <=100 pushed invitations per project','ProcurementAction'],
 ['/projects/{id}/matches','get','Owner-only deterministic category/keyword/coverage match preview'],
 ['/projects/{id}/join','post','Eligible public supplier joins, or follows an existing invitation','ProcurementAction'],
 ['/projects/{id}/quotes','get','Buyer comparison or only the supplier’s own quote/history; never competitor data'],
 ['/projects/{id}/quotes','post','Save draft or submit versioned partial quote for current demand revision','ProcurementQuoteInput'],
 ['/projects/{id}/quote-withdraw','post','Withdraw own quotation before deadline','ProcurementAction'],
 ['/projects/{id}/participants/{vendor}/state','post','Buyer intent disclosure or supplier participation state','ProcurementParticipantInput'],
 ['/projects/{id}/messages/{vendor}','get','Private buyer–supplier conversation with pagination'],
 ['/projects/{id}/messages/{vendor}','post','Send private conversation message','ProcurementMessageInput'],
 ['/projects/{id}/attachments','post','Upload private file before binding it to a demand, quote or message','multipart'],
 ['/attachments/{id}','get','Authenticated download/preview; never a public object-store URL'],['/attachments/{id}','head','Authenticated attachment headers'],
 ['/notifications','get','Current account notifications, unread count and target_scope'],['/notifications/read','post','Mark own available notifications read; empty ids means all','ProcurementReadInput'],
 ['/preferences','get','Current account opportunity preferences'],['/preferences','put','Update frequency without changing platform restriction','ProcurementPreferenceInput'],
])add('/procurement'+path,method,summary,input,false,output);
for(const [path,method,summary,input] of [
 ['/projects','get','procurement.read: project administration'],['/projects/{id}','get','procurement.read: project and demand versions'],
 ['/reviews','get','procurement.read: review queue'],['/reviews/{id}','post','procurement.review: approve/reject exact pending revision','ProcurementReviewInput'],
 ['/projects/{id}/quotes','get','procurement.quote: quotation comparison and history'],['/projects/{id}/matches','get','procurement.match: match reasons'],['/projects/{id}/invite','post','procurement.match: supplement invitations','ProcurementAction'],
 ['/projects/{id}/moderate','post','procurement.manage: suspend project, exclude supplier or hide quote; reason required','ProcurementModerationInput'],
 ['/accounts/{id}/restrict','post','procurement.manage: buyer/supplier restriction; reason required','ProcurementRestrictionInput'],['/attachments/{id}','get','Scoped attachment access; quote/message files additionally require procurement.quote'],['/stats','get','procurement.stats: lifecycle/quote/invitation/review counts'],
])add('/admin/procurement'+path,method,summary,input);
for(const path of ['/public/procurements','/public/procurement-vendors','/procurement/projects','/procurement/notifications','/admin/procurement/projects','/admin/procurement/reviews'])doc.paths[path].get.parameters.push({name:'page',in:'query',schema:{type:'integer',minimum:1,default:1},description:'20 records per page'});
doc.paths['/public/procurements'].get.parameters.push(...['hall_id','category_id','region','q','state'].map(name=>({name,in:'query',schema:str,description:name==='state'?'open or closing_soon (next 72 hours)':'Optional filter'})));
if(!doc.info.description.includes('Schema21'))doc.info.description+='\nSchema21 introduces procurement with additive buyer identity, reviewed publication/expansion, immutable demand and quotation versions, private attachments and conversations, deterministic supplier matching, scoped notifications and restored-object validation. Prices and quantities are decimal strings. Native App APIs remain compatible. Procurement may be disabled with PROCUREMENT_ENABLED=false.';
for(const path of ['/procurement/projects/{id}/matches','/admin/procurement/projects/{id}/matches'])doc.paths[path].get.parameters.push({name:'revision_id',in:'query',schema:id,description:'Owner/platform-only preview for a specific demand version'});
const profile=doc.components.schemas.CompanyProfile;if(profile?.properties){profile.properties.supply_product_category_ids={type:'array',maxItems:35,items:id};profile.properties.supply_region_codes={type:'array',maxItems:50,items:str}}

doc.paths['/procurement/notifications'].get.parameters.push({name:'scope',in:'query',schema:{type:'string',enum:['buyer','supplier']},description:'Optional identity scope; omitted returns all available account messages'});
const phoneCode=doc.paths['/auth/phone/code']?.post?.requestBody?.content?.['application/json']?.schema;
if(phoneCode?.properties?.purpose)phoneCode.properties.purpose.enum=['register','setup_password','login'];
const smsLogin=structuredClone(doc.paths['/auth/phone/password'].post);smsLogin.summary='SMS code login for an existing verified buyer/supplier account; never creates a vendor or accepts an administrator';smsLogin.requestBody.content['application/json'].schema={type:'object',properties:{phone:phoneCode.properties.phone,code:{type:'string',pattern:'^[0-9]{6}$'}},required:['phone','code']};doc.paths['/auth/phone/login']={post:smsLogin};
if(!doc.info.description.includes('Unified procurement opportunities')&&!doc.info.description.includes('Supplier opportunities use /merchant/opportunities'))doc.info.description+='\nSupplier opportunities use /merchant/opportunities; legacy /buyer/opportunities pages redirect with query and fragment preserved. Procurement notifications accept optional buyer/supplier scope for lists, unread counts and marking read. Phone SMS login uses purpose=login and the existing verification lifecycle. Runtime capabilities include phone_sms_auth, development_sms and procurement_demo; demo_accounts is only populated in explicit isolated demo mode. Production never exposes development codes.';

doc.info.description=doc.info.description.replace('Unified procurement opportunities use /buyer/opportunities; legacy merchant opportunity pages redirect with query and fragment preserved.','Supplier opportunities use /merchant/opportunities; legacy /buyer/opportunities pages redirect with query and fragment preserved. Procurement notifications support optional buyer/supplier scope for lists, unread counts and marking read.');
fs.writeFileSync(file,JSON.stringify(doc,null,2)+'\n');
