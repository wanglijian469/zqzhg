import {useEffect,useId,useState} from 'react';
import {api} from './api';
import {MediaImage,Modal,useLanguage} from './ui';
import {domesticChannels,shareChannelURL,shareText,addressScope,type DomesticChannel,domesticShareGuide} from './share-channels';
import {shareURL,qrDataURL,posterDataURL,download} from './showroom-share';
import type {Item,ShowroomResponse} from './types';

export function DomesticShareShowroom({item,canonical,close,inline=false,shareLabel}:{item:Item;canonical:string;close:()=>void;inline?:boolean;shareLabel?:string}){
 const {lang,t}=useLanguage(),zh=lang==='zh',heading=useId();
 const url=shareURL(canonical),name=zh?item.data.name:item.data.name_en||item.data.name;
 const [channel,setChannel]=useState<DomesticChannel>('wechatFriend');
 const [qr,setQR]=useState(''),[poster,setPoster]=useState(''),[busy,setBusy]=useState(false);
 const [notice,setNotice]=useState(''),[error,setError]=useState(''),[manual,setManual]=useState('');
 useEffect(()=>{setQR('');setPoster('');setNotice('');setError('');setManual('')},[item.id,url,lang]);
 const copy=async(text:string)=>{setManual('');try{await navigator.clipboard.writeText(text);setNotice(t('copied'))}catch{setManual(text);setNotice(t('copyFallback'))}};
 const makeQR=async()=>{setBusy(true);setError('');try{setQR(qr||await qrDataURL(url))}catch{setError(t('posterFailed'))}finally{setBusy(false)}};
 const makePoster=async()=>{setBusy(true);setError('');setNotice('');try{
  const published=await api<ShowroomResponse>('/public/showrooms/'+encodeURIComponent(item.showroom_slug||'v-'+item.id));
  if(published.item.id!==item.id)throw Error('vendor_mismatch');
  const result=await posterDataURL(published.item,shareURL(published.canonical_url),lang,published.representative_products??published.home_products??[]);
  setPoster(result.data);if(result.imageMissing)setNotice(t('posterTextFallback'));
 }catch{setError(t('posterFailed'))}finally{setBusy(false)}};
 const scope=addressScope(url),image=item.data.company_profile?.logo_media_id||item.data.company_profile?.cover_media_id||item.data.media_ids?.[0];
 const content=<div className="compact-share domestic-share">
  <h2 id={heading}>{shareLabel||(zh?'分享展厅':'Share showroom')}</h2>
  <div className="share-company-preview">{!!image&&<MediaImage id={image}/>}<div><strong>{name}</strong><p>{zh?(item.data.company_profile?.summary||item.data.description):(item.data.company_profile?.summary_en||item.data.description_en||item.data.description)}</p></div></div>
  {(scope||new URL(url).protocol!=='https:'||/^\d{1,3}(\.\d{1,3}){3}$/.test(new URL(url).hostname))&&<p className="notice">{t(scope||'testShareAddress')}</p>}
  <label>{t('shareAddress')}<input readOnly value={url} onFocus={e=>e.currentTarget.select()}/></label>
  <div className="share-channel-grid" role="group" aria-label={zh?'国内分享渠道':'Sharing channels'}>{domesticChannels.map(key=>{
   const target=shareChannelURL(key,name,url);
   return target?<a key={key} href={target} target="_blank" rel="noopener noreferrer" aria-current={channel===key?'true':undefined} onClick={()=>setChannel(key)}>{t(key)}</a>:<button type="button" key={key} aria-pressed={channel===key} onClick={()=>setChannel(key)}>{t(key)}</button>;
  })}</div>
  <p className="domestic-share-guide" role="note">{domesticShareGuide(channel,zh)}</p>
  <button className="button share-primary" onClick={()=>void copy(url)}>{zh?'复制展厅地址':'Copy showroom link'}</button>
  <div className="share-auxiliary"><button className="button secondary" disabled={busy} onClick={()=>void makeQR()}>{t('showroomQR')}</button><button className="button secondary" disabled={busy} onClick={()=>void makePoster()}>{t(busy?'generating':'previewPoster')}</button></div>
  {qr&&<a href={qr} download={item.data.name+'-QR.png'}><img className="share-qr" src={qr} alt={t('showroomQR')}/><span>{zh?'保存展厅二维码':'Save QR code'}</span></a>}
  {poster&&<section className="poster-preview"><h3>{t('posterPreview')}</h3><p className="muted">{t('posterReview')}</p><img className="share-poster" src={poster} alt={t('posterPreview')}/><div className="poster-actions"><button className="button" onClick={()=>download(poster,item.data.name+'-poster.png')}>{t('downloadPoster')}</button><button className="button secondary" disabled={busy} onClick={()=>void makePoster()}>{t('posterRegenerate')}</button></div></section>}
  <details className="share-more"><summary>{t('shareMoreHelp')}</summary><p>{shareText(name,url)}</p><button className="button secondary" onClick={()=>void copy(shareText(name,url))}>{t('copyPromotion')}</button></details>
  {(qr||poster)&&<p className="muted">{t('imageSaveHelp')}</p>}
  {notice&&<p className="notice" role="status">{notice}</p>}{manual&&<label>{zh?'手动复制':'Copy manually'}<textarea readOnly value={manual} onFocus={e=>e.currentTarget.select()}/></label>}{error&&<p className="notice error" role="alert">{error}</p>}
 </div>;
 return inline?<section className="workspace-panel">{content}</section>:<Modal close={close} labelledBy={heading}>{content}</Modal>;
}
