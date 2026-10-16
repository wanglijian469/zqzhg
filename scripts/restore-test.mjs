import fs from 'node:fs';
import path from 'node:path';
import {spawnSync} from 'node:child_process';
import crypto from 'node:crypto';
if(!process.argv[2]) throw new Error('Usage: node scripts/restore-test.mjs <backup-directory>');
const dir=path.resolve(process.argv[2]);
const name='zqzt_exhibition_restore_'+Date.now();
const bin=process.env.MYSQL_BIN || 'D:/InforGuardMC/InforGuardMC/mysql/bin';
const args=['--no-defaults','--host='+ (process.env.DB_HOST || '127.0.0.1'),'--port='+ (process.env.DB_PORT || '13306'),'--user='+ (process.env.DB_USER || 'root'),'--default-character-set=utf8mb4','--batch'];
const env={...process.env,MYSQL_PWD:process.env.DB_PASSWORD || 'root'};
const sql=fs.readFileSync(path.join(dir,'database.sql'));
const manifest=JSON.parse(fs.readFileSync(path.join(dir,'manifest.json'),'utf8').replace(/^\uFEFF/,''));
for(const [file,expected] of [['database.sql',manifest.sqlHash],['media.zip',manifest.mediaHash]]){
  const actual=crypto.createHash('sha256').update(fs.readFileSync(path.join(dir,file))).digest('hex');
  if(!expected || actual!==expected.toLowerCase())throw new Error('Backup checksum mismatch: '+file);
}
function run(input,extra=[]){const r=spawnSync(path.join(bin,'mysql.exe'),[...args,...extra],{input,env,encoding:'utf8',maxBuffer:8*1024*1024});if(r.status!==0)throw new Error(r.stderr||r.error?.message);return r.stdout}
run(`CREATE DATABASE \`${name}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;`);
run(sql,[name]);
const evidence=run('SELECT COUNT(*) AS migrated_versions FROM schema_migrations; SELECT COUNT(*) AS vendors_and_products FROM contents; SELECT COUNT(*) AS image_records FROM media;',[name]);
const mediaDirectory=path.join(dir,'restored-media-'+name);
const unpack=spawnSync('pwsh',['-NoProfile','-Command','Add-Type -AssemblyName System.IO.Compression.FileSystem; [IO.Compression.ZipFile]::ExtractToDirectory($env:ZQZT_RESTORE_ZIP,$env:ZQZT_RESTORE_MEDIA)'],{env:{...process.env,ZQZT_RESTORE_ZIP:path.join(dir,'media.zip'),ZQZT_RESTORE_MEDIA:mediaDirectory},encoding:'utf8'});
if(unpack.status!==0)throw new Error(unpack.stderr || 'Media restore failed');
fs.writeFileSync(path.join(dir,'restore-test.txt'),`${name}\n${evidence}\nRestored media: ${mediaDirectory}\nSHA-256 checks passed.\n`);
console.log(name+'\n'+evidence+'\nRestored media: '+mediaDirectory+'\nSHA-256 checks passed.');
// The restored database is retained for inspection. Its prefix identifies it as a restore test.
