import {useState} from 'react';
import {useLanguage} from './ui';
import {externalVideoURL,shareVideoWatchLabel,stopAllVideoSessions,type VideoSource} from './video-playback';
import './video-share.css';

export function ShareVideoTip({source}:{source?:VideoSource}){
 const {lang}=useLanguage(),zh=lang==='zh';
 return <p className="share-video-tip">{source==='WECHAT_CHANNELS'?(zh?'此视频在微信视频号观看。如需使用微信，可复制链接后在微信内打开。':'Watch on WeChat Channels. If WeChat is required, copy the link and open it in WeChat.'):(zh?'此视频在快手观看。':'Watch this video on Kuaishou.')}</p>;
}

export function ShareVideoActions({source,url,onOpen}:{source?:VideoSource;url:string;onOpen?:()=>void}){
 const {lang}=useLanguage(),zh=lang==='zh';const [copied,setCopied]=useState(false),[fallback,setFallback]=useState(false);
 const safe=externalVideoURL(url,source);
 if(!safe)return <p role="alert">{zh?'视频链接无效，请联系企业更新。':'Invalid video link. Please ask the company to update it.'}</p>;
 return <div className="share-video-actions">
  <a className="button secondary" href={safe} target="_blank" rel="noopener noreferrer" onClick={()=>{stopAllVideoSessions();onOpen?.()}}>{shareVideoWatchLabel(source,zh)}</a>
  <button type="button" className="text-link" onClick={async()=>{setCopied(false);setFallback(false);try{await navigator.clipboard.writeText(safe);setCopied(true)}catch{setFallback(true)}}}>{zh?'复制链接':'Copy link'}</button>
  {copied&&<span role="status">{zh?'链接已复制':'Link copied'}</span>}
  {fallback&&<label className="share-video-copy-fallback">{zh?'请手动复制视频链接':'Copy the video link manually'}<input readOnly aria-label={zh?'视频链接':'Video link'} value={safe} onFocus={e=>e.target.select()}/></label>}
 </div>;
}
