export const domesticChannels=['wechatFriend','wechatMoments','qq','qzone','wecom','dingtalk'] as const;
export const overseasChannels=['whatsapp','telegram','email'] as const;
export function shareText(name:string,url:string){return `${name}\n${url}`}
export function shareChannelURL(channel:string,name:string,url:string){
 const text=shareText(name,url);const p=new URLSearchParams({url,title:name,summary:name});
 if(channel==='qq')return 'https://connect.qq.com/widget/shareqq/index.html?'+p;
 if(channel==='qzone')return 'https://sns.qzone.qq.com/cgi-bin/qzshare/cgi_qzshare_onekey?'+p;
 if(channel==='telegram')return 'https://t.me/share/url?'+new URLSearchParams({url,text:name});
 if(channel==='email')return 'mailto:?'+new URLSearchParams({subject:name,body:text}).toString().replace(/\+/g,'%20');
 return '';
}
export function addressScope(url:string){const host=new URL(url).hostname.toLowerCase();if(['localhost','127.0.0.1','[::1]'].includes(host))return 'localOnly';if(/^(10\.|192\.168\.|172\.(1[6-9]|2\d|3[01])\.|169\.254\.)/.test(host)||/^(\[)?(fc|fd|fe80)/.test(host))return 'lanShare';return ''}
