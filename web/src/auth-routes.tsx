import {Navigate,useLocation,useNavigate} from 'react-router-dom';
import {MerchantEntryPage,MerchantEntryFooter} from './merchant-entry';
import {useHall} from './halls';
export function authReturnPath(search:string){const p=new URLSearchParams(search).get('next');if(!p||!p.startsWith('/')||p.startsWith('//')||/[\\\u0000-\u0020]/.test(p)||/^\/(?:login|register|join|merchant\/login)(?:\?|$)/.test(p))return '/';return p}
export function MerchantLoginPage(){const navigate=useNavigate(),location=useLocation(),{href}=useHall(),next=authReturnPath(location.search);return <MerchantEntryPage kind="login" onLogin={()=>navigate(next==='/'?href('/'):next,{replace:true})}/>}
export function Merchant(){const navigate=useNavigate(),{href}=useHall();return <MerchantEntryPage kind="register" onLogin={()=>navigate(href('/'),{replace:true})}/>}
export function LegacyAuthRedirect({register=false}:{register?:boolean}){const location=useLocation();return <Navigate to={(register?'/join':'/merchant/login')+location.search} replace/>}
export {MerchantEntryFooter};
