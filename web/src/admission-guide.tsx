import {useEffect,useState,type ReactNode} from 'react';
import {Link,useLocation} from 'react-router-dom';
import {ArrowRight,BookOpen,Check,ChevronRight,Expand,HelpCircle} from 'lucide-react';
import {Modal} from './ui';
import {useRuntimeConfig} from './runtime-config';
import './admission-guide.css';

const chapters=[['register','如何注册'],['company','填写企业资料'],['products','添加产品'],['review','提交审核与修改资料'],['find','查找展厅'],['share','分享展厅'],['contact','采购联系'],['fees','收费说明'],['faq','常见问题'],['help','遇到问题怎么办']] as const;
const steps=[['register','免费注册'],['company','填写企业资料'],['products','添加产品'],['review','提交审核'],['share','分享展厅']] as const;
const faqs=[
 ['已有账号，还要重新注册吗？','不用。点击“登录”，填写手机号或用户名和密码。已有企业展厅请继续使用原账号，避免重复申请。'],
 ['忘记密码怎么办？','在登录页点击“忘记密码？”，使用已验证的手机号找回密码。没有绑定手机号或暂时无法验证时，请通过“投诉与服务申请”说明情况，不要把密码发给别人。'],
 ['保存草稿后，别人能看到吗？','不能。点击“保存云端草稿”只是把资料保存到网站，方便下次继续；还要点击“提交审核”。审核通过、展厅公开后，客户才能看到。'],
 ['资料没有通过审核怎么办？','进入企业资料或产品管理，查看审核原因，按提示修改后重新提交。请填写真实资料，不要反复提交没有修改的内容。'],
 ['为什么修改后，展厅还是旧资料？','已公开的资料修改后需要再次提交审核。审核期间继续展示原来的公开资料；通过后才更新。待审核时暂不能编辑。'],
 ['为什么找不到自己的展厅或产品？','先检查是否已提交、审核通过并处于公开展示状态。企业尚未公开时，即使产品已通过审核，也不会对外展示。已下架的内容也不能被搜索到。'],
 ['照片上传失败怎么办？','先检查网络，按页面提示重试；也可以换一张更小的照片。没有上传成功时先不要刷新，刷新后可能需要重新选择照片。文件大小和格式以上传页面的提示为准。'],
 ['二维码打不开或无法保存怎么办？','先确认展厅已经公开，再重新生成二维码。把二维码图片发到另一部手机上扫描。保存方式因手机而异，也可复制展厅地址发送给客户。'],
 ['入驻后一定会出现在首页吗？','不一定。首页展示位置有限，免费入驻不代表一定获得首页展示。客户仍可通过搜索、分类、地区或您分享的展厅地址找到企业。'],
 ['一定要下载 App 吗？','不用，手机浏览器也能注册、填写资料和管理展厅。如需安卓 App，可前往“下载 App”页面；苹果 App 暂未开放，请先使用网页。'],
];

function Section({id,number,title,children}:{id:string;number:number;title:string;children:ReactNode}){
 return <section id={id} className="guide-chapter" aria-labelledby={'guide-heading-'+id} tabIndex={-1}><header className="guide-chapter-heading"><span>{String(number).padStart(2,'0')}</span><h2 id={'guide-heading-'+id}>{title}</h2></header>{children}<a className="guide-top" href="#guide-directory">返回目录 ↑</a></section>;
}
function Instructions({items}:{items:ReactNode[]}){return <ol className="guide-instructions">{items.map((item,i)=><li key={i}><span aria-hidden="true">{i+1}</span><div>{item}</div></li>)}</ol>}
function GuideImage({file,title,onOpen}:{file:string;title:string;onOpen:(src:string,title:string)=>void}){
 const [failed,setFailed]=useState(false),[attempt,setAttempt]=useState(0);const src='/guide-images/'+file+'.png'+(attempt?'?retry='+attempt:'');
 return <figure className="guide-image">{failed?<div className="guide-image-fallback"><p>这张操作图暂时无法显示，照着上面的文字也能完成。</p><button className="button secondary" onClick={()=>{setFailed(false);setAttempt(n=>n+1)}}>重新加载图片</button></div>:<button type="button" className="guide-image-button" aria-label={'放大查看：'+title} onClick={()=>onOpen(src,title)}><img src={src} alt={title+'，示例资料'} loading="lazy" decoding="async" onError={()=>setFailed(true)}/><span><Expand size={16}/>点击放大</span></button>}<figcaption>{title}<small>当前网页截图 · 示例资料，请填写您自己的信息</small></figcaption></figure>;
}

