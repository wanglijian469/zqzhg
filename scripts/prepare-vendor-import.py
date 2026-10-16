"""Read the supplied V0001 workbook and group local product images. No source edits."""
import hashlib
import json
import re
from pathlib import Path
import openpyxl

root = Path(__file__).resolve().parent.parent
source = root / '厂商信息/V0001_河北冀农农机具有限公司'
workbook = source / '01_企业基础/单厂商资料采集模板.xlsx'
wb = openpyxl.load_workbook(workbook, data_only=True)
fields = {str(row[1]): str(row[3] or '').strip() for row in wb['厂商基础信息'].iter_rows(min_row=5, values_only=True) if row[1]}
description = '\n\n'.join(line.strip() for line in fields['企业简介'].splitlines() if line.strip())
description += '\n\n企业信息\n' + '\n'.join(f'{label}：{fields[label].rstrip(",，")}' for label in ['厂商简称', '厂商类型', '成立年份', '员工规模', '厂房面积', '年产能/供货能力', '联系人', '官网URL', '服务优势'] if fields.get(label))
description += '\n\n区域业务联系\n' + '\n'.join(line.strip() for line in fields['服务区域'].splitlines() if line.strip())
relative = lambda path: path.relative_to(source).as_posix()
factory = source / '03_品牌与厂房图片/厂房门店封面图文件名'
vendor = {'data': {'name': fields['厂商名称'], 'main_products': fields['主营产品'], 'region_name': ' '.join(fields[k] for k in ['省份', '城市', '区县']), 'phone': fields['联系电话'], 'address': fields['详细地址'], 'description': description, 'keywords': '冀农 冀丰 翻转犁 农机具'}, 'media_paths': [relative(p) for p in sorted(factory.glob('*')) if p.name.startswith('53b3')] + [relative(p) for p in sorted(factory.glob('*')) if p.name.startswith('03340')] + ['03_品牌与厂房图片/logo/logo.png']}
directory = source / '04_产品资料/P001_产品名称_型号/02_产品图片'
groups = {}
for path in sorted(directory.glob('*.jpg')):
    if path.name == '3米驱动耙-参数.jpg':
        key = '液压翻转犁1LF-440'  # Image itself identifies 1LF-440, despite its filename.
    else:
        key = re.sub(r'-[1-3]$', '', path.stem)
    groups.setdefault(key, []).append(path)
products = []
for name, paths in groups.items():
    model = re.search(r'1[A-Z]+-\d+', name)
    parameters = ('型号：' + model.group() + '\n' if model else '') + '品牌：冀丰'
    if name == '液压翻转犁1LF-440':
        parameters += '\n外形尺寸：3505 × 1880 × 1775 mm\n配套动力：140–160 马力\n重量：900 kg\n耕幅：160 cm\n耕深：20–35 cm'
    data = {'name': name, 'region_name': vendor['data']['region_name'], 'keywords': name + ' 冀农 冀丰 耕整地', 'parameters': parameters}
    if name == '液压翻转犁1LF-360':
        data['description'] = '产品资料来源：https://www.hbjinong.com/product/48.html'
    products.append({'data': data, 'category': '驱动耙' if '驱动耙' in name else '翻转犁', 'media_paths': [relative(p) for p in sorted(paths, key=lambda p: (p.name.endswith('-参数.jpg'), p.name))]})
manifest = {'source_key': 'V0001', 'source_root': str(source), 'vendor': vendor, 'products': products}
# Fingerprint includes every imported image as well as the normalized workbook data.
manifest['source_hash'] = hashlib.sha256(json.dumps(manifest, ensure_ascii=False, sort_keys=True).encode() + b''.join(hashlib.sha256((source / p).read_bytes()).digest() for item in [vendor] + products for p in item['media_paths'])).hexdigest()
output = root / '.local/imports/V0001.json'
output.parent.mkdir(parents=True, exist_ok=True)
output.write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding='utf-8')
print(json.dumps({'manifest': str(output), 'products': len(products), 'images': sum(len(p['media_paths']) for p in [vendor] + products)}, ensure_ascii=False))
