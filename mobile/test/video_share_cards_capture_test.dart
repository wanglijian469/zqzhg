import 'dart:io';
import 'dart:ui' as ui;
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'mobile_upgrade_test.dart' as f;

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  final enabled = Platform.environment['VIDEO_SHARE_CAPTURE'] == '1';
  setUpAll(() async {
    if (!enabled) {
      return;
    }
    await (FontLoader('MobileAcceptance')..addFont(
          File(
            'C:/Windows/Fonts/msyh.ttc',
          ).readAsBytes().then(ByteData.sublistView),
        ))
        .load();
    await (FontLoader('MaterialIcons')..addFont(
          File(
            '../.tools/flutter/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
          ).readAsBytes().then(ByteData.sublistView),
        ))
        .load();
  });
  for (final width in [390.0, 768.0]) {
    testWidgets('capture free share cards and source choices $width', (
      t,
    ) async {
      t.view.physicalSize = Size(width, 1100);
      t.view.devicePixelRatio = 1;
      addTearDown(t.view.resetPhysicalSize);
      addTearDown(t.view.resetDevicePixelRatio);
      final m = await f.upgradeModel(
        signed: true,
        handler: (r) async => r.url.path.endsWith('/public/config')
            ? f.jsonResponse({
                'capabilities': {'video_api_version': 4, 'douyin_video': true},
              })
            : null,
      );
      final boundary = GlobalKey();
      Future<void> capture(String name) async {
        await t.runAsync(() async {
          final image =
              await (boundary.currentContext!.findRenderObject()!
                      as RenderRepaintBoundary)
                  .toImage(pixelRatio: 1);
          final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
          final dir = Directory(
            '../docs/screenshots/video-share-cards-20261116',
          );
          await dir.create(recursive: true);
          await File(
            '${dir.path}/app-$name-${width.toInt()}-zh.png',
          ).writeAsBytes(bytes!.buffer.asUint8List());
          image.dispose();
        });
      }

      final cards = [
        for (final source in ['WECHAT_CHANNELS', 'KUAISHOU'])
          NativePublicResourceCard(
            item: {
              'id': 8,
              'kind': 'video',
              'vendor_id': 21,
              'data': {
                'video_source': source,
                'title': source == 'KUAISHOU' ? '产品加工现场展示' : '企业展会与产品介绍',
                'description': '分享企业真实的产品与生产现场。',
                'canonical_video_url': source == 'KUAISHOU'
                    ? 'https://v.kuaishou.com/Abc123'
                    : 'https://weixin.qq.com/sph/Aht6MMIJPm',
                'purpose': 'company',
                'file_media_id': 0,
                'cover_media_id': 0,
                'page_media_ids': <int>[],
              },
            },
          ),
      ];
      await t.pumpWidget(
        f.upgradeApp(
          m,
          Scaffold(
            appBar: AppBar(title: const Text('企业视频')),
            body: ListView(padding: const EdgeInsets.all(16), children: cards),
          ),
          boundary: boundary,
        ),
      );
      await t.pumpAndSettle();
      await capture('cards');
      await t.pumpWidget(const SizedBox());
      await t.pumpWidget(
        f.upgradeApp(m, const NativeVideoAddScreen(), boundary: boundary),
      );
      await t.pumpAndSettle();
      await capture('sources');
      await f.tapReachable(t, find.text('微信视频号分享链接'));
      await capture('form');
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    }, skip: !enabled);
  }
}
