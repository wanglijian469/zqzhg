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
  final enabled = Platform.environment['VENDOR_CATEGORY_CAPTURE'] == '1';
  setUpAll(() async {
    if (enabled) {
      final bytes = await File('C:/Windows/Fonts/msyh.ttc').readAsBytes();
      final font = FontLoader('MobileAcceptance')
        ..addFont(Future.value(ByteData.sublistView(bytes)));
      await font.load();
      final icons = FontLoader('MaterialIcons')
        ..addFont(
          File(
            '../.tools/flutter/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
          ).readAsBytes().then(ByteData.sublistView),
        );
      await icons.load();
    }
  });
  for (final width in [390.0, 768.0]) {
    testWidgets('capture company category management $width', (t) async {
      t.view.physicalSize = Size(width, 1000);
      t.view.devicePixelRatio = 1;
      addTearDown(t.view.resetPhysicalSize);
      addTearDown(t.view.resetDevicePixelRatio);
      final m = await fixture.upgradeModel(
        signed: true,
        handler: (r) async => r.url.path.endsWith('/product-categories')
            ? fixture.jsonResponse({
                'items': [
                  {
                    'id': 51,
                    'vendor_id': 21,
                    'name': '液压回转马达',
                    'name_en': 'Hydraulic rotary motors',
                    'industry_category_id': 13,
                    'industry_name': '液压泵、阀、输出装置零部件',
                    'enabled': true,
                    'sort_order': 0,
                  },
                  {
                    'id': 52,
                    'vendor_id': 21,
                    'name': '液压阀组',
                    'industry_category_id': 13,
                    'industry_name': '液压泵、阀、输出装置零部件',
                    'enabled': true,
                    'sort_order': 10,
                  },
                ],
                'industry_categories': [
                  {'id': 13, 'name': '液压泵、阀、输出装置零部件'},
                ],
                'preferred_industry_ids': [13],
              })
            : null,
      );
      final boundary = GlobalKey();
      await t.pumpWidget(
        fixture.upgradeApp(
          m,
          const VendorProductCategoriesScreen(),
          boundary: boundary,
        ),
      );
      await t.pumpAndSettle();
      Future<void> capture(String label) async {
        await t.runAsync(() async {
          final image =
              await (boundary.currentContext!.findRenderObject()!
                      as RenderRepaintBoundary)
                  .toImage(pixelRatio: 1);
          final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
          final folder = Directory(
            '../docs/screenshots/vendor-product-categories-20261115',
          );
          await folder.create(recursive: true);
          await File(
            '${folder.path}/app-$label-${width.toInt()}-zh.png',
          ).writeAsBytes(bytes!.buffer.asUint8List());
          image.dispose();
        });
      }

      await capture('manager');
      await fixture.tapReachable(t, find.text('编辑').first);
      await capture('edit-category');
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    }, skip: !enabled);
  }
}
