import {mediaURL} from './runtime-config';
import QRCode from 'qrcode';
import type {Item,Lang} from './types';
export function vendorPath(item:{id:number;showroom_slug?:string}) {return '/v/'+(item.showroom_slug||'v-'+item.id)}
export function shareURL(canonical:string) {const url=new URL(canonical,window.location.origin);url.search='';url.hash='';return url.href}
export async function qrDataURL(url:string) {return QRCode.toDataURL(url,{width:400,margin:3,errorCorrectionLevel:'M',color:{dark:'#071b36',light:'#ffffff'}})}
export function download(data:string,name:string) {const a=document.createElement('a');a.href=data;a.download=name;document.body.append(a);a.click();a.remove()}
function loadImage(src:string):Promise<HTMLImageElement> {return new Promise((resolve,reject)=>{const image=new Image();const timer=window.setTimeout(()=>reject(new Error('image_timeout')),15000);image.onload=()=>{clearTimeout(timer);resolve(image)};image.onerror=()=>{clearTimeout(timer);reject(new Error('image_error'))};image.crossOrigin='anonymous';image.src=src})}
export function wrapLines(ctx:CanvasRenderingContext2D,text:string,width:number) {const lines:string[]=[];let line='';for(const char of text){if(char==='\n'||ctx.measureText(line+char).width>width){if(line)lines.push(line);line=char==='\n'?'':char}else line+=char}if(line)lines.push(line);return lines}
export function posterContent(item:Item,products:Item[],lang:Lang){
 const p=item.data.company_profile,english=lang==='en';const text=(zh?:string,en?:string)=>((english&&en)||zh||'').trim();const clean=(v:string)=>v.replace(/<[^>]*>/g,' ').replace(/\s+/g,' ').trim();
 const main=clean(item.data.main_products||'');let intro=clean(text(p?.summary,p?.summary_en)||text(item.data.description,item.data.description_en));
 const normal=(v:string)=>v.replace(/[\s，,。.;；:：]/g,'').toLowerCase();intro=intro.split(/(?<=[。！？.!?])\s*/u).filter(sentence=>normal(sentence.replace(/^(?:主营产品|主要产品|主营|main products)\s*[:：]?\s*/i,''))!==normal(main)).join(english?' ':'');
 const seen=new Set<number>();const selected=products.filter(x=>{if(x.kind!=='product'||x.vendor_id!==item.id||seen.has(x.id))return false;seen.add(x.id);return true}).slice(0,3);
 return {name:text(item.data.name,item.data.name_en),main,intro,region:item.data.region_name,phone:item.data.phone,wechat:item.data.wechat,contact:normal(text(p?.contact_name,p?.contact_name_en))===normal(item.data.phone||'')?'':text(p?.contact_name,p?.contact_name_en),logo:p?.logo_media_id||0,cover:p?.cover_media_id||item.data.media_ids?.[0]||0,products:selected.map(x=>({id:x.id,name:text(x.data.name,x.data.name_en),model:x.data.parameters?.match(/(?:^|\n)\s*(?:型号|model)\s*[:：]\s*([^\n]+)/i)?.[1]?.trim()||'',image:x.data.media_ids?.[0]||0}))};
}
export async function posterDataURL(item:Item,url:string,lang:Lang,products:Item[]=[]) {
 await document.fonts?.ready;const content=posterContent(item,products,lang),en=lang==='en';
 const canvas=document.createElement('canvas');canvas.width=1000;const ctx=canvas.getContext('2d')!;if(!ctx)throw new Error('canvas_unavailable');
 const font=(size:number,bold=false)=>{ctx.font=`${bold?'bold ':''}${size}px "Microsoft YaHei", Arial, sans-serif`};
 const lines=(text:string,size:number,width=872,max=100)=>{font(size);const all=wrapLines(ctx,text,width);if(all.length>max){const last=all[max-1];return [...all.slice(0,max-1),last.slice(0,-1)+'…']}return all};
 const names=lines(content.name,44,872,20),main=lines(content.main,27,872,4),intro=lines(content.intro,25,872,4);
 const contacts=[content.contact,content.phone,content.wechat&&(en?'WeChat: ':'微信：')+content.wechat].filter(Boolean).flatMap(x=>lines(x!,26,872,4));
 const regions=lines(content.region||'',23,872,2);const imageIDs=[content.logo,content.cover,...content.products.map(x=>x.image)].filter(Boolean);
 const pictures=new Map<number,HTMLImageElement>();let imageMissing=false;await Promise.all([...new Set(imageIDs)].map(async id=>{try{pictures.set(id,await loadImage(mediaURL(id)))}catch{imageMissing=true}}));
 const productImageHeight=content.products.some(p=>p.image)?190:0;
 const qr=await loadImage(await qrDataURL(url));canvas.height=118+(content.logo?120:0)+names.length*60+(main.length?8:0)+main.length*39+(intro.length?62+intro.length*35:0)+(content.cover?438:0)+(content.products.length?180+productImageHeight:0)+((regions.length||contacts.length)?24+regions.length*34+contacts.length*38:0)+380;
 ctx.fillStyle='#fff';ctx.fillRect(0,0,1000,canvas.height);ctx.fillStyle='#102e50';ctx.fillRect(0,0,1000,16);let y=76;
 function label(text:string){font(23,true);ctx.fillStyle='#1769c2';ctx.fillText(text,64,y);y+=42}
 function drawText(rows:string[],size:number,step:number,color='#203b58',bold=false){font(size,bold);ctx.fillStyle=color;for(const line of rows){ctx.fillText(line,64,y);y+=step}}
 function picture(id:number,x:number,top:number,w:number,h:number){const img=pictures.get(id);ctx.fillStyle='#f2f6fa';ctx.fillRect(x,top,w,h);if(!img){font(20);ctx.fillStyle='#52667b';ctx.fillText(en?'Image unavailable':'图片暂不可用',x+18,top+h/2);return}const scale=Math.min(w/img.width,h/img.height);ctx.drawImage(img,x+(w-img.width*scale)/2,top+(h-img.height*scale)/2,img.width*scale,img.height*scale)}
 label(en?'ZQZG · COMPANY SHOWROOM':'中企展馆 · 企业品牌展厅');if(content.logo){picture(content.logo,64,y-10,80,80);y+=120}
 drawText(names,44,60,'#102e50',true);if(main.length){y+=8;drawText(main,27,39)}
 if(intro.length){y+=20;label(en?'ABOUT THE COMPANY':'了解这家企业');drawText(intro,25,35,'#465d76')}
 if(content.cover){y+=18;picture(content.cover,64,y,872,390);y+=420}
 if(content.products.length){y+=18;label(en?'MAIN PRODUCTS':'主营产品');const width=(872-32)/3;content.products.forEach((p,i)=>{const x=64+i*(width+16);if(p.image)picture(p.image,x,y,width,productImageHeight);font(23,true);ctx.fillStyle='#203b58';lines(p.name,23,width,2).forEach((line,j)=>ctx.fillText(line,x,y+productImageHeight+34+j*31));font(20);ctx.fillStyle='#52667b';lines(p.model,20,width,1).forEach(line=>ctx.fillText(line,x,y+productImageHeight+104))});y+=120+productImageHeight}
 if(regions.length||contacts.length){y+=24;drawText(regions,23,34,'#52667b');drawText(contacts,26,38,'#203b58',true)}
 const bottom=canvas.height-300;ctx.fillStyle='#e0e7ef';ctx.fillRect(64,bottom-24,872,1);ctx.drawImage(qr,64,bottom,248,248);font(30,true);ctx.fillStyle='#102e50';ctx.fillText(en?'Explore the showroom':'扫码查看企业展厅',344,bottom+90);font(23);ctx.fillStyle='#52667b';const prompt=en?'View products and contact this manufacturer':'查看产品详情，直接联系厂家';lines(prompt,23,590,2).forEach((line,i)=>ctx.fillText(line,344,bottom+136+i*34));
 return {data:canvas.toDataURL('image/png'),imageMissing};
}
