import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'node:path';
import {compressWebAssets} from '../scripts/compress-web-assets.mjs';
export default defineConfig({ plugins: [react(), (()=>{let directory='';return {name:'compressed-web-assets',configResolved(config){if(config.command==='build')directory=path.resolve(config.root,config.build.outDir)},async closeBundle(){if(directory)console.log('gzip assets:',await compressWebAssets(directory))}}})()], server: { port: 5173, proxy: { '/api': 'http://127.0.0.1:8080' } } });
