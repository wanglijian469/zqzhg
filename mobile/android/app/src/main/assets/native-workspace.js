(function(p){
  window.__DLNJ_NATIVE__=true;
  document.documentElement?.classList.add('native-workspace');
  // New native activity: replace any previous account. Reload in this activity:
  // keep the current Web session, including an explicit logout.
  if(sessionStorage.getItem('dlnj.bootstrap')!==p.nonce){
    for(const k of ['zqzt.merchant','zqzt.merchant.id','zqzt.merchant.role','zqzt.admin'])sessionStorage.removeItem(k);
    if(p.token){
      sessionStorage.setItem('zqzt.merchant',p.token);
      sessionStorage.setItem('zqzt.merchant.id',String(p.account.id));
      sessionStorage.setItem('zqzt.merchant.role',p.account.role||'visitor');
    }
    localStorage.setItem('zqzt.language',p.language);
    sessionStorage.setItem('dlnj.bootstrap',p.nonce);
  }
  const files=new Map(),create=URL.createObjectURL.bind(URL),revoke=URL.revokeObjectURL.bind(URL);
  URL.createObjectURL=function(b){const u=create(b);files.set(u,b);return u;};
  URL.revokeObjectURL=function(u){files.delete(u);return revoke(u);};
  function save(u,name,preview=false){
    const b=files.get(u);
    if(!b||b.size>20*1024*1024){alert('附件无法保存，或超过20MB');return;}
    const r=new FileReader();
    // Android handles this same-origin prompt directly, without a JS interface,
    // public download URL, console logging or a token in the URL.
    r.onload=()=>prompt(p.nonce,JSON.stringify({name:name||'attachment',mime:b.type,data:r.result,preview}));
    r.readAsDataURL(b);
  }
  const click=HTMLAnchorElement.prototype.click;
  HTMLAnchorElement.prototype.click=function(){
    if(this.href.startsWith('blob:')){save(this.href,this.download,!this.download&&this.target==='_blank');return;}
    return click.call(this);
  };
  document.addEventListener('click',function(e){
    const a=e.target.closest?.('a');
    if(a?.href.startsWith('blob:')){e.preventDefault();save(a.href,a.download,!a.download&&a.target==='_blank');}
  },true);
  const open=window.open;
  window.open=function(u,...args){
    if(String(u).startsWith('blob:')){save(u,'',true);return null;}
    return open.call(window,u,...args);
  };
})(__DLNJ_BOOTSTRAP__);
