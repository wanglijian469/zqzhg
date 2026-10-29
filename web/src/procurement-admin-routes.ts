const base='/admin/procurement';
const pages={
 requests:{zh:'采购询比价',en:'Procurement center',tab:'requests'},
 reviews:{zh:'采购需求审核',en:'Procurement reviews',tab:'reviews'},
 stats:{zh:'采购询比价统计',en:'Procurement statistics',tab:'stats'},
 project:{zh:'采购项目详情',en:'Procurement project details',tab:'requests'},
} as const;

export function procurementAdminPage(path:string){
 if(path!==base&&!path.startsWith(base+'/'))return undefined;
 const section=path.slice(base.length).replace(/^\/|\/$/g,'');
 const page=section==='reviews'?pages.reviews:section==='stats'?pages.stats:section.startsWith('projects/')?pages.project:pages.requests;
 return {...page,nested:section!==''};
}
