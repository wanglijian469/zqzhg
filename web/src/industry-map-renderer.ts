import {init,registerMap,use} from 'echarts/core';
import {MapChart} from 'echarts/charts';
import {TooltipComponent,VisualMapComponent} from 'echarts/components';
import {CanvasRenderer} from 'echarts/renderers';
import type {BoundaryGeometry,RegionStatistics} from './industry-map-types';
use([MapChart,TooltipComponent,VisualMapComponent,CanvasRenderer]);
export function renderIndustryMap(element:HTMLDivElement,key:string,boundary:BoundaryGeometry,statistics:RegionStatistics,onSelect:(code:string)=>void,t:(key:string)=>string){
 const geometry={...boundary,features:boundary.features.map(feature=>({...feature,properties:{...feature.properties,name:String(feature.properties.adcode)}}))};
 registerMap(key,JSON.stringify(geometry));
 const instance=init(element);
 const items=statistics.items.length?statistics.items:statistics.region?[{...statistics.region,vendor_count:statistics.total}]:[];
 const names=new Map(boundary.features.map(feature=>[String(feature.properties.adcode),feature.properties.name||String(feature.properties.adcode)]));
 for(const item of items)names.set(item.code,item.name);
 const counts=new Map(items.map(item=>[item.code,item.vendor_count]));
 try{instance.setOption({tooltip:{trigger:'item',renderMode:'richText',formatter:(p:{name:string;value:number})=>(names.get(p.name)||p.name)+'：'+(Number.isFinite(p.value)?p.value+' '+t('imCompanies'):t('imOutsideCoverage'))},visualMap:{min:0,max:Math.max(1,...items.map(item=>item.vendor_count)),left:12,bottom:26,inRange:{color:['#edf3fc','#94b9e3','#1764b3']},text:[t('imMore'),t('imLess')],textStyle:{fontSize:10,color:'#5d7797'},calculable:false,itemHeight:95},series:[{id:'factory-map',type:'map',map:key,roam:true,layoutCenter:['50%','48%'],layoutSize:'88%',scaleLimit:{min:1,max:8},label:{show:true,formatter:(p:{name:string})=>(names.get(p.name)||p.name)+(counts.has(p.name)?'\n'+counts.get(p.name)+' '+t('imCompanies'):''),fontSize:10,lineHeight:14,color:'#183a62',textBorderColor:'#ffffff',textBorderWidth:2},labelLayout:{hideOverlap:true},itemStyle:{borderColor:'#fff',borderWidth:1},emphasis:{label:{show:true,fontWeight:700},itemStyle:{areaColor:'#b1d1f3',borderColor:'#3977bd'}},data:items.map(item=>({name:item.code,value:item.vendor_count}))}]});}catch(error){instance.dispose();throw error}
 instance.on('click',params=>{if(statistics.items.some(item=>item.code===params.name))onSelect(params.name)});
 return {dispose:()=>instance.dispose(),resize:()=>instance.resize(),zoom:(factor:number)=>instance.dispatchAction({type:'geoRoam',seriesIndex:0,zoom:factor}),reset:()=>instance.setOption({series:[{id:'factory-map',center:null,zoom:1}]})};
}
