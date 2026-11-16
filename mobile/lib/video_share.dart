part of 'main.dart';

String videoPlatformLabel(String source, bool zh) => switch (source) {
  'WECHAT_CHANNELS' => zh ? '微信视频号' : 'WeChat Channels',
  'KUAISHOU' => zh ? '快手' : 'Kuaishou',
  'DOUYIN' => zh ? '抖音' : 'Douyin',
  _ => zh ? '本地视频' : 'Local video',
};

String shareVideoWatchLabel(String source, bool zh) => zh
    ? '前往${source == 'WECHAT_CHANNELS' ? '视频号' : '快手'}观看'
    : 'Watch on ${videoPlatformLabel(source, false)}';

String shareVideoTip(String source, bool zh) => source == 'WECHAT_CHANNELS'
    ? (zh
          ? '此视频在微信视频号观看。如需使用微信，可复制链接后在微信内打开。'
          : 'Watch on WeChat Channels. If WeChat is required, copy the link and open it in WeChat.')
    : (zh ? '此视频在快手观看。' : 'Watch this video on Kuaishou.');

class NativeShareVideoActions extends StatelessWidget {
  final Json item;
  final String? url;
  final Future<void> Function()? onWatch;
  const NativeShareVideoActions({
    super.key,
    required this.item,
    this.url,
    this.onWatch,
  });

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context),
        data = asMap(item['data']),
        zh = m.language == 'zh';
    final source = (data['video_source'] ?? '').toString();
    final uri = trustedExternalVideo(
      url ?? (data['canonical_video_url'] ?? '').toString(),
      kuaishou: source == 'KUAISHOU',
      source: source,
    );
    if (uri == null) {
      return Text(
        zh
            ? '视频链接无效，请联系企业更新。'
            : 'Invalid video link. Please ask the company to update it.',
      );
    }
    return Wrap(
      spacing: 8,
      runSpacing: 8,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        OutlinedButton(
          onPressed: onWatch ?? () => openNativeResource(context, item),
          child: Text(shareVideoWatchLabel(source, zh)),
        ),
        TextButton(
          onPressed: () => copyNativeText(context, uri.toString()),
          child: Text(zh ? '复制链接' : 'Copy link'),
        ),
      ],
    );
  }
}
