import 'dart:io';
import 'dart:ui' as ui;
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'app_navigation_test.dart' as f;

void main() {
  for (final scale in [1.0, 2.0]) {
    testWidgets('vendor actions stay compact and readable at text scale $scale', (
      tester,
    ) async {
      tester.view.physicalSize = const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final font = File('C:/Windows/Fonts/msyh.ttc');
      final icons = File(
        '../.tools/flutter/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
      );
      if (icons.existsSync()) {
        final loader = FontLoader(
          'MaterialIcons',
        )..addFont(Future.value(ByteData.sublistView(icons.readAsBytesSync())));
        await loader.load();
      }
      if (font.existsSync()) {
        final loader = FontLoader('NavigationQA')
          ..addFont(Future.value(ByteData.sublistView(font.readAsBytesSync())));
        await loader.load();
      }
      final m = await f.fixture(signed: false);
      const boundary = ValueKey('vendor-visual');
      await tester.pumpWidget(
        f.harness(
          m,
          scale: scale,
          boundary: boundary,
          home: Scaffold(
            appBar: AppBar(title: const Text('找厂家')),
            body: ListView(
              padding: const EdgeInsets.all(12),
              children: [
                for (var i = 0; i < 3; i++)
                  NativeVendorCard(item: {...f.company(), 'id': i + 1}),
              ],
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      final primary = find.byKey(const ValueKey('showroom-1'));
      final secondary = find.byKey(const ValueKey('contact-vendor-1'));
      expect(tester.getSize(primary).height, inInclusiveRange(40, 44));
      expect(tester.getSize(secondary).height, inInclusiveRange(40, 44));
      final buttonContext = tester.element(primary);
      expect(
        Theme.of(buttonContext).colorScheme.primary,
        const Color(0xff1769e0),
      );
      final contact = tester.widget<OutlinedButton>(secondary);
      expect(contact.style?.backgroundColor?.resolve({}), isNot(Colors.white));
      expect(tester.takeException(), isNull);
      if (scale == 1) {
        expect(
          tester.getSize(find.byType(NativeVendorCard).first).height,
          lessThan(190),
        );
        await tester.tap(find.byKey(const ValueKey('favorite-vendor-1')));
        await tester.pumpAndSettle();
        expect(
          tester
              .widget<IconButton>(
                find.byKey(const ValueKey('favorite-vendor-1')),
              )
              .isSelected,
          isTrue,
        );
        final box = tester.renderObject<RenderRepaintBoundary>(
          find.byKey(boundary),
        );
        await tester.runAsync(() async {
          final img = await box.toImage(pixelRatio: 1);
          final data = await img.toByteData(format: ui.ImageByteFormat.png);
          final out = Directory('../.local/vendor-visual');
          out.createSync(recursive: true);
          File(
            '${out.path}/app-390.png',
          ).writeAsBytesSync(data!.buffer.asUint8List());
          img.dispose();
        });
      }
      await tester.pumpWidget(const SizedBox());
    });
  }
}
