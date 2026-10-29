import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'mobile_upgrade_test.dart' as f;

class RoutedAuthCapture extends StatefulWidget {
  final Widget page;
  const RoutedAuthCapture({super.key, required this.page});
  @override
  State<RoutedAuthCapture> createState() => _RoutedAuthCaptureState();
}

class _RoutedAuthCaptureState extends State<RoutedAuthCapture> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) {
        Navigator.push(
          context,
          MaterialPageRoute<void>(builder: (_) => widget.page),
        );
      }
    });
  }

  @override
  Widget build(BuildContext context) => const Scaffold();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  testWidgets('Phone forms layout, large text, keyboard and screenshots', (
    t,
  ) async {
    final capture = Platform.environment['PHONE_AUTH_CAPTURE'] == '1';
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
    final checks = <Json>[];
    for (final size in [
      const Size(360, 640),
      const Size(390, 844),
      const Size(768, 1024),
    ]) {
      t.view.physicalSize = size;
      t.view.devicePixelRatio = 1;
      for (final lang in ['zh', 'en']) {
        for (final scale in [1.0, 1.4, 1.6]) {
          for (final mode in [
            'merchant-login',
            'merchant-register',
            'visitor-login',
            'visitor-register',
            'binding',
          ]) {
            final m = await f.upgradeModel(signed: mode == 'binding');
            m.language = lang;
            final key = GlobalKey();
            await t.pumpWidget(
              f.upgradeApp(
                m,
                RoutedAuthCapture(
                  page: LoginScreen(companyAccount: false,
                    visitor: mode.startsWith('visitor'),
                    register: mode.endsWith('register'),
                    binding: mode == 'binding',
                  ),
                ),
                scale: scale,
                boundary: key,
              ),
            );
            await t.pumpAndSettle();
            expect(
              t.takeException(),
              isNull,
              reason: '$size $lang $scale $mode',
            );
            for (final field in t.widgetList<TextField>(
              find.byType(TextField),
            )) {
              expect(
                t.getSize(find.byKey(field.key!)).height,
                greaterThanOrEqualTo(48),
              );
            }
            expect(find.byType(ChoiceChip), findsNothing);
            if (capture && lang == 'zh' && scale == 1) {
              await t.runAsync(() async {
                final render =
                    key.currentContext!.findRenderObject()
                        as RenderRepaintBoundary;
                final img = await render.toImage(pixelRatio: 1.5);
                final bytes = await img.toByteData(
                  format: ui.ImageByteFormat.png,
                );
                final target = File(
                  '../docs/screenshots/phone-auth/$mode-${size.width.toInt()}.png',
                );
                target.parent.createSync(recursive: true);
                target.writeAsBytesSync(bytes!.buffer.asUint8List());
                img.dispose();
              });
            }
            checks.add({
              'page': mode,
              'width': size.width,
              'language': lang,
              'scale': scale,
              'keyboard': false,
              'passed': true,
            });
            t.view.viewInsets = const FakeViewPadding(bottom: 280);
            await t.pumpAndSettle();
            await t.ensureVisible(find.byType(FilledButton).first);
            await t.pumpAndSettle();
            expect(
              find.byType(FilledButton).first.hitTestable(),
              findsOneWidget,
            );
            expect(
              t.takeException(),
              isNull,
              reason: 'keyboard $size $lang $scale $mode',
            );
            checks.add({
              'page': mode,
              'width': size.width,
              'language': lang,
              'scale': scale,
              'keyboard': true,
              'passed': true,
            });
            t.view.resetViewInsets();
            await t.pumpWidget(const SizedBox());
            m.dispose();
          }
        }
      }
    }
    t.view.resetPhysicalSize();
    t.view.resetDevicePixelRatio();
    if (capture) {
      File('../docs/test-output/phone-native-layout.json').writeAsStringSync(
        const JsonEncoder.withIndent('  ').convert({
          'checks': checks.length,
          'syntheticFixtures': true,
          'realDevice': false,
          'layouts': checks,
        }),
      );
    }
  });
}
