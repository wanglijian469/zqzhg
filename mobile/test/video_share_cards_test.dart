import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
// ignore: depend_on_referenced_packages
import 'package:url_launcher_platform_interface/url_launcher_platform_interface.dart';
// ignore: depend_on_referenced_packages
import 'package:url_launcher_platform_interface/link.dart';
import 'mobile_upgrade_test.dart' as f;

class ShareCardLauncher extends UrlLauncherPlatform {
  final calls = <String>[];
  final modes = <PreferredLaunchMode>[];
  bool opened = true;
  @override
  LinkDelegate? get linkDelegate => null;
  @override
  Future<bool> launchUrl(String url, LaunchOptions options) async {
    calls.add(url);
    modes.add(options.mode);
    return opened;
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  const url = 'https://weixin.qq.com/sph/Aht6MMIJPm';
  Json item(String source) => {
    'id': 8,
    'kind': 'video',
    'vendor_id': 21,
    'data': {
      'video_source': source,
      'title': '企业分享视频',
      'title_en': 'Company shared video',
      'description': '视频简介',
      'canonical_video_url': source == 'KUAISHOU'
          ? 'https://v.kuaishou.com/Abc123'
          : url,
      'purpose': 'company',
      'file_media_id': 0,
      'cover_media_id': 0,
      'page_media_ids': <int>[],
    },
  };
  late ShareCardLauncher launcher;
  late UrlLauncherPlatform previous;
  setUp(() {
    previous = UrlLauncherPlatform.instance;
    launcher = ShareCardLauncher();
    UrlLauncherPlatform.instance = launcher;
  });
  tearDown(() {
    UrlLauncherPlatform.instance = previous;
  });

  test(
    'validates WeChat canonical links without accepting forged destinations',
    () {
      expect(
        trustedExternalVideo(
          url,
          kuaishou: false,
          source: 'WECHAT_CHANNELS',
        )?.toString(),
        url,
      );
      for (final raw in [
        url.replaceFirst('weixin.qq.com', 'weixin.qq.com.evil.test'),
        url.replaceFirst('https:', 'http:'),
        '$url?x=1',
        '$url#x',
        url.replaceFirst('/sph/A', '/sph/%41'),
        url.replaceFirst('.com/', '.com:443/'),
      ]) {
        expect(
          trustedExternalVideo(raw, kuaishou: false, source: 'WECHAT_CHANNELS'),
          isNull,
        );
      }
    },
  );
  for (final source in ['KUAISHOU', 'WECHAT_CHANNELS']) {
    testWidgets(
      '$source opens externally from its card without a player request or dialog',
      (t) async {
        final calls = <String>[];
        final m = await f.upgradeModel(
          handler: (r) async {
            calls.add(r.url.path);
            return null;
          },
        );
        await t.pumpWidget(
          f.upgradeApp(
            m,
            Scaffold(
              body: SingleChildScrollView(
                child: NativePublicResourceCard(item: item(source)),
              ),
            ),
          ),
        );
        await t.pumpAndSettle();
        await f.tapReachable(t, find.text(shareVideoWatchLabel(source, true)));
        expect(
          launcher.calls.single,
          asMap(item(source)['data'])['canonical_video_url'],
        );
        expect(launcher.modes.single, PreferredLaunchMode.externalApplication);
        expect(find.byType(AlertDialog), findsNothing);
        expect(find.byType(NativeResourceViewer), findsNothing);
        expect(
          calls.any(
            (p) =>
                p.contains('/playback') ||
                p.contains('resource-media') ||
                p.contains('resolve'),
          ),
          false,
        );
        expect(t.takeException(), isNull);
        await t.pumpWidget(const SizedBox());
      },
    );
  }
  testWidgets(
    'failed external opening preserves the card and provides copying',
    (t) async {
      launcher.opened = false;
      String? copied;
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(SystemChannels.platform, (call) async {
            if (call.method == 'Clipboard.setData') {
              copied = (call.arguments as Map)['text'] as String?;
            }
            return null;
          });
      addTearDown(
        () => TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
            .setMockMethodCallHandler(SystemChannels.platform, null),
      );
      final m = await f.upgradeModel();
      await t.pumpWidget(
        f.upgradeApp(
          m,
          Scaffold(
            body: SingleChildScrollView(
              child: NativePublicResourceCard(item: item('WECHAT_CHANNELS')),
            ),
          ),
        ),
      );
      await t.pumpAndSettle();
      await f.tapReachable(t, find.text('前往视频号观看'));
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.textContaining('暂时无法打开原视频'), findsOneWidget);
      await f.tapReachable(
        t,
        find.descendant(
          of: find.byType(AlertDialog),
          matching: find.text('复制链接'),
        ),
      );
      expect(copied, url);
      expect(find.byType(NativeResourceViewer), findsNothing);
      await f.tapReachable(t, find.widgetWithText(TextButton, m.t('close')));
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets('v4 adds WeChat and source checking uses the v4 API', (t) async {
    final writes = <Json>[];
    final m = await f.upgradeModel(
      signed: true,
      handler: (r) async {
        if (r.url.path.endsWith('/public/config')) {
          return f.jsonResponse({
            'capabilities': {'video_api_version': 4, 'douyin_video': true},
          });
        }
        if (r.url.path.endsWith('/merchant/video-links/check')) {
          expect(r.url.queryParameters['video_api_version'], '4');
          writes.add(asMap(jsonDecode(r.body)));
          return f.jsonResponse({
            'original_share_url': url,
            'check': {
              'source': 'WECHAT_CHANNELS',
              'video_id': 'sph:Aht6MMIJPm',
              'canonical_url': url,
              'status': 'EXTERNAL_ONLY',
            },
          });
        }
        return null;
      },
    );
    expect(await m.videoVersion(), 4);
    await t.pumpWidget(f.upgradeApp(m, const NativeVideoAddScreen()));
    await t.pumpAndSettle();
    expect(find.text('微信视频号分享链接'), findsOneWidget);
    await f.tapReachable(t, find.text('微信视频号分享链接'));
    final input = find.byWidgetPredicate(
      (w) => w is TextField && w.decoration?.labelText == '视频分享链接',
    );
    await t.enterText(input, url);
    await f.tapReachable(t, find.text('检查链接'));
    expect(writes.single['source'], 'WECHAT_CHANNELS');
    expect(find.text('链接格式正确，审核后访客可前往原平台观看。'), findsOneWidget);
    expect(find.text('上传视频'), findsNothing);
    await t.pumpWidget(const SizedBox());
  });
  for (final width in [360.0, 390.0, 768.0]) {
    for (final language in ['zh', 'en']) {
      testWidgets('long share card fits $width $language large text', (
        t,
      ) async {
        t.view.physicalSize = Size(width, 1200);
        t.view.devicePixelRatio = 1;
        addTearDown(t.view.resetPhysicalSize);
        addTearDown(t.view.resetDevicePixelRatio);
        final m = await f.upgradeModel();
        m.language = language;
        final value = item('WECHAT_CHANNELS');
        (value['data'] as Json)['title'] = List.filled(6, '企业生产现场与产品展示').join();
        (value['data'] as Json)['title_en'] = List.filled(
          6,
          'Company production and product demonstration ',
        ).join();
        await t.pumpWidget(
          f.upgradeApp(
            m,
            Scaffold(
              body: SingleChildScrollView(
                child: NativePublicResourceCard(item: value),
              ),
            ),
            scale: 1.6,
          ),
        );
        await t.pumpAndSettle();
        expect(t.takeException(), isNull);
        expect(
          find.text(shareVideoWatchLabel('WECHAT_CHANNELS', language == 'zh')),
          findsOneWidget,
        );
        await t.pumpWidget(const SizedBox());
      });
    }
  }
}
