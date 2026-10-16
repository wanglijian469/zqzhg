import 'dart:io';
import 'dart:convert';
import 'dart:ui' as ui;
import 'package:dalunongji/main.dart';
import 'package:http/http.dart' as http;
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'app_navigation_test.dart' as nav;

void main() {
  testWidgets('Capture home upgrade evidence', (tester) async {
    await tester.runAsync(() async {
      for (final entry in {
        'NavigationQA': 'C:/Windows/Fonts/msyh.ttc',
        'MaterialIcons':
            '../.tools/flutter/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
      }.entries) {
        final loader = FontLoader(entry.key);
        loader.addFont(
          Future.value(
            ByteData.sublistView(File(entry.value).readAsBytesSync()),
          ),
        );
        await loader.load();
      }
    });
    final stage = Platform.environment['UPGRADE_CAPTURE']!;
    final Json? live = stage == 'live' || stage == 'landmark'
        ? asMap(
            jsonDecode(
              File(
                '../.local/home-upgrade-live-fixture.json',
              ).readAsStringSync(),
            ),
          )
        : null;
    if (live != null) {
      final cache = PaintingBinding.instance.imageCache;
      final originalLimit = cache.maximumSizeBytes;
      cache.maximumSizeBytes = 256 * 1024 * 1024;
      addTearDown(() => cache.maximumSizeBytes = originalLimit);
      await tester.runAsync(() async {
        for (final file in Directory(
          '../.local/home-upgrade-images',
        ).listSync().whereType<File>()) {
          final id = file.uri.pathSegments.last.split('.').first;
          final codec = await ui.instantiateImageCodec(
            file.readAsBytesSync(),
            targetWidth: id == live['home']['hero']?['media_id']?.toString()
                ? 900
                : 400,
          );
          final frame = await codec.getNextFrame();
          PaintingBinding.instance.imageCache.putIfAbsent(
            NetworkImage('http://fixture/api/v1/media/$id?thumb=1'),
            () => OneFrameImageStreamCompleter(
              Future.value(ImageInfo(image: frame.image.clone())),
            ),
          );
          if (id == live['home']['hero']?['media_id']?.toString()) {
            cache.putIfAbsent(
              NetworkImage('http://fixture/api/v1/media/$id'),
              () => OneFrameImageStreamCompleter(
                Future.value(ImageInfo(image: frame.image.clone())),
              ),
            );
          }
          frame.image.dispose();
          codec.dispose();
        }
      });
    }
    for (final width in [360.0, 390.0, 412.0]) {
      tester.view.physicalSize = ui.Size(width, 844);
      tester.view.devicePixelRatio = 1;
      final m = await nav.fixture(
        signed: false,
        intercept: (r) async {
          if (live == null) return null;
          final key = r.url.path.split('/').last;
          return live.containsKey(key)
              ? http.Response(
                  jsonEncode(live[key]),
                  200,
                  headers: {'content-type': 'application/json; charset=utf-8'},
                )
              : null;
        },
      );
      final key = GlobalKey();
      await tester.pumpWidget(nav.harness(m, boundary: key));
      await tester.pumpAndSettle();
      Future<void> capture(String name) async {
        expect(tester.takeException(), isNull);
        final render =
            key.currentContext!.findRenderObject() as RenderRepaintBoundary;
        await tester.runAsync(() async {
          final img = await render.toImage(pixelRatio: 1.5);
          final bytes = await img.toByteData(format: ui.ImageByteFormat.png);
          File(
            '../docs/screenshots/home-upgrade-$stage-${width.toInt()}-$name.png',
          ).writeAsBytesSync(bytes!.buffer.asUint8List());
          img.dispose();
        });
      }

      await capture('home');
      if (stage == 'landmark') {
        await nav.tap(tester, find.byKey(const ValueKey('home-hall-selector')));
        await capture('hall-menu');
        await nav.tap(
          tester,
          find.byKey(const ValueKey('hall-option-cablesPreview')),
        );
        await capture('cables-home');
        await nav.tap(tester, find.byKey(const ValueKey('app-tab-1')));
        await capture('cables-manufacturers');
        await nav.tap(tester, find.byKey(const ValueKey('return-agriculture')));
        await nav.tap(tester, find.byKey(const ValueKey('app-tab-0')));
      }
      await nav.tap(
        tester,
        find.byKey(ValueKey('app-tab-${stage == 'before' ? 0 : 1}')),
      );
      await capture('manufacturers');
      if (stage == 'live' || stage == 'landmark') {
        await nav.tap(tester, find.byKey(const ValueKey('app-tab-2')));
        await capture('products');
        await nav.tap(tester, find.byKey(const ValueKey('app-tab-0')));
        await tester.drag(
          find.byType(CustomScrollView).first,
          const Offset(0, -520),
        );
        await tester.pumpAndSettle();
        await capture('recommended');
        if (width == 390) {
          await m.toggleLanguage();
          await tester.pumpWidget(nav.harness(m, boundary: key, scale: 1.4));
          await tester.pumpAndSettle();
          await capture('english-large');
          await tester.drag(
            find.byType(CustomScrollView).first,
            const Offset(0, 10000),
          );
          await tester.pumpAndSettle();
          await capture('english-home-large');
          if (stage == 'landmark') {
            final homeScroll = find
                .descendant(
                  of: find.byType(CustomScrollView).first,
                  matching: find.byType(Scrollable),
                )
                .first;
            tester.state<ScrollableState>(homeScroll).position.jumpTo(0);
            await tester.pumpAndSettle();
            await capture('english-home-large');
            await tester.pumpWidget(nav.harness(m, boundary: key, scale: 2));
            await tester.pumpAndSettle();
            await capture('english-home-largest');
            await m.toggleLanguage();
            await tester.pumpAndSettle();
            await capture('chinese-home-largest');
          }
        }
      }
      await tester.pumpWidget(const SizedBox());
    }
    tester.view.resetPhysicalSize();
    tester.view.resetDevicePixelRatio();
  }, skip: Platform.environment['UPGRADE_CAPTURE'] == null);
}
