import {useSyncExternalStore} from 'react';
const key='zqzt.privacy.analytics';
export function analyticsAllowed(){try{return localStorage.getItem(key)==='yes'}catch{return false}}
export function setAnalyticsConsent(allowed:boolean){try{localStorage.setItem(key,allowed?'yes':'no');if(!allowed)for(const item of Object.keys(localStorage))if(item.startsWith('zqzt.analytics.'))localStorage.removeItem(item)}catch{/* Browsing remains available. */}window.dispatchEvent(new Event('zqzt.privacy.changed'))}
export function useAnalyticsConsent(){return useSyncExternalStore(callback=>{window.addEventListener('zqzt.privacy.changed',callback);window.addEventListener('storage',callback);return()=>{window.removeEventListener('zqzt.privacy.changed',callback);window.removeEventListener('storage',callback)}},analyticsAllowed,()=>false)}
