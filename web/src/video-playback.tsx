import './douyin-video.css';
import {PublicationAgreements} from './service-center';
import {createPortal} from 'react-dom';
import {useEffect,useLayoutEffect,useRef,useState,useId,type ReactNode} from 'react';
import {useLocation} from 'react-router-dom';
import {api,APIError,body} from './api';
import {useLanguage,Loading,Modal} from './ui';
import {runtimeConfig,useRuntimeConfig} from './runtime-config';
import type {ResourceData,ResourceItem} from './resources';

export type VideoSource='LOCAL'|'DOUYIN'|'KUAISHOU';
export const videoAPIVersion=(config=runtimeConfig())=>Number(config.capabilities?.video_api_version)>=3?'3':'2';
export const videoV2=(path:string)=>path+(path.includes('?')?'&':'?')+'video_api_version='+videoAPIVersion();
export const isExternalVideo=(source?:VideoSource)=>source==='DOUYIN'||source==='KUAISHOU';
export interface VideoCheck {source:string;video_id:string;canonical_url:string;player_url?:string;title:string;width:number;height:number;status:string;reason:string;expires_at:string}
export function trustedPlayer(raw:string,id?:string){try{const u=new URL(raw);return u.protocol==='https:'&&u.hostname==='open.douyin.com'&&!u.port&&!u.username&&!u.password&&u.pathname==='/player/video'&&u.searchParams.get('autoplay')==='0'&&(!id||u.searchParams.get('vid')===id)?u.href:''}catch{return ''}}
const sessions=new Set<()=>void>();
export function stopAllVideoSessions(){const previous=[...sessions];sessions.clear();previous.forEach(stop=>stop())}
export function VideoNavigation(){const location=useLocation();useLayoutEffect(()=>stopAllVideoSessions(),[location.key]);return null}
export function usePlaybackSession(stop:()=>void,enabled=true){const ref=useRef(stop);ref.current=stop;useEffect(()=>{if(!enabled)return;const end=()=>ref.current();stopAllVideoSessions();sessions.add(end);const hidden=()=>{if(document.hidden)stopAllVideoSessions()};document.addEventListener('visibilitychange',hidden);window.addEventListener('pagehide',stopAllVideoSessions);return()=>{sessions.delete(end);document.removeEventListener('visibilitychange',hidden);window.removeEventListener('pagehide',stopAllVideoSessions)}},[enabled])}
export function originalVideoURL(raw:string){try{const u=new URL(raw);return u.protocol==='https:'&&!u.username&&!u.password&&!u.port&&['www.douyin.com','v.douyin.com'].includes(u.hostname)?u.href:''}catch{return ''}}
export function externalVideoURL(raw:string,source?:VideoSource){
 if(source!=='KUAISHOU')return originalVideoURL(raw);
 try{
  const u=new URL(raw);
  if(u.protocol!=='https:'||u.username||u.password||u.port||u.search||u.hash)return '';
  const path=u.hostname==='v.kuaishou.com'?/^\/[A-Za-z0-9]{3,48}$/:u.hostname==='www.kuaishou.com'?/^\/short-video\/[A-Za-z0-9]{3,48}$/:null;
  return path?.test(u.pathname)?u.href:'';
 }catch{return ''}
}
export function douyinHostDocument(raw:string){const src=trustedPlayer(raw);if(!src)return '';const escaped=src.replace(/&/g,'&amp;').replace(/"/g,'&quot;');return `<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; frame-src https://open.douyin.com; style-src 'unsafe-inline'; object-src 'none'; base-uri 'none'"><style>html,body{margin:0;width:100%;height:100%;background:#000}iframe{display:block;width:100%;height:100%;border:0}</style></head><body><iframe src="${escaped}" title="抖音视频" sandbox="allow-scripts allow-same-origin allow-presentation" allow="fullscreen; encrypted-media; picture-in-picture" referrerpolicy="origin" allowfullscreen></iframe></body></html>`}
// All video entry points share the existing accessible dialog and explicit retry lifecycle.
export function VideoModal({close,originalURL='',source,children}:{close:()=>void;originalURL?:string;source?:VideoSource;children:(headingID:string)=>ReactNode}){
 const headingID=useId(),{lang}=useLanguage();const [attempt,setAttempt]=useState(0);const original=externalVideoURL(originalURL,source);const kuaishou=source==='KUAISHOU';
 return createPortal(<div onKeyDownCapture={event=>{if(event.key==='Escape'&&document.fullscreenElement){event.preventDefault();event.stopPropagation();if(typeof document.exitFullscreen==='function')void document.exitFullscreen().catch(()=>{})}}}><Modal close={close} labelledBy={headingID} className="video-modal"><div className="video-modal-actions">{!kuaishou&&<button type="button" className="text-link" onClick={()=>setAttempt(n=>n+1)}>{lang==='zh'?'重试播放':'Retry playback'}</button>}{original&&<a className="text-link" href={original} target="_blank" rel="noopener noreferrer">{lang==='zh'?(kuaishou?'前往快手观看':source==='DOUYIN'?'前往抖音观看':'打开原视频'):(kuaishou?'Watch on Kuaishou':source==='DOUYIN'?'Watch on Douyin':'Open original video')}</a>}</div><div key={attempt}>{children(headingID)}</div></Modal></div>,document.body);
}
export function OfficialVideo({item,role,check}:{item?:ResourceItem;role?:'merchant'|'admin';check?:VideoCheck}){
 const {lang}=useLanguage(),zh=lang==='zh';const [result,setResult]=useState(check),[error,setError]=useState(''),[active,setActive]=useState(true);const host=useRef<HTMLDivElement>(null);const [size,setSize]=useState({width:800,height:window.innerHeight*.65});usePlaybackSession(()=>setActive(false));
 useEffect(()=>{const el=host.current;if(!el)return;const measure=()=>setSize({width:el.clientWidth||800,height:window.innerHeight*.65});measure();const ro=typeof ResizeObserver==='undefined'?undefined:new ResizeObserver(measure);ro?.observe(el);window.addEventListener('resize',measure);return()=>{ro?.disconnect();window.removeEventListener('resize',measure)}},[result?.status]);
 useEffect(()=>{if(check){setResult(check);return}if(!item?.id)return;const abort=new AbortController();void api<VideoCheck>(videoV2('/'+(role||'public')+'/resources/'+item.id+'/playback'),{signal:abort.signal},role).then(setResult).catch(e=>{if(!abort.signal.aborted)setError((e as APIError).code)});return()=>abort.abort()},[item?.id,role,check]);
 const src=result?.player_url?trustedPlayer(result.player_url,item?.data.external_video_id||result.video_id):'';
 if(!active)return <p>{zh?'播放已停止，请关闭后重新打开。':'Playback stopped. Close and reopen to play.'}</p>;
 if(error||result&&(!src||result.status!=='READY'))return <p role="alert">{videoMessage(error||result?.reason||'invalid_official_player',zh)}</p>;
 if(!result)return <Loading/>;
 // The official player changes to a fixed portrait UI below 730 CSS pixels.
 // Scale its entire viewport, including controls and branding, rather than clipping it.
 const wide=result.width>=result.height,logicalWidth=wide?800:324,logicalHeight=wide?485:672;
 const scale=size.width?Math.min(1,size.width/logicalWidth,size.height/logicalHeight):0;
 return <><p className="notice">{zh?'请点击官方播放器内的播放按钮开始播放。':'Click the play button inside the official player to start playback.'}</p><div ref={host} className="official-video-host"><div className="official-video" style={{width:logicalWidth*scale,height:logicalHeight*scale}}>{scale>0&&<iframe width={logicalWidth} height={logicalHeight} style={{width:logicalWidth,height:logicalHeight,transform:`scale(${scale})`,transformOrigin:'0 0'}} title={result.title||'抖音视频'} srcDoc={douyinHostDocument(src)} sandbox="allow-scripts allow-same-origin allow-presentation" allow="fullscreen; encrypted-media; picture-in-picture" referrerPolicy="origin" allowFullScreen/>}</div></div></>;
}
export function videoMessage(code:string,zh=true){const messages:Record<string,[string,string]>={unsupported_video_content:['这是抖音图文链接，当前视频播放器暂不支持。请复制视频作品的分享链接。','Photo posts are not supported by this video player. Copy a video post link.'],video_redirect_limit:['分享链接跳转次数过多，请使用完整视频链接。','Too many redirects. Use the full video link.'],video_id_not_found:['暂未识别视频，请稍后重新检查或使用完整视频链接。','The video ID is not available yet. Retry or use the full link.'],video_link_unreachable:['分享短链接暂时无法访问，请稍后重试。','The short link is temporarily unavailable.'],video_network_timeout:['连接抖音超时，请稍后重新检查。','Connection to Douyin timed out. Please retry.'],video_not_public:['该视频目前无法公开播放。请确认公开视频状态，或上传本地视频。','This video is not publicly playable.'],video_provider_unavailable:['抖音服务暂时无法连接，请稍后重新检查。','Douyin is temporarily unavailable.'],video_check_required:['请先检查链接，确认可以预览后再提交审核。','Check the link before submitting.'],invalid_video_link:['请输入有效的视频分享链接。','Enter a valid video share link.'],one_video_link_required:['请每次粘贴一个视频链接。','Paste exactly one video link.'],invalid_official_player:['官方未返回可用播放器，请稍后重试。','The official player is unavailable.'],duplicate_video:['这个视频已添加过，请到视频列表修改。','This video has already been added.'],video_rights_required:['请确认您有权展示该视频。','Confirm your right to display the video.'],douyin_video_disabled:['抖音视频暂未开放，请上传本地视频。','Douyin is disabled. Upload a local video.'],video_client_upgrade_required:['请使用新版网站或 App 管理此视频。','Please upgrade your app.'],video_short_link_unreachable:['分享短链接暂时无法访问，请使用完整视频链接重试。','The short link cannot be reached.']};return messages[code]?.[zh?0:1]||(zh?'暂时无法播放，请重新检查链接后重试。':'Cannot play or save. Check the link and retry.');}
export const canPreviewVideo=(check?:VideoCheck)=>check?.status==='READY'||check?.status==='EXTERNAL_ONLY';

export function VideoSourceFields({data,onChange,onCheck,role,disabled}:{data:ResourceData;onChange:(d:ResourceData)=>void;onCheck:(c?:VideoCheck)=>void;role:'merchant'|'admin';disabled:boolean}){
 const {lang}=useLanguage(),zh=lang==='zh',config=useRuntimeConfig();
 const kuaishou=data.video_source==='KUAISHOU',enabled=kuaishou?videoAPIVersion(config)==='3':config.capabilities?.douyin_video===true;
 const platform=kuaishou?(zh?'快手':'Kuaishou'):(zh?'抖音':'Douyin');
 const [checking,setChecking]=useState(false),[check,setCheck]=useState<VideoCheck>(),[error,setError]=useState(''),[preview,setPreview]=useState(false);
 const generation=useRef(0),current=useRef(data);current.current=data;
 useEffect(()=>()=>{generation.current++},[]);
 const change=(value:string)=>{
  generation.current++;setChecking(false);setPreview(false);setCheck(undefined);onCheck(undefined);setError('');
  onChange({...data,original_share_url:value,external_video_id:'',canonical_video_url:''});
 };
 const inspect=async()=>{
  const g=++generation.current;setChecking(true);setError('');
  try{
   const r=await api<{original_share_url:string;check:VideoCheck}>(videoV2('/'+role+'/video-links/check'),{method:'POST',body:body({text:data.original_share_url,source:data.video_source})},role);
   if(g!==generation.current)return;
   setCheck(r.check);onCheck(r.check);onChange({...current.current,external_video_id:r.check.video_id,canonical_video_url:r.check.canonical_url,title:current.current.title||r.check.title});
  }catch(e){if(g===generation.current)setError((e as APIError).code)}
  finally{if(g===generation.current)setChecking(false)}
 };
 return <div className="video-source-fields">
  <p>{zh?'视频来源':'Source'} <strong>{platform}</strong></p>
  <label>{zh?'视频分享链接':platform+' share link'}<textarea maxLength={8000} value={data.original_share_url||''} disabled={disabled||!enabled} onChange={e=>change(e.target.value)} placeholder={zh?'粘贴从'+platform+'复制的分享链接或分享内容':''}/></label>
  <p className="muted">{kuaishou?(zh?'链接检查仅验证格式。审核时会确认作品可公开打开。':'Link check validates the format. Review confirms public access.'):(zh?'请使用抖音公开视频的分享链接。':'Use a share link for a public Douyin video.')}</p>
  <div className="form-actions">
   <button type="button" className="button secondary" disabled={disabled||checking||!enabled||!data.original_share_url?.trim()} onClick={()=>void inspect()}>{checking?(zh?'正在检查…':'Checking…'):(zh?'检查链接':'Check link')}</button>
   {!kuaishou&&<button type="button" className="button secondary" disabled={!canPreviewVideo(check)||disabled} onClick={()=>setPreview(true)}>{zh?'预览播放':'Preview playback'}</button>}
  </div>
  {check&&<p role="status" className="notice">{kuaishou?(zh?'链接格式正确，访客可前往快手观看。':'The link format is valid. Visitors can watch on Kuaishou.'):check.status==='READY'?(zh?'官方播放器可预览，请点击预览确认播放效果。':'Official preview available.'):videoMessage(check.reason,zh)}</p>}
  {check&&!canPreviewVideo(check)&&<p className="muted">{zh?'可以先保存草稿；检查通过并保存后，才能提交审核。':'You may save a draft. Check successfully and save before submitting.'}</p>}
  {error&&<p role="alert" className="notice error">{videoMessage(error,zh)}</p>}
  <PublicationAgreements hideAccepted role={role}/>
  {preview&&check&&<VideoModal close={()=>setPreview(false)} source="DOUYIN" originalURL={check.canonical_url}>{headingID=><><h2 id={headingID}>{check.title||(zh?'视频预览':'Video preview')}</h2><OfficialVideo check={check}/></>}</VideoModal>}
 </div>;
}

export function VideoAddChoice({value,onChange,product=false}:{product?:boolean;value?:VideoSource;onChange:(v:VideoSource)=>void}){
 const {lang}=useLanguage(),zh=lang==='zh',config=useRuntimeConfig(),enabled=config.capabilities?.douyin_video===true;
 const sources:VideoSource[]=videoAPIVersion(config)==='3'?['DOUYIN','KUAISHOU','LOCAL']:['DOUYIN','LOCAL'];
 const labels={DOUYIN:zh?(sources.includes('KUAISHOU')?'抖音分享链接':'粘贴视频链接'):'Paste Douyin link',KUAISHOU:zh?'快手分享链接':'Paste Kuaishou link',LOCAL:zh?'上传本地视频':'Upload local video'};
 return <section className="douyin-add-choice">
  <h2>{product?(zh?'添加产品视频':'Add product video'):(zh?'添加企业宣传视频':'Add company video')}</h2>
  <div role="group" aria-label={zh?'选择添加方式':'Choose video source'}>{sources.map(v=><button type="button" aria-pressed={value===v} disabled={v==='DOUYIN'&&!enabled} key={v} onClick={()=>onChange(v)}>{labels[v]}</button>)}</div>
  {!value&&<p className="muted">{zh?'请选择添加方式':'Choose how to add the video'}</p>}
  {!enabled&&<p className="muted">{zh?'抖音链接暂未开放。':'Douyin links are disabled.'}</p>}
 </section>;
}
