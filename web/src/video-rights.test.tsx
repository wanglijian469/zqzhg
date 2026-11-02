// @vitest-environment jsdom
import {afterEach,expect,it,vi} from 'vitest';
import {render,screen,cleanup,waitFor} from '@testing-library/react';
import {BrowserRouter} from 'react-router-dom';
import {VideoSourceFields} from './video-playback';
import type {ResourceData} from './resources';
const documents=['terms','privacy','showroom-agreement'].map((key,i)=>({key,id:i+1,version:1,title:key,title_en:key,body:'示例协议',body_en:'Example',digest:key+'-v1',status:'published',effective_at:'2026-10-01'}));
const data:ResourceData={title:'视频',title_en:'',description:'',description_en:'',purpose:'company',format:'video',product_id:0,file_media_id:0,cover_media_id:0,page_media_ids:[],video_source:'DOUYIN'};
afterEach(()=>{cleanup();vi.unstubAllGlobals();sessionStorage.clear()});
for(const accepted of [true,false])it(`reuses current agreements without per-video rights confirmation (${accepted})`,async()=>{
 sessionStorage.setItem('zqzt.merchant','test');
 vi.stubGlobal('fetch',vi.fn(async()=>new Response(JSON.stringify({documents,acceptances:accepted?documents.map(d=>({document_id:d.id,digest:d.digest})):[]}))));
 render(<BrowserRouter><VideoSourceFields data={data} onChange={()=>{}} onCheck={()=>{}} role="merchant" disabled={false}/></BrowserRouter>);
 await waitFor(()=>expect(fetch).toHaveBeenCalled());await waitFor(()=>expect(document.querySelector('.loading')).toBeNull());
 expect(screen.queryByText('我确认该视频由本人发布，或已取得合法展示授权。')).toBeNull();
 expect(screen.queryAllByRole('checkbox')).toHaveLength(accepted?0:1);
 if(accepted)expect(screen.queryByText('服务协议确认')).toBeNull();else expect(screen.getByText('服务协议确认')).toBeTruthy();
});
