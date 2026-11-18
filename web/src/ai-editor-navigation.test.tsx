// @vitest-environment jsdom
import {afterEach,expect,it,vi} from 'vitest';
import {cleanup,render,screen,fireEvent,waitFor} from '@testing-library/react';
import {BrowserRouter,Link} from 'react-router-dom';
import {useState} from 'react';
import {useUnsavedChanges} from './use-unsaved-changes';
afterEach(cleanup);
it('a submitted/unmounted editor no longer intercepts another editor navigation',async()=>{
 history.replaceState(null,'','/merchant/ai-import/1');const ask=vi.fn(async()=>false);
 function Editor({dirty}:{dirty:boolean}){useUnsavedChanges(()=>dirty,ask);return <span>editor</span>}
 function View(){const [submitted,setSubmitted]=useState(false);return <>{!submitted&&<Editor dirty/>}<Editor dirty={false}/><button onClick={()=>setSubmitted(true)}>Submitted</button><Link to="/merchant/ai-import">History</Link></>}
 render(<BrowserRouter><View/></BrowserRouter>);fireEvent.click(screen.getByText('Submitted'));fireEvent.click(screen.getByText('History'));await waitFor(()=>expect(location.pathname).toBe('/merchant/ai-import'));expect(ask).not.toHaveBeenCalled();
});
