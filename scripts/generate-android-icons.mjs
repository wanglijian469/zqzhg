import fs from 'node:fs';
import path from 'node:path';
import {createRequire} from 'node:module';
const require=createRequire(path.resolve('.tools/icon-renderer/package.json'));
const {Resvg}=require('@resvg/resvg-js');
const svg=fs.readFileSync('mobile/assets/branding/app-icon.svg','utf8');
const res='mobile/android/app/src/main/res';
for(const [density,size] of [['mdpi',48],['hdpi',72],['xhdpi',96],['xxhdpi',144],['xxxhdpi',192]]){
 fs.mkdirSync(`${res}/mipmap-${density}`,{recursive:true});fs.writeFileSync(`${res}/mipmap-${density}/ic_launcher.png`,new Resvg(svg,{fitTo:{mode:'width',value:size}}).render().asPng());
}
// Use the submitted filing artwork as supplied. Do not substitute a generated logo.
// Keep its complete artwork in the 72dp adaptive viewport; Android owns the mask.
fs.mkdirSync(`${res}/drawable-nodpi`,{recursive:true});
fs.copyFileSync('mobile/assets/branding/filing-icon-original.png',`${res}/drawable-nodpi/ic_launcher_artwork.png`);
const foreground=`<inset xmlns:android="http://schemas.android.com/apk/res/android" android:inset="16.666667%"><bitmap android:src="@drawable/ic_launcher_artwork" android:gravity="fill" android:filter="true"/></inset>\n`;
fs.writeFileSync(`${res}/drawable/ic_launcher_foreground.xml`,foreground);
fs.writeFileSync(`${res}/drawable/ic_zqzt.xml`,`<bitmap xmlns:android="http://schemas.android.com/apk/res/android" android:src="@drawable/ic_launcher_artwork" android:gravity="fill" android:filter="true"/>\n`);
fs.mkdirSync(`${res}/mipmap-anydpi-v26`,{recursive:true});
fs.writeFileSync(`${res}/mipmap-anydpi-v26/ic_launcher.xml`,`<adaptive-icon xmlns:android="http://schemas.android.com/apk/res/android"><background android:drawable="@color/ic_launcher_background"/><foreground android:drawable="@drawable/ic_launcher_foreground"/></adaptive-icon>\n`);
fs.writeFileSync(`${res}/values/ic_launcher_colors.xml`,`<resources><color name="ic_launcher_background">#007846</color></resources>\n`);
fs.mkdirSync('docs/screenshots',{recursive:true});
fs.writeFileSync('docs/screenshots/filing-app-icon.png',new Resvg(svg,{fitTo:{mode:'width',value:256}}).render().asPng());
console.log('Android icons generated from the supplied filing artwork: adaptive icon and 5 legacy densities.');
