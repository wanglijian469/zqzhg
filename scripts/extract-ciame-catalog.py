"""Extract the supplied CIAME workbook without changing its contents."""
import hashlib
import json
from pathlib import Path

import openpyxl

root = Path(__file__).resolve().parents[1]
source = root / "需求文档/CIAME产品类别分类表.xlsx"
book = openpyxl.load_workbook(source, read_only=True, data_only=True)
groups = []
for row in book.worksheets[0].iter_rows(values_only=True):
    if not isinstance(row[0], int):
        continue
    if row[1]:
        groups.append({"name": row[1], "children": []})
    groups[-1]["children"].append(row[2])
assert len(groups) == 29
assert sum(len(g["children"]) for g in groups) == 159
catalog = {
    "title": "2026中国国际农业机械展览会（CIAME）产品类别分类",
    "source": "需求文档/CIAME产品类别分类表.xlsx",
    "sha256": hashlib.sha256(source.read_bytes()).hexdigest(),
    "primary_count": 29,
    "secondary_count": 159,
    "workbook_statistics": {"primary_count": 32, "secondary_count": 179},
    "groups": groups,
}
target = root / "server/catalog/ciame-2026.json"
target.write_text(json.dumps(catalog, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
print("CIAME catalog: 29 primary / 159 secondary")
