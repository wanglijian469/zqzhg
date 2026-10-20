import {init,registerMap,use} from 'echarts/core';
import {MapChart} from 'echarts/charts';
import {TooltipComponent} from 'echarts/components';
import {SVGRenderer} from 'echarts/renderers';
import {LabelLayout} from 'echarts/features';
import polygonClipping from 'polygon-clipping';
import type {MultiPolygon,Polygon} from 'polygon-clipping';
import type {PreviewBoundary,RegionStatistics} from './industry-map-types';
use([MapChart,TooltipComponent,SVGRenderer,LabelLayout]);

export function mergedPreviewGeometry(resource:Pick<PreviewBoundary,'region_code'|'sha256'|'geometry'>){
 const groups=new Map<string,PreviewBoundary['geometry']['features']>();
 for(const feature of resource.geometry.features){const code=feature.properties.adcode;const parts=groups.get(code)||[];parts.push(feature);groups.set(code,parts)}
 return {type:'FeatureCollection' as const,features:[...groups.values()].map(parts=>{
  const first=parts[0];if(parts.length===1)return {...first,properties:{...first.properties,name:first.properties.adcode}};
  const polygons=parts.map(part=>{const geometry=part.geometry as {type:string;coordinates:Polygon|MultiPolygon};if(!['Polygon','MultiPolygon'].includes(geometry.type))throw Error('invalid_polygon');return geometry.coordinates});
  const coordinates=polygonClipping.union(polygons[0],...polygons.slice(1));
  if(!coordinates.length)throw Error('empty_group_geometry');
  const properties={...first.properties,name:first.properties.adcode};delete properties.cp;
  return {...first,properties,geometry:{type:'MultiPolygon',coordinates}};
 })};
}
const colors=['#dfeafa','#d4e5f8','#a5c9ee','#69a3db','#347dc0','#17589c'];
function color(value:number,max:number){return colors[value===0?0:1+Math.min(4,Math.floor(Math.sqrt(value/Math.max(1,max))*4))]}
export function renderOutlineMap(element:HTMLDivElement,resource:Pick<PreviewBoundary,'region_code'|'sha256'|'geometry'>,statistics:RegionStatistics,onSelect:(code:string)=>void,t:(key:string)=>string){
 const geometry=mergedPreviewGeometry(resource),key='local-outline:'+resource.region_code+':'+resource.sha256;
 registerMap(key,JSON.stringify(geometry));const chart=init(element,undefined,{renderer:'svg'});
 let current=statistics,disposed=false,width=element.clientWidth,height=element.clientHeight;
 const features=new Map(resource.geometry.features.map(feature=>[feature.properties.adcode,feature.properties]));
 const mapped=new Set(resource.geometry.features.filter(feature=>feature.properties.mapped).map(feature=>feature.properties.adcode));
 const draw=(reset:boolean)=>{
  const items=current.items.length?current.items:current.region?[{...current.region,vendor_count:current.total}]:[];
  const counts=new Map(items.filter(item=>mapped.has(item.code)).map(item=>[item.code,item.vendor_count]));
  const names=new Map([...features].map(([code,props])=>[code,props.name||'']));for(const item of items)names.set(item.code,item.name);
  const duplicateNames=new Set(items.filter((item,index)=>items.some((other,otherIndex)=>index!==otherIndex&&other.name===item.name)).map(item=>item.name));
  const full=(code:string)=>{const name=names.get(code)||'';return duplicateNames.has(name)?name+' · '+code:name};
  const max=Math.max(0,...counts.values());
  const short=(code:string)=>{const name=names.get(code)||'';if(duplicateNames.has(name))return full(code);if(!current.region)return name.replace(/(壮族自治区|回族自治区|维吾尔自治区|自治区|省|市)$/,'');return name.length>8?name.slice(0,7)+'…':name};
  chart.setOption({animation:false,tooltip:{trigger:'item',renderMode:'richText',confine:true,formatter:(p:{name:string})=>full(p.name)+'\n'+(counts.has(p.name)?counts.get(p.name)+' '+t('imCompanies'):t('imOutsideCoverage'))},series:[{
   id:'factory-outline',type:'map',map:key,roam:true,layoutCenter:['50%','50%'],layoutSize:'92%',scaleLimit:{min:.7,max:10},
   ...(reset?{zoom:1,center:null}:{}),
   projection:{project:([x,y]:number[])=>[x,-180/Math.PI*Math.log(Math.tan(Math.PI/4+y*Math.PI/360))],unproject:([x,y]:number[])=>[x,360/Math.PI*Math.atan(Math.exp(-y*Math.PI/180))-90]},
   label:{show:true,formatter:(p:{name:string})=>counts.has(p.name)?short(p.name)+'\n'+counts.get(p.name):'',fontSize:element.clientWidth<500?9:11,lineHeight:14,color:'#21466f',textBorderColor:'#ffffff',textBorderWidth:2},labelLayout:{hideOverlap:true},
   itemStyle:{areaColor:'#e3e8ef',borderColor:'#fff',borderWidth:1},
   emphasis:{label:{show:true},itemStyle:{borderColor:'#1769c4',borderWidth:1.5}},
   data:[...features].map(([code,props])=>({name:code,...(counts.has(code)?{value:counts.get(code)}:{}),itemStyle:props.decoration?{areaColor:'transparent',borderColor:'#8495aa',borderWidth:.8}:{areaColor:counts.has(code)?color(counts.get(code)!,max):'#e3e8ef'},...(props.decoration?{label:{show:false},emphasis:{disabled:true},tooltip:{show:false},silent:true}:{})}))
  }]},{notMerge:reset});
 };
 try{draw(true);chart.on('click',params=>{if(mapped.has(params.name)&&current.items.some(item=>item.code===params.name))onSelect(params.name)})}catch(error){chart.dispose();throw error}
 return {
  dispose:()=>{disposed=true;chart.dispose()},
  resize:()=>{if(!disposed&&(width!==element.clientWidth||height!==element.clientHeight)){width=element.clientWidth;height=element.clientHeight;chart.resize()}},
  update:(value:RegionStatistics)=>{current=value;draw(false)},
  zoom:(factor:number)=>chart.dispatchAction({type:'geoRoam',seriesIndex:0,zoom:factor,originX:element.clientWidth/2,originY:element.clientHeight/2}),
  reset:()=>draw(true)
 };
}
