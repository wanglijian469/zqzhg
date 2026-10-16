# 大陆农机 Android 1.5.14+22：备案图标更新

使用用户提供的绿色农机备案图标；包名 com.zqzt.dalunongji、原签名和云端地址 http://39.97.250.244:8089 保留。网站及后端仍为 1.5.13。

## 手机安装

安装 dalunongji-1.5.14+22.apk，直接覆盖旧版，不要卸载。原登录、语言、服务地址、本机草稿按原应用逻辑保留。系统启动器有图标缓存时，可先重启手机或等待启动器刷新，勿为刷新图标而清空应用数据。本次没有连接的 Android 实机或模拟器，覆盖安装和桌面图标实机显示尚未验证。

## 备案资料

filing/备案填写信息.md 包含三项备案字段及复制说明；公钥十进制模数见 filing/public-key-modulus-decimal.txt。证书 MD5 见 filing/certificate-md5.txt。该 MD5 属于签名证书。签名证书和公钥可公开，不含私钥、密钥库或密码。filing/filing-icon-original.png 与用户附件逐字节一致。APK 内原始图标和五种缩放图标经过像素核对。

## 更新阿里云手机下载安装链接

将 dalunongji-android-v1.5.14-update.tar.gz 上传到服务器 /opt，在 root 终端执行：

```bash
cd /opt && tar -xzf dalunongji-android-v1.5.14-update.tar.gz && bash /opt/dalunongji-android-v1.5.14/update-android.sh
```

脚本核对全包 SHA-256，将当前下载 APK 备份到 /opt/zqzt/shared/apk-archive，再原子替换当前程序目录的 web/dist/downloads/dalunongji.apk。不会重新导入数据库，也不需重启服务。服务器下载地址保持为 http://39.97.250.244:8089/downloads/dalunongji.apk。本次仅生成更新包，尚未替用户更新 ECS 上的 APK。

旧的 Linux 完整部署归档仍含当时版本的 APK；若未来从旧归档重新部署，需再次执行本更新。历史部署包和 APK 均保留。
