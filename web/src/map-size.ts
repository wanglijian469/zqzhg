// Route transitions can briefly collapse a map's container. ECharts requires
// a nonzero viewport to invert its projection matrix.
export function waitForMapSize(element:HTMLElement,signal:AbortSignal):Promise<void>{
 return new Promise((resolve,reject)=>{
  let frame=0;
  const cleanup=()=>{cancelAnimationFrame(frame);signal.removeEventListener('abort',abort)};
  const abort=()=>{cleanup();reject(new DOMException('Aborted','AbortError'))};
  const check=()=>{if(signal.aborted){abort();return}if(element.clientWidth>0&&element.clientHeight>0){cleanup();resolve()}else frame=requestAnimationFrame(check)};
  signal.addEventListener('abort',abort,{once:true});check();
 });
}
