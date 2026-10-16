import fs from 'node:fs';
const file = 'docs/openapi.json';
const doc = JSON.parse(fs.readFileSync(file, 'utf8'));
doc.info.version = '1.5.16';
const ref = name => ({$ref: `#/components/schemas/${name}`});
const integer = {type:'integer',format:'int64',minimum:1};
const object = (properties,required=Object.keys(properties)) => ({type:'object',properties,required});
const json = schema => ({'application/json':{schema}});
const response = (schema,description='Success') => ({description,content:json(schema)});
const error = description => response(ref('Error'),description);
const body = schema => ({required:true,content:json(schema)});
const array = (items,maxItems) => ({type:'array',items,...(maxItems===undefined?{}:{maxItems})});
const kind = {type:'string',enum:['vendor','product']};
const target = object({kind,content_id:integer});
const favorite = object({kind,content_id:integer,available:{type:'boolean'},
  collected_at:{type:'string',format:'date-time'},item:ref('View')},['kind','content_id','available']);
// Find the existing public view schema instead of introducing a second wire shape.
const viewName = Object.keys(doc.components.schemas).find(name => name==='ContentView') ?? 'View';
if(doc.components.schemas[viewName]) favorite.properties.item=ref(viewName);
else favorite.properties.item={type:'object',description:'Existing public View response: id, kind, data, showroom_slug, vendor summary; published snapshot only.'};
doc.components.schemas.Account.properties.role.enum=['merchant','visitor','admin'];
doc.components.schemas.FavoriteTarget=target;
doc.components.schemas.Favorite=favorite;
doc.components.schemas.FavoriteMutation=object({...target.properties,
  operation_id:{type:'string',pattern:'^[A-Za-z0-9_-]{1,80}$'},favorited:{type:'boolean'}});
const visitorLogin=structuredClone(doc.paths['/auth/merchant']);
visitorLogin.post.summary='Visitor account password login (10 requests/IP/minute)';
visitorLogin.post.description='Accepts visitor accounts only. Returns the existing Session shape. Does not grant manufacturer or administrator permissions.';
doc.paths['/auth/visitor']=visitorLogin;
const registration=JSON.parse(JSON.stringify(doc.paths['/auth/merchant/register']).replaceAll('merchant','visitor').replaceAll('Merchant','Visitor'));
registration.post.summary='Register a visitor account without creating a showroom';
registration.post.description='Up to 5 attempts/IP/minute. Globally unique login_name, 1–100 Unicode characters; password 12–72 UTF-8 bytes, not all whitespace. Forced visitor role. Atomic account/session/audit transaction. No company information required.';
doc.paths['/auth/visitor/register']=registration;
doc.paths['/public/favorites/resolve']={post:{summary:'Resolve up to 100 saved public manufacturer/product references',tags:['public'],
  description:'Read-only. Unknown, private or withdrawn content returns available=false with no item. Withdrawn manufacturers also hide their products.',
  requestBody:body(object({items:array(ref('FavoriteTarget'),100)})),
  responses:{200:response(object({items:array(ref('Favorite'))})),400:error('Invalid type, ID or batch size')}}};
const security=[{bearerAuth:[]}];
doc.paths['/favorites']={get:{summary:'List the current visitor/manufacturer account favorites',security,tags:['favorites'],
  parameters:[{name:'kind',in:'query',schema:kind},...['page','page_size'].map(name=>({name,in:'query',schema:{type:'integer',minimum:1,...(name==='page_size'?{maximum:100,default:20}:{default:1})}}))],
  responses:{200:response(object({items:array(ref('Favorite')),total:{type:'integer'},page:{type:'integer'},page_size:{type:'integer'},counts:object({vendor:{type:'integer'},product:{type:'integer'}})})),
    400:error('Invalid filter'),401:error('Authentication required'),403:error('Visitor or manufacturer role required')}}};
doc.paths['/favorites/sync']={post:{summary:'Apply an ordered batch of favorite mutations with replay protection',security,tags:['favorites'],
  description:'Maximum 100 operations. Each operation_id belongs to the current account and must never be reused for another payload. Repeated IDs are acknowledged without applying again, even after another device changes the item. The server serializes each account; the last newly accepted operation wins. Unknown target references are retained as unavailable, without exposing private existence or payload. A conflicting reuse rolls back the entire batch.',
  requestBody:body(object({operations:array(ref('FavoriteMutation'),100)})),
  responses:{200:response(object({acknowledged:array({type:'string'})})),400:error('Invalid operation or batch size'),401:error('Authentication required'),403:error('Visitor or manufacturer role required'),409:error('operation_conflict: operation ID reused with another payload; batch rolled back')}}};
fs.writeFileSync(file,JSON.stringify(doc,null,2)+'\n');
console.log('Favorites and visitor account API documented: 1.5.16');
