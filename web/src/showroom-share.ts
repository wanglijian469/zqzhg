import QRCode from 'qrcode';
import type {Item,Lang} from './types';
export function vendorPath(item:{id:number;showroom_slug?:string}) {return '/v/'+(item.showroom_slug||'v-'+item.id)}
export function shareURL(canonical:string) {const url=new URL(canonical,window.location.origin);url.search='';url.hash='';return url.href}
export async function qrDataURL(url:string) {return QRCode.toDataURL(url,{width:400,margin:3,errorCorrectionLevel:'M',color:{dark:'#071b36',light:'#ffffff'}})}
export function download(data:string,name:string) {const a=document.createElement('a');a.href=data;a.download=name;document.body.append(a);a.click();a.remove()}
function loadImage(src:string):Promise<HTMLImageElement> {return new Promise((resolve,reject)=>{const image=new Image();const timer=window.setTimeout(()=>reject(new Error('image_timeout')),15000);image.onload=()=>{clearTimeout(timer);resolve(image)};image.onerror=()=>{clearTimeout(timer);reject(new Error('image_error'))};image.src=src})}
export function wrapLines(ctx:CanvasRenderingContext2D,text:string,width:number) {const lines:string[]=[];let line='';for(const char of text){if(char==='\n'||ctx.measureText(line+char).width>width){if(line)lines.push(line);line=char==='\n'?'':char}else line+=char}if(line)lines.push(line);return lines}
export async function posterDataURL(item:Item,url:string,lang:Lang) {
 await document.fonts?.ready;
 const canvas=document.createElement('canvas');canvas.width=900;const ctx=canvas.getContext('2d');if(!ctx)throw new Error('canvas_unavailable');
 const name=lang==='en'&&item.data.name_en?item.data.name_en:item.data.name;ctx.font='bold 46px "Microsoft YaHei", sans-serif';const titleLines=wrapLines(ctx,name,772);
 ctx.font='26px "Microsoft YaHei", sans-serif';const leadLines=wrapLines(ctx,item.data.main_products,772);
 const textHeight=titleLines.length*64+leadLines.length*42;canvas.height=1250+textHeight;
 ctx.fillStyle='#ffffff';ctx.fillRect(0,0,canvas.width,canvas.height);ctx.fillStyle='#071b36';ctx.fillRect(0,0,900,22);
 ctx.font='22px "Microsoft YaHei", sans-serif';ctx.fillStyle='#667e9c';ctx.fillText(lang==='en'?'ZQZT · COMPANY SHOWROOM':'中企展馆 · 企业展厅',64,85);
 ctx.font='bold 46px "Microsoft YaHei", sans-serif';ctx.fillStyle='#071b36';let y=160;for(const line of titleLines){ctx.fillText(line,64,y);y+=64}
 ctx.font='26px "Microsoft YaHei", sans-serif';ctx.fillStyle='#3d5878';y+=12;for(const line of leadLines){ctx.fillText(line,64,y);y+=42}
 const cover=item.data.company_profile?.cover_media_id||item.data.media_ids?.[0];let imageMissing=false;y+=28;
 if(cover){try{const image=await loadImage('/api/v1/media/'+cover);ctx.fillStyle='#f4f7fb';ctx.fillRect(64,y,772,390);const scale=Math.min(772/image.width,390/image.height);ctx.drawImage(image,64+(772-image.width*scale)/2,y+(390-image.height*scale)/2,image.width*scale,image.height*scale)}catch{imageMissing=true}}
 if(!cover||imageMissing){ctx.fillStyle='#eef4fb';ctx.fillRect(64,y,772,390);ctx.fillStyle='#071b36';ctx.font='bold 36px "Microsoft YaHei", sans-serif';ctx.fillText(lang==='en'?'COMPANY SHOWROOM':'企业产品展厅',100,y+190)}y+=450;
 ctx.fillStyle='#3d5878';ctx.font='24px "Microsoft YaHei", sans-serif';for(const line of wrapLines(ctx,item.data.region_name,772)){ctx.fillText(line,64,y);y+=36}
 if(item.data.phone){ctx.font='bold 32px "Microsoft YaHei", sans-serif';ctx.fillText(item.data.phone,64,y+50)}
 const qr=await loadImage(await qrDataURL(url));const bottom=canvas.height-355;ctx.drawImage(qr,64,bottom,280,280);ctx.fillStyle='#071b36';ctx.font='bold 26px "Microsoft YaHei", sans-serif';ctx.fillText(lang==='en'?'SCAN TO EXPLORE':'扫码查看企业展厅',380,bottom+90);ctx.font='20px "Microsoft YaHei", sans-serif';ctx.fillStyle='#667e9c';for(const [i,line]of wrapLines(ctx,lang==='en'?'Products and company information':'产品与企业资料 · 联系厂家',450).entries())ctx.fillText(line,380,bottom+135+i*30);
 return {data:canvas.toDataURL('image/png'),imageMissing};
}
