import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'mobile_upgrade_test.dart' as fixture;

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  testWidgets('Native entry, company, product stages and map layout matrix', (
    t,
  ) async {
    final capture = Platform.environment['APP_V18_CAPTURE'] == '1';
    if (capture) {
      await t.runAsync(() async {
        for (final (family, file) in [
          ('MobileAcceptance', 'C:/Windows/Fonts/msyh.ttc'),
          (
            'MaterialIcons',
            '../.tools/flutter/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
          ),
        ]) {
          final loader = FontLoader(family);
          loader.addFont(
            Future.value(ByteData.sublistView(File(file).readAsBytesSync())),
          );
          await loader.load();
        }
      });
    }
    final checks = <Map<String, dynamic>>[];
    for (final size in [
      const Size(360, 640),
      const Size(375, 812),
      const Size(390, 844),
      const Size(414, 896),
      const Size(768, 1024),
      const Size(844, 390),
    ]) {
      t.view.physicalSize = size;
      t.view.devicePixelRatio = 1;
      for (final language in ['zh', 'en']) {
        for (final scale in [1.0, 1.4]) {
          for (final mode in ['register', 'login', 'company', 'product']) {
            final m = await fixture.upgradeModel(
              signed: mode == 'company' || mode == 'product',
            );
            m.language = language;
            final boundary = GlobalKey();
            final page = mode == 'register'
                ? const LoginScreen(register: true)
                : mode == 'login'
                ? const LoginScreen()
                : EditorScreen(kind: mode == 'company' ? 'vendor' : 'product');
            await t.pumpWidget(
              fixture.upgradeApp(m, page, scale: scale, boundary: boundary),
            );
            await t.pumpAndSettle();
            expect(
              t.takeException(),
              isNull,
              reason: '$size $language $scale $mode',
            );
            checks.add({
              'width': size.width,
              'height': size.height,
              'language': language,
              'scale': scale,
              'page': mode,
              'passed': true,
            });
            if (mode == 'product') {
              for (final stage in [1, 2]) {
                await fixture.tapReachable(
                  t,
                  find.textContaining(
                    stage == 1
                        ? (language == 'zh' ? '2. 图片' : '2. Images')
                        : (language == 'zh' ? '3. 预览' : '3. Preview'),
                  ),
                );
                expect(
                  t.takeException(),
                  isNull,
                  reason: '$size $language $scale product stage $stage',
                );
                checks.add({
                  'width': size.width,
                  'height': size.height,
                  'language': language,
                  'scale': scale,
                  'page': 'product-$stage',
                  'passed': true,
                });
              }
            }
            if (capture &&
                language == 'zh' &&
                scale == 1 &&
                [360.0, 390.0, 768.0].contains(size.width)) {
              await t.runAsync(() async {
                final render =
                    boundary.currentContext!.findRenderObject()
                        as RenderRepaintBoundary;
                final img = await render.toImage(pixelRatio: 1.5);
                final bytes = await img.toByteData(
                  format: ui.ImageByteFormat.png,
                );
                final target = File(
                  '../docs/screenshots/app-v1.8/$mode-${size.width.toInt()}.png',
                );
                target.parent.createSync(recursive: true);
                target.writeAsBytesSync(bytes!.buffer.asUint8List());
                img.dispose();
              });
            }
            // Short viewports emulate a raised keyboard; forms retain a natural scroll.
            if (mode == 'register' &&
                size.width == 360 &&
                language == 'zh' &&
                scale == 1) {
              t.view.physicalSize = const Size(360, 360);
              await t.pumpAndSettle();
              await fixture.tapReachable(
                t,
                find.widgetWithText(FilledButton, '注册并继续'),
              );
              expect(t.takeException(), isNull);
              t.view.physicalSize = size;
            }
            await t.pumpWidget(const SizedBox());
            m.dispose();
          }
        }
      }
    }
    t.view.resetPhysicalSize();
    t.view.resetDevicePixelRatio();
    if (capture) {
      File('../docs/test-output/mobile18-native-layout.json').writeAsStringSync(
        const JsonEncoder.withIndent('  ').convert({
          'checks': checks.length,
          'syntheticFixtures': true,
          'realDevice': false,
          'boundariesAvailable': false,
          'layouts': checks,
        }),
      );
    }
  });
}
