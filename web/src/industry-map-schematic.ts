import {init,use} from 'echarts/core';
import {GraphChart} from 'echarts/charts';
import {TooltipComponent} from 'echarts/components';
import {SVGRenderer} from 'echarts/renderers';
import type {RegionCount,RegionStatistics} from './industry-map-types';

use([GraphChart,TooltipComponent,SVGRenderer]);

// Abstract cartogram slots, not geographic coordinates or administrative boundaries.
const nationalSlots:Record<string,[number,number]>={
 '65':[0,1],'54':[0,4],'63':[2,3],'62':[3,2],'64':[4,2],'15':[6,1],
 '23':[10,0],'22':[10,1],'21':[9,2],'11':[7,2],'12':[8,2],'13':[7,3],
 '14':[6,3],'61':[5,3],'37':[8,3],'41':[6,4],'32':[9,4],'34':[8,4],
 '31':[10,4],'33':[9,5],'42':[7,5],'36':[8,5],'43':[7,6],'35':[9,6],
 '44':[8,7],'45':[6,7],'46':[7,8],'51':[3,4],'50':[5,5],'52':[5,6],'53':[3,6]
};
const provinceLabels:Record<string,string>={'11':'北京','12':'天津','13':'河北','14':'山西','15':'内蒙古','21':'辽宁','22':'吉林','23':'黑龙江','31':'上海','32':'江苏','33':'浙江','34':'安徽','35':'福建','36':'江西','37':'山东','41':'河南','42':'湖北','43':'湖南','44':'广东','45':'广西','46':'海南','50':'重庆','51':'四川','52':'贵州','53':'云南','54':'西藏','61':'陕西','62':'甘肃','63':'青海','64':'宁夏','65':'新疆'};
export function schematicLabel(name:string,width:number,font:number){const limit=Math.max(2,Math.floor((width-12)/font)),chars=Array.from(name);return chars.length>limit?chars.slice(0,limit-1).join('')+'…':name}
export const schematicColors=['#edf3fc','#d4e5f8','#a5c9ee','#69a3db','#347dc0','#17589c'];
export function schematicColor(count:number,max:number){return schematicColors[count===0?0:1+Math.min(4,Math.floor(Math.sqrt(count/Math.max(1,max))*4))]}

export function schematicLayout(items:RegionCount[],width:number,height:number,national:boolean){
 const sorted=[...items].sort((a,b)=>a.code.localeCompare(b.code));
 const w=Math.max(1,width),h=Math.max(1,height),pad=12,gap=8;
 const spatial=national&&w>=680&&h>=300&&sorted.every(item=>nationalSlots[item.code]);
 const usableW=Math.max(1,w-2*pad),usableH=Math.max(1,h-2*pad);
 const cols=spatial?11:Math.min(Math.max(1,sorted.length),Math.max(1,Math.ceil(Math.sqrt(sorted.length*usableW/usableH/1.65))));
 const rows=spatial?9:Math.max(1,Math.ceil(sorted.length/cols));
 const cellW=usableW/cols,cellH=usableH/rows;
 const tileW=Math.min(150,Math.max(1,cellW-gap)),tileH=Math.min(80,Math.max(1,cellH-gap));
 const nodes=sorted.map((item,index)=>{const [col,row]=spatial?nationalSlots[item.code]:[index%cols,Math.floor(index/cols)];return {...item,x:pad+(col+.5)*cellW,y:pad+(row+.5)*cellH}});
 return {nodes,tileW,tileH,spatial};
}

export function renderSchematicMap(element:HTMLDivElement,statistics:RegionStatistics,onSelect:(code:string)=>void,t:(key:string)=>string){
 const instance=init(element,undefined,{renderer:'svg'});
 const items=statistics.items.length?statistics.items:statistics.region?[{...statistics.region,vendor_count:statistics.total}]:[];
 let disposed=false,lastWidth=0,lastHeight=0;
 const draw=()=>{
  const width=element.clientWidth||320,height=element.clientHeight||360;
  lastWidth=width;lastHeight=height;
  const {nodes,tileW,tileH}=schematicLayout(items,width,height,!statistics.region);
  const max=Math.max(0,...items.map(item=>item.vendor_count));
  const font=tileW<70||tileH<40?10:12;
  const xs=nodes.map(node=>node.x),ys=nodes.map(node=>node.y);
  const minX=Math.min(...xs),maxX=Math.max(...xs),minY=Math.min(...ys),maxY=Math.max(...ys);
  instance.setOption({animation:false,tooltip:{trigger:'item',renderMode:'richText',confine:true,formatter:(p:{data:{regionName:string;value:number}})=>p.data.regionName+'\n'+p.data.value+' '+t('imCompanies')},series:[{
   id:'factory-schematic',type:'graph',layout:'none',preserveAspect:false,
   left:minX-(minX===maxX?1:0),right:width-maxX-(minX===maxX?1:0),top:minY-(minY===maxY?1:0),bottom:height-maxY-(minY===maxY?1:0),
   roam:true,scaleLimit:{min:.7,max:4},nodeScaleRatio:1,zoom:1,center:null,
   symbol:'roundRect',symbolSize:[tileW,tileH],draggable:false,links:[],
   label:{show:true,position:'inside',formatter:(p:{data:{labelName:string;value:number}})=>'{region|'+p.data.labelName+'}\n{count|'+p.data.value.toLocaleString()+'}',rich:{region:{fontSize:font,lineHeight:font+5,align:'center'},count:{fontSize:font+2,fontWeight:700,lineHeight:font+5,align:'center'}}},
   emphasis:{scale:false,itemStyle:{borderColor:'#1769c4',borderWidth:2}},
   data:nodes.map(node=>({id:node.code,name:node.code,regionName:node.name,labelName:schematicLabel(!statistics.region?provinceLabels[node.code]||node.name:node.name,tileW,font),value:node.vendor_count,x:node.x,y:node.y,cursor:statistics.items.length?'pointer':'default',itemStyle:{color:schematicColor(node.vendor_count,max),borderColor:node.vendor_count?'#5496d1':'#d5e2f2',borderWidth:1},label:{color:node.vendor_count>0&&node.vendor_count>=max*.4?'#fff':'#294d78'}}))
  }]},{notMerge:true});
 };
 try{draw();instance.on('click',params=>{if(statistics.items.some(item=>item.code===params.name))onSelect(params.name)})}catch(error){instance.dispose();throw error}
 return {
  dispose:()=>{disposed=true;instance.dispose()},
  resize:()=>{if(!disposed&&(lastWidth!==(element.clientWidth||320)||lastHeight!==(element.clientHeight||360))){instance.resize();draw()}},
  zoom:(factor:number)=>instance.dispatchAction({type:'graphRoam',seriesIndex:0,zoom:factor,originX:element.clientWidth/2,originY:element.clientHeight/2}),
  reset:()=>{if(!disposed)draw()}
 };
}
