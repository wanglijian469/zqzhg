import type {BusinessAccount} from './procurement-api';

export function storeBusinessSession(result:{token:string;account:Pick<BusinessAccount,'id'|'role'>}){
 sessionStorage.setItem('zqzt.merchant',result.token);
 sessionStorage.setItem('zqzt.merchant.id',String(result.account.id));
 if(result.account.role)sessionStorage.setItem('zqzt.merchant.role',result.account.role);else sessionStorage.removeItem('zqzt.merchant.role');
 window.dispatchEvent(new Event('zqzt.merchant.changed'));
}
