export const domesticChannels=['wechatFriend','wechatMoments','qq','qzone','wecom','dingtalk'] as const;
export type DomesticChannel=typeof domesticChannels[number];
export function domesticShareGuide(channel:DomesticChannel,zh=true){
 const guides:Record<DomesticChannel,[string,string]>={
  wechatFriend:['复制展厅地址，打开微信，找到客户或群聊，粘贴后发送；也可以保存二维码或海报后发送。','Copy the link, open WeChat, choose a contact or group, then paste and send. You can also send the saved QR code or poster.'],
  wechatMoments:['生成并保存海报，打开微信朋友圈，选择海报并粘贴宣传文案后自行发布。网站不会自动发布朋友圈。','Generate and save a poster, then select it in WeChat Moments and paste the promotional text. Posting is manual.'],
  qq:['在QQ官方分享页完成发送。打不开分享页时，请复制展厅地址，再打开QQ粘贴给客户。','Complete sharing on the official QQ page. If it does not open, copy the link and paste it into QQ.'],
  qzone:['在QQ空间官方分享页完成发布。打不开时，可以保存海报并复制文案，到QQ空间自行发布。','Use the official Qzone share page. If it does not open, save a poster and copy the text to post manually.'],
  wecom:['复制展厅地址，打开企业微信，选择客户或群聊，粘贴后发送；也可以发送保存的海报。','Copy the link, open WeCom, choose a contact or group, then paste and send. You can also send a saved poster.'],
  dingtalk:['复制展厅地址，打开钉钉，选择联系人或群聊，粘贴后发送；也可以发送保存的二维码。','Copy the link, open DingTalk, choose a contact or group, then paste and send. You can also send a saved QR code.'],
 };
 return guides[channel][zh?0:1];
}
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
