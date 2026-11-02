import {useEffect,useRef,useState,type RefObject} from 'react';

// Mounted only after the user opens a video. A blocked play request never retries itself.
export function LocalVideo({src,poster,width,height,playRequested=false,videoRef,zh=true}:{src:string;poster:string;width?:number;height?:number;playRequested?:boolean;videoRef:RefObject<HTMLVideoElement|null>;zh?:boolean}){
 const [ratio,setRatio]=useState(width&&height?width/height:16/9);
 const [state,setState]=useState<'loading'|'ready'|'blocked'|'error'|'slow'>('loading');
 const generation=useRef(0);
 const start=()=>{
  const video=videoRef.current;if(!video)return;
  const request=++generation.current;setState('loading');
  try{void Promise.resolve(video.play()).catch(error=>{if(request===generation.current)setState(error?.name==='NotAllowedError'?'blocked':'error')})}
  catch(error){if(request===generation.current)setState((error as DOMException)?.name==='NotAllowedError'?'blocked':'error')}
 };
 useEffect(()=>{
  const video=videoRef.current;
  // Cancel the first scheduled start if a development StrictMode mount is discarded.
  const first=window.setTimeout(()=>{if(playRequested)start()},0);
  const slow=window.setTimeout(()=>setState(old=>old==='loading'?'slow':old),15000);
  return()=>{generation.current++;window.clearTimeout(first);window.clearTimeout(slow);video?.pause()};
 },[src,playRequested,videoRef]);
 return <div className="local-video">
  <video ref={videoRef} key={src} controls preload="metadata" playsInline src={src} poster={poster}
   style={{width:`min(100%, ${65*ratio}vh)`,aspectRatio:String(ratio)}}
   onLoadedMetadata={e=>{const video=e.currentTarget;if(video.videoWidth&&video.videoHeight)setRatio(video.videoWidth/video.videoHeight);if(!playRequested)setState('ready')}}
   onPlaying={()=>setState('ready')} onWaiting={()=>setState('loading')} onError={()=>{generation.current++;setState('error')}}/>
  {state==='loading'&&<p role="status">{zh?'视频加载中…':'Loading video…'}</p>}
  {state==='slow'&&<p role="status">{zh?'视频加载较慢，请检查网络，或点击上方“重试播放”。':'Loading is taking longer. Check your connection or retry above.'}</p>}
  {state==='error'&&<p role="alert">{zh?'视频暂时无法加载，请点击上方“重试播放”。':'The video could not load. Retry playback above.'}</p>}
  {state==='blocked'&&<button type="button" className="button" onClick={start}>{zh?'点击开始播放':'Play video'}</button>}
 </div>;
}