export function AdmissionGuide(){
 const cfg=useRuntimeConfig(),sms=cfg.capabilities?.phone_sms_register===true;
 const location=useLocation();const [image,setImage]=useState<{src:string;title:string}>();
 useEffect(()=>{
  const id=location.hash.slice(1);if(!id)return;
  const frame=requestAnimationFrame(()=>{const node=document.getElementById(id);if(node?.closest('.admission-guide')){node.scrollIntoView({block:'start'});node.focus({preventScroll:true})}});
  return()=>cancelAnimationFrame(frame);
 },[location.hash]);
 const open=(src:string,title:string)=>setImage({src,title});
 return <div className="admission-guide" lang="zh-CN">
  <header className="guide-hero"><div className="guide-hero-inner"><span className="guide-eyebrow"><BookOpen size={18}/>中企展馆 · 新手指南</span><h1>厂商入驻与使用指南</h1><p className="guide-lead">第一次用也没关系。跟着下面的步骤，<br className="guide-desktop-break"/>把企业和产品介绍给更多客户。</p><div className="guide-actions"><Link className="button" to="/join">立即免费入驻<ArrowRight size={18}/></Link><Link className="button secondary" to="/merchant/login">登录</Link><Link className="guide-service-link" to="/service-requests">投诉与服务申请<ChevronRight size={16}/></Link></div><p className="guide-free-note"><Check size={16}/>基础入驻免费 · 不用一次填完 · 手机也能操作</p></div></header>
  <div className="container guide-content">
   <section className="guide-five" aria-labelledby="guide-five-title"><div><h2 id="guide-five-title">5步完成入驻</h2><p>资料审核通过后才会公开。先准备企业名称、主营产品、联系方式和产品照片。</p></div><ol>{steps.map(([id,title],i)=><li key={id}><a href={'#'+id}><span>{i+1}</span><strong>{title}</strong><ChevronRight size={16}/></a></li>)}</ol></section>
   <div className="guide-layout"><nav id="guide-directory" className="guide-directory" aria-label="指南目录" tabIndex={-1}><span>跟着目录找答案</span><h2>我想了解…</h2><ol>{chapters.map(([id,title],i)=><li key={id}><a href={'#'+id}><small>{String(i+1).padStart(2,'0')}</small>{title}</a></li>)}</ol><p><HelpCircle size={17}/>操作遇到困难？<Link to="/service-requests">提交问题，我们帮您看</Link></p></nav>
   <article className="guide-article" aria-label="入驻操作说明">
    <Section id="register" number={1} title="如何注册">
     <p>先注册一个自己的账号，再申请企业展厅。已有账号的用户直接<Link to="/merchant/login">登录</Link>，不用重新注册。</p>
     <h3>用用户名和密码注册</h3><Instructions items={['点击“立即免费入驻”，进入“免费注册”页面，再选择“账号密码注册”。','填写“用户名”“设置密码”“确认密码”。密码为8–20位，建议用字母和数字组合，请自己妥善保存。','阅读并同意“用户协议”和“隐私政策”，点击“注册并进入展馆”。注册成功后会自动登录。']}/>
     <GuideImage file="register-account" title="选择账号密码注册，填写后注册" onOpen={open}/>
     {sms?<><h3>也可以用手机验证码注册</h3><Instructions items={['选择“手机验证码注册”，填写自己的手机号。如页面要求图形验证码，请一并填写，再点击“获取验证码”。','收到短信后，填写六位验证码，点击“下一步”。','设置并确认密码，阅读并同意协议，再点击“注册并进入展馆”。以后使用手机号和密码登录。']}/><GuideImage file="register-phone" title="手机验证后，还需设置登录密码" onOpen={open}/><p className="guide-note">没有收到短信？先核对手机号，稍等片刻，并查看手机是否拦截。按页面倒计时重试；也可改用“账号密码注册”。</p></>:<p className="guide-note">手机验证码注册暂未开放或尚未确认可用，请先选择“账号密码注册”。登录时使用手机号或用户名和密码，不是短信验证码登录。</p>}
     <Link className="button" to="/join">去免费注册<ArrowRight size={17}/></Link>
    </Section>
    <Section id="company" number={2} title="填写企业资料">
     <Instructions items={['首次注册后，在欢迎提示中点击“我是厂家，免费入驻”。如果已跳过提示，点击网站上的“我的账号”。','在“我的账号”页面点击“确认申请免费企业展厅”，进入企业资料填写页面。','填写“企业 / 工厂名称”、省份、城市和“主营产品”。区县可选填，电话或微信至少填一项。选择符合实际业务的“主营分类”，这是提交审核前必须完成的一项。','点击“保存云端草稿”，把资料保存到网站。保存成功后就能继续添加产品，下次登录也可继续填写。']}/>
     <div className="guide-image-pair"><GuideImage file="company-apply" title="在我的账号中申请免费企业展厅" onOpen={open}/><GuideImage file="company-basics" title="填写企业基本资料，再保存草稿" onOpen={open}/></div>
     <p className="guide-note">企业介绍、Logo、工厂照片、生产能力等可以逐步补充；英文资料选填。有真实资质才添加，不需要编造。填完草稿不代表已经公开。</p><Link className="button secondary" to="/account">打开我的账号</Link>
    </Section>
    <Section id="products" number={3} title="添加产品">
     <Instructions items={['先保存企业基本资料，再进入“产品管理”，点击“添加产品”。','在“基础信息”中填写产品名称等资料。提交审核前，请把产品分类选到最具体的一类。','点击“下一步”到“图片与展示资料”，添加清晰的产品照片和介绍。可以调整封面；没有上传成功的照片要先重试或移除。','再次点击“下一步”到“预览并提交”，核对内容。暂时没填完，可先点“保存云端草稿”；准备好后再“提交审核”。']}/>
     <GuideImage file="product-basics" title="添加产品，按三个步骤填写" onOpen={open}/><Link className="button secondary" to="/merchant/products">打开产品管理</Link>
    </Section>
    <Section id="review" number={4} title="提交审核与修改资料">
     <h3>企业资料和产品，要分别提交</h3><Instructions items={['先核对企业资料是否已选择“主营分类”。在编辑页面点击“协议与提交”，阅读“服务协议确认”中的协议，勾选同意后点击“确认协议”，再点击“提交审核”。如果已显示“已确认当前版本协议”，可直接核对后提交。','产品在“预览并提交”这一步点击“提交审核”。每款产品都需要单独提交；首次提交前，也需要先完成企业页面的协议确认。','返回企业资料或产品管理，查看“审核状态”和审核原因。资料审核通过、企业展厅公开后，客户才能看到相应内容。']}/>
     <GuideImage file="company-agreements" title="阅读并确认当前版本协议" onOpen={open}/>
     <div className="guide-image-pair"><GuideImage file="company-submit" title="企业资料：确认协议后提交审核" onOpen={open}/><GuideImage file="product-submit" title="产品：预览后单独提交审核" onOpen={open}/></div>
     <h3>以后想修改，怎么做？</h3><p>登录后进入“企业资料”或“产品管理”，打开要修改的资料，保存后重新“提交审核”。待审核时暂不能编辑；未通过时先查看原因再修改。</p><p className="guide-note">修改审核期间，客户仍看到原来的公开版本。审核不是立即完成，也不代表平台对企业资质或产品质量作出保证。</p>
     <Link className="button secondary" to="/merchant/booth">管理我的展厅</Link>
    </Section>
    <Section id="find" number={5} title="查找展厅">
     <Instructions items={['在网站的找厂家或找产品页面，输入企业名称、产品名称或型号，也可以按分类、地区缩小范围。','点击厂家卡片进入企业展厅；在产品详情里，也可点击所属厂商查看展厅。','自己的展厅公开后，可在企业管理页面点击“查看展厅”。尚未审核通过、未公开或已下架的资料不能被客户找到。']}/>
     <div className="guide-actions"><Link className="button secondary" to="/browse">去找厂家</Link><Link className="button secondary" to="/browse/products">去找产品</Link><Link className="button secondary" to="/regions">打开厂商地图</Link></div>
    </Section>
    <Section id="share" number={6} title="分享展厅">
     <p>等展厅公开后再分享，客户才能打开。登录后进入“分享官网”，也可以在公开展厅中打开分享面板。</p><Instructions items={['点击“复制地址”，再粘贴到微信聊天等地方发送。如果复制失败，可长按展厅地址复制。','点击“展厅二维码”，查看二维码，点击二维码图片可下载；手机上的保存方式以浏览器提示为准。','需要海报时，点击“生成海报预览”，核对企业、产品和联系方式后，再点击“保存宣传海报”。','把保存的二维码或海报发送给客户，也可自行发到朋友圈。网站不会替您自动发送或发布。']}/>
     <GuideImage file="showroom-share" title="公开后复制展厅地址、查看二维码或生成海报" onOpen={open}/><p className="guide-note">示例截图中的地址与二维码不用于推广，请使用您自己的公开展厅地址。“分享官网”能否调用手机分享面板，取决于手机和浏览器；不支持时，复制地址或保存图片也能分享。</p><Link className="button secondary" to="/merchant/promotion">打开分享官网</Link>
    </Section>
    <Section id="contact" number={7} title="采购联系">
     <Instructions items={['先打开企业展厅或产品详情，查看产品、型号和厂商公开的联系方式。','手机上可点电话号码联系厂家；也可复制微信号，再到微信中添加联系。企业没有填写的联系方式不会显示。','把产品名称、所需型号、数量和使用要求告诉厂家，直接确认价格、供货及售后安排。']}/><p className="guide-note">线上询价和采购中心暂未开放。目前请通过电话、微信直接联系厂家，网站不提供在线下单或支付。</p><Link className="button secondary" to="/browse/products">查看产品并联系厂家</Link>
    </Section>
    <Section id="fees" number={8} title="收费说明">
     <p><strong>基础入驻免费。</strong>具体服务内容和免费资源说明，请查看现有《入驻与服务说明》；可能收费的增值服务也以该说明及双方另行确认为准。</p><p>首页展示位置有限，不承诺入驻后一定登上首页。请不要把获得首页展示当作注册或入驻的条件。</p><Link className="button secondary" to="/services">查看入驻与服务说明<ArrowRight size={17}/></Link>
    </Section>
    <Section id="faq" number={9} title="常见问题">
     <p>点一下问题，就能展开查看答案。</p><div className="guide-faq">{faqs.map(([q,a])=><details key={q}><summary>{q}</summary><p>{a}</p></details>)}</div>
    </Section>
    <Section id="help" number={10} title="遇到问题怎么办">
     <Instructions items={['打开“投诉与服务申请”，选择申请类型，写明您在哪一步遇到问题、页面提示了什么；如需我们联系，请留下联系方式。不要填写登录密码。','点击“提交申请”后，保存好申请编号和“私密查询凭证”，也可以点击“下载查询凭证”。','以后在同一页面的“查询反馈”中，填写“工单编号”和“私密查询凭证”，点击“查询”查看回复。需要时可“提交补充”。']}/><p className="guide-note">投诉举报可不登录提交；企业资料导出、停止展示、退出、注销及免费资源调整，需要登录并核对资料归属，提交申请不会自动删除资料。</p><div className="guide-actions"><Link className="button" to="/service-requests">投诉与服务申请<ArrowRight size={17}/></Link><Link className="button secondary" to="/app">查看下载 App 说明</Link></div>
    </Section>
    <div className="guide-finish"><BookOpen size={25}/><h2>准备好了？从第一步开始</h2><p>先注册，先保存，资料可以慢慢完善。</p><Link className="button" to="/join">立即免费入驻<ArrowRight size={18}/></Link></div>
   </article></div>
  </div>
  {image&&<Modal close={()=>setImage(undefined)} className="guide-image-modal" labelledBy="guide-image-title"><h2 id="guide-image-title">{image.title}</h2><p>当前网页截图 · 示例资料。图片较长时可以向下滚动。</p><img src={image.src} alt={image.title+'，放大图'} onError={()=>setImage(undefined)}/></Modal>}
 </div>;
}
