"""Export public filing information from apksigner's VERIFIED APK certificate.
No keystore, signing password, or private key is read or copied.
"""
from pathlib import Path
from datetime import datetime, timezone
import hashlib
import json
import re
from cryptography import x509
from cryptography.hazmat.primitives import serialization

ROOT = Path(__file__).resolve().parents[1]
VERSION = '1.5.14+22'
APK = ROOT / f'delivery/android/dalunongji-{VERSION}.apk'
REPORT = ROOT / 'docs/test-output/filing-apk-certificate.txt'
text = REPORT.read_text(encoding='utf-8-sig')
assert text.count('-----BEGIN CERTIFICATE-----') == 1
pem = re.search(r'-----BEGIN CERTIFICATE-----.*?-----END CERTIFICATE-----', text, re.S).group().encode()
cert = x509.load_pem_x509_certificate(pem)
der = cert.public_bytes(serialization.Encoding.DER)
fingerprint = hashlib.sha256(der).hexdigest()
assert fingerprint == '85656c85541b188c832dc48d67cbf320046a4311aa9e0fefcc662dbe2e0f1585'
public = cert.public_key()
numbers = public.public_numbers()
md5 = hashlib.md5(der).hexdigest().upper()
assert len(md5) == 32
assert f'certificate MD5 digest: {md5.lower()}' in text
package = 'com.zqzt.dalunongji'
assert f"name='{package}' versionCode='22' versionName='1.5.14'" in (ROOT / 'docs/test-output/filing-apk-metadata.txt').read_text(encoding='utf-8-sig')
out = ROOT / 'delivery/filing-v1.5.14'
out.mkdir(parents=True, exist_ok=True)
values = {'package-name.txt': package, 'certificate-md5.txt': md5,
          'public-key-modulus-decimal.txt': str(numbers.n), 'public-key-modulus-hex.txt': format(numbers.n, 'X')}
for filename, value in values.items():
    (out / filename).write_text(value + '\n', encoding='utf-8')
(out / 'signing-certificate.pem').write_bytes(cert.public_bytes(serialization.Encoding.PEM))
(out / 'public-key.pem').write_bytes(public.public_bytes(serialization.Encoding.PEM, serialization.PublicFormat.SubjectPublicKeyInfo))
(out / 'public-key.der').write_bytes(public.public_bytes(serialization.Encoding.DER, serialization.PublicFormat.SubjectPublicKeyInfo))
icon = ROOT / 'mobile/assets/branding/filing-icon-original.png'
(out / 'filing-icon-original.png').write_bytes(icon.read_bytes())
data = {'applicationName': '大陆农机', 'androidVersion': VERSION, 'packageName': package,
        'publicKeyFormatForAliyun': 'RSA modulus, decimal (same as Jadx GUI Modulus)',
        'rsaModulusDecimal': str(numbers.n), 'rsaModulusHex': format(numbers.n, 'X'),
        'rsaExponent': numbers.e, 'rsaBits': public.key_size, 'certificateMD5': md5,
        'certificateMD5ColonSeparated': ':'.join(md5[i:i+2] for i in range(0, 32, 2)),
        'certificateSHA256': fingerprint, 'apkSHA256': hashlib.sha256(APK.read_bytes()).hexdigest(),
        'iconSHA256': hashlib.sha256(icon.read_bytes()).hexdigest(), 'createdAt': datetime.now(timezone.utc).isoformat(),
        'apiBaseUrl': 'http://39.97.250.244:8089', 'websiteVersion': '1.5.13', 'backendVersion': '1.5.13',
        'privateKeyIncluded': False, 'signingCertificateUnchanged': True,
        'source': 'verified release APK certificate exported by apksigner --print-certs-pem',
        'reference': 'https://help.aliyun.com/zh/icp-filing/basic-icp-service/user-guide/fill-in-app-feature-information'}
(out / 'filing-info.json').write_text(json.dumps(data, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
doc = f'''# 大陆农机 Android 备案提交信息

本资料从已验证签名的 Android **{VERSION}** APK 提取。包名和签名证书与原版一致，仅更换为用户提供的备案图标并提升 Android 版本。网站、后台仍为 1.5.13。

| 阿里云字段 | 填写内容 |
| --- | --- |
| App 名称 | 大陆农机 |
| 安卓平台软件包名称 | `{package}` |
| 证书 MD5 指纹 | `{md5}` |
| 公钥 | 完整复制下方 RSA 模数，或 `public-key-modulus-decimal.txt` 的一整行 |

## 公钥（RSA 模数，十进制）

```text
{numbers.n}
```

该数值与阿里云官方方法中 Jadx GUI 的“模数（公钥）”一致。RSA 位数：{public.key_size}；指数：{numbers.e}。十六进制等值另附 `public-key-modulus-hex.txt`。`public-key.pem` / `public-key.der` 为标准 SubjectPublicKeyInfo 格式的公钥文件，供技术核验；填写上述模数字段时使用模数纯文本。

## 指纹与文件校验

证书 MD5（无分隔符）：`{md5}`

证书 MD5（冒号格式）：`{data['certificateMD5ColonSeparated']}`

证书 SHA-256：`{fingerprint}`

APK SHA-256：`{data['apkSHA256']}`

MD5 取自签名证书，不能用 APK 文件本身的 MD5 替代。备案图标 `filing-icon-original.png` 与用户附件逐字节一致；Android 仅为显示尺寸进行缩放，并由系统启动器应用图标遮罩。APK 源图摘要：`{data['iconSHA256']}`。

## 填写依据

[阿里云：填写 App 特征信息]({data['reference']})：安卓数字摘要填写证书 MD5（十六进制），可用 Jadx 的模数（公钥）和签名 MD5，或使用备案智能助理上传 APK 自动识别。MD5 大小写均可。包名取自 APK AndroidManifest。

[Jadx 公钥显示实现](https://github.com/skylot/jadx/blob/master/jadx-gui/src/main/java/jadx/gui/utils/CertificateManager.java)：RSA Modulus 使用十进制显示。

本包仅含可公开的公钥和证书，不含私钥、签名密钥库或任何密码。
'''
(out / '备案填写信息.md').write_text(doc, encoding='utf-8')
print(json.dumps({key: data[key] for key in ['packageName', 'certificateMD5', 'certificateSHA256', 'rsaBits', 'signingCertificateUnchanged']}, indent=2))
