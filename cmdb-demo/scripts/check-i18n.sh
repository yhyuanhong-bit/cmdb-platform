#!/bin/bash
# Quick i18n health check
echo "=== i18n Health Check ==="
cd "$(dirname "$0")/.."

python3 -c "
import json,re
en=json.load(open('src/i18n/locales/en.json'))
cn=json.load(open('src/i18n/locales/zh-CN.json'))
tw=json.load(open('src/i18n/locales/zh-TW.json'))
ek=sum(len(v) for v in en.values() if isinstance(v,dict))
ck=sum(len(v) for v in cn.values() if isinstance(v,dict))
tk=sum(len(v) for v in tw.values() if isinstance(v,dict))
print(f'Keys: EN={ek} CN={ck} TW={tk}', '✅' if ek==ck==tk else '❌ MISMATCH')
ci=sum(1 for ns,v in en.items() if isinstance(v,dict) for k,val in v.items() if isinstance(val,str) and re.search(r'[\u4e00-\u9fff]',val))
print(f'Chinese in en.json: {ci}', '✅' if ci==0 else '❌')
"
echo "=== Done ==="
