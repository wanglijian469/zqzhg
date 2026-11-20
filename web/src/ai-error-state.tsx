import {ErrorState} from './ui';
import {aiMessages} from './ai-import-common';
export function AIErrorState({code,retry}:{code:string;retry?:()=>void}){
 const message=aiMessages[code];if(!message&&!code.startsWith('ai_'))return <ErrorState code={code} retry={retry}/>;
 return <div className="notice error" role="alert"><p>{message||'该识别步骤暂未完成，已保存成果保留。请重新加载状态或重试失败项。'}</p>{retry&&<button className="button secondary" onClick={retry}>重新加载</button>}<details><summary>技术详情</summary>{code}</details></div>
}
