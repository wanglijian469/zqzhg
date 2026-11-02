// @vitest-environment jsdom
import {expect,it,vi} from 'vitest';
import {waitForMapSize} from './map-size';
it('waits for a collapsed route container to regain height',async()=>{
 const el=document.createElement('div');let height=0;
 Object.defineProperties(el,{clientWidth:{get:()=>800},clientHeight:{get:()=>height}});
 const ready=vi.fn();const pending=waitForMapSize(el,new AbortController().signal).then(ready);
 await new Promise(r=>setTimeout(r,35));expect(ready).not.toHaveBeenCalled();
 height=500;await pending;expect(ready).toHaveBeenCalledOnce();
});
it('cancels an unmounted map without initializing its renderer',async()=>{
 const el=document.createElement('div'),controller=new AbortController();
 const result=waitForMapSize(el,controller.signal);controller.abort();
 await expect(result).rejects.toMatchObject({name:'AbortError'});
});
