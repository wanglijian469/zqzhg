// @vitest-environment jsdom
import {useState} from 'react';
import {afterEach,beforeEach,expect,it,vi} from 'vitest';
import {act,cleanup,fireEvent,render,screen,waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter,Link,Route,Routes,useNavigate} from 'react-router-dom';
import {useUnsavedChanges} from './use-unsaved-changes';

let answer:((accepted:boolean)=>void)|undefined;
const confirm=vi.fn(()=>new Promise<boolean>(resolve=>{answer=resolve}));
function Editor(){
 const [value,setValue]=useState(''),navigate=useNavigate();
 useUnsavedChanges(()=>value!=='',confirm);
 return <><input aria-label="Draft" value={value} onChange={e=>setValue(e.target.value)}/><button onClick={()=>navigate('/done')}>Programmatic navigation</button><Link to="/done">Leave editor</Link></>;
}
function mount(){render(<BrowserRouter><Routes><Route path="/start" element={<Link to="/edit">Edit</Link>}/><Route path="/edit" element={<Editor/>}/><Route path="/done" element={<h1>Done</h1>}/></Routes></BrowserRouter>)}
beforeEach(()=>{confirm.mockClear();answer=undefined;history.replaceState({idx:0},'','/start')});
afterEach(()=>{cleanup();answer?.(false)});
async function edit(){mount();await userEvent.click(screen.getByRole('link',{name:'Edit'}));fireEvent.change(screen.getByRole('textbox',{name:'Draft'}),{target:{value:'Unsaved draft'}})}

it('guards programmatic navigation and refresh, then leaves once confirmed',async()=>{
 await edit();const refresh=new Event('beforeunload',{cancelable:true});window.dispatchEvent(refresh);expect(refresh.defaultPrevented).toBe(true);
 await userEvent.click(screen.getByRole('button',{name:'Programmatic navigation'}));expect(confirm).toHaveBeenCalledTimes(1);await act(async()=>answer?.(false));expect(location.pathname).toBe('/edit');
 await userEvent.click(screen.getByRole('button',{name:'Programmatic navigation'}));await act(async()=>answer?.(true));expect(await screen.findByRole('heading',{name:'Done'})).toBeTruthy();
 const after=new Event('beforeunload',{cancelable:true});window.dispatchEvent(after);expect(after.defaultPrevented).toBe(false);
});

it('restores Back before Router can unmount the editor and preserves input on cancellation',async()=>{
 await edit();history.back();await waitFor(()=>expect(confirm).toHaveBeenCalledTimes(1));expect(location.pathname).toBe('/edit');expect(screen.getByRole('textbox',{name:'Draft'})).toHaveProperty('value','Unsaved draft');
 await act(async()=>answer?.(false));history.back();await waitFor(()=>expect(confirm).toHaveBeenCalledTimes(2));await act(async()=>answer?.(true));await waitFor(()=>expect(location.pathname).toBe('/start'));expect(screen.getByRole('link',{name:'Edit'})).toBeTruthy();
});

it('leaves a pristine editor without prompting and cleans up the navigation wrappers',async()=>{
 mount();await userEvent.click(screen.getByRole('link',{name:'Edit'}));await userEvent.click(screen.getByRole('link',{name:'Leave editor'}));await screen.findByRole('heading',{name:'Done'});expect(confirm).not.toHaveBeenCalled();
});
