import fs from 'node:fs';
import path from 'node:path';
import {createRequire} from 'node:module';
const require=createRequire(path.resolve('.tools/icon-renderer/package.json'));
const {Resvg}=require('@resvg/resvg-js');
const svg=fs.readFileSync('web/public/brand.svg','utf8').replace('rx="9"','rx="0"');
function image(file,size){const resvg=new Resvg(svg,{fitTo:{mode:'width',value:size},background:'#153f31'});fs.writeFileSync(file,resvg.render().asPng())}
const ios='mobile/ios/Runner/Assets.xcassets/AppIcon.appiconset';
const assets=JSON.parse(fs.readFileSync(path.join(ios,'Contents.json'),'utf8'));
for(const asset of assets.images){if(asset.filename)image(path.join(ios,asset.filename),Math.round(parseFloat(asset.size)*parseFloat(asset.scale || '1')))}
for(const [name,size] of [['mdpi',48],['hdpi',72],['xhdpi',96],['xxhdpi',144],['xxxhdpi',192]])image(`mobile/android/app/src/main/res/mipmap-${name}/ic_launcher.png`,size);
console.log('Application icons generated from the shared vector brand.');
