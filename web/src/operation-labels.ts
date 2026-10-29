const actions:Record<string,[string,string]>={
 visitor_registered:['普通账号注册','Account registration'],merchant_registered:['历史厂商账号注册','Company account registration'],
 phone_registered:['手机账号注册','Mobile account registration'],phone_registration:['手机账号注册','Mobile account registration'],
 phone_password_login:['密码登录','Password sign-in'],phone_login:['密码登录','Password sign-in'],
 phone_password_reset:['找回密码','Password recovery'],password_login:['密码登录','Password sign-in'],
 phone_bound:['绑定手机号','Mobile number linked'],phone_password_setup:['设置密码','Password setup'],
 visitor_upgraded_to_merchant:['申请企业展厅','Company showroom application'],
 draft_saved:['保存草稿','Draft saved'],created:['新增资料','Materials created'],submit:['提交审核','Submitted for review'],
 approve:['审核通过','Approved'],reject:['驳回修改','Rejected for correction'],offline:['停止展示','Unpublished'],
 service_document_published:['发布服务文档','Service document published'],service_document_saved:['保存服务文档','Service document saved'],
 service_request_created:['提交服务申请','Service request submitted'],service_request_updated:['处理服务申请','Service request updated'],
 verification_submitted:['历史企业核验提交','Historical verification submission'],verification_approved:['历史企业核验通过','Historical verification approval'],
 verification_revoked:['历史企业核验撤销','Historical verification withdrawal'],
};
export function operationText(action:string,lang:string){return actions[action]?.[lang==='zh'?0:1]||(lang==='zh'?'其他操作':'Other activity')}
