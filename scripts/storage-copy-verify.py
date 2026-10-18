#!/usr/bin/env python3
"""Copy an explicitly confirmed local media root without renaming or overwriting files.
Stop writes first. This script never changes configuration or restore pointers.
"""
import argparse, hashlib, json, os, shutil
from pathlib import Path

def digest(p):
    h=hashlib.sha256()
    with p.open('rb') as f:
        for block in iter(lambda:f.read(1024*1024),b''): h.update(block)
    return h.hexdigest()

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--source',required=True,help='Actual effective root verified in storage profiles and active.json')
    p.add_argument('--target',required=True)
    p.add_argument('--operations',required=True,help='Stable existing OPERATIONS_DIR')
    p.add_argument('--copy',action='store_true',help='Default only checks/plans; copy never overwrites mismatched files')
    a=p.parse_args();source=Path(a.source).resolve(strict=True);target=Path(a.target).resolve();operations=Path(a.operations).resolve(strict=True)
    if not source.is_dir() or source==Path(source.anchor) or target==Path(target.anchor) or source==target or source in target.parents or target in source.parents: p.error('Unsafe or overlapping source/target')
    pointer=operations/'active.json';control=json.loads(pointer.read_text('utf-8-sig')) if pointer.exists() else None
    if control and control.get('pending'): p.error('Resolve pending restore/cutover first; preserve existing control state')
    files=[]
    for root,dirs,names in os.walk(source,followlinks=False):
        for name in dirs+names:
            f=Path(root)/name
            if f.is_symlink(): p.error('Symlink refused: '+str(f))
        for name in names:
            f=Path(root)/name;key=f.relative_to(source);dst=target/key;sha=digest(f)
            if dst.exists() and (not dst.is_file() or digest(dst)!=sha): p.error('Existing different target file: '+str(dst))
            files.append((f,dst,str(key).replace('\\','/'),f.stat().st_size,sha))
    if a.copy:
        target.mkdir(parents=True,exist_ok=True)
        for src,dst,key,size,sha in files:
            dst.parent.mkdir(parents=True,exist_ok=True)
            if not dst.exists():
                # Exclusive file creation: never overwrite a concurrent destination.
                with src.open('rb') as inp,dst.open('xb') as out: shutil.copyfileobj(inp,out,1024*1024)
                shutil.copystat(src,dst)
            if dst.stat().st_size!=size or digest(dst)!=sha: raise RuntimeError('Checksum mismatch: '+key)
    result={'source':str(source),'target':str(target),'operations':str(operations),'restore_control':control,'copied_and_verified':a.copy,'objects':[{'key':key,'size':size,'sha256':sha} for _,_,key,size,sha in files]}
    # Contains paths, not credentials. Keep this report with operations records.
    print(json.dumps(result,ensure_ascii=False,indent=2))
if __name__=='__main__':main()
