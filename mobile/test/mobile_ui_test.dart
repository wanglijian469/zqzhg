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
  testWidgets('phone list and field layout matrix; filters keep business query', (
    t,
  ) async {
    final capture = Platform.environment['MOBILE_UI_CAPTURE'] == '1';
    if (capture) {
      await t.runAsync(() async {
        final font = FontLoader('MobileAcceptance');
        font.addFont(
          Future.value(
            ByteData.sublistView(
              await File('C:/Windows/Fonts/msyh.ttc').readAsBytes(),
            ),
          ),
        );
        await font.load();
        final icons = FontLoader('MaterialIcons');
        icons.addFont(
          Future.value(
            ByteData.sublistView(
              await File(
                '../.tools/flutter/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
              ).readAsBytes(),
            ),
          ),
        );
        await icons.load();
      });
    }
    final requests = <Uri>[];
    for (final width in [360.0, 390.0, 430.0]) {
      t.view.physicalSize = Size(width, 844);
      t.view.devicePixelRatio = 1;
      for (final language in ['zh', 'en']) {
        for (final scale in [1.0, 1.3, 1.5]) {
          for (final kind in ['product', 'vendor']) {
            final m = await f.upgradeModel(
              handler: (r) async {
                requests.add(r.url);
                if (r.url.path.endsWith('/taxonomy')) {
                  return f.jsonResponse({
                    'categories': [
                      {
                        'id': 101,
                        'name': '产品分类长名称示例',
                        'name_en': 'Agricultural machinery and equipment',
                        'parent_id': 0,
                      },
                    ],
                    'regions': [
                      {'id': 13, 'name': '河北省', 'name_en': 'Hebei'},
                    ],
                  });
                }
                if (r.url.path.endsWith('/products') ||
                    r.url.path.endsWith('/vendors')) {
                  return f.jsonResponse({
                    'items': [
                      for (var i = 1; i <= 5; i++)
                        {
                          'id': i,
                          'kind': kind,
                          'showroom_slug': 'fixture-$i',
                          'data': {
                            'name': '河北冀农农机具有限公司长名称与农用机械产品示例',
                            'name_en':
                                'Hebei Jinong Agricultural Machinery Manufacturing Company and Equipment',
                            'main_products': '液压翻转犁、旋耕机和农机配件',
                            'region_name': '河北省邢台市',
                            'model': 'JN-2026-A',
                            'media_ids': <int>[],
                          },
                          'vendor': {
                            'name': '河北冀农农机具有限公司',
                            'name_en': 'Hebei Jinong Agricultural Machinery',
                            'region_name': '河北省邢台市',
                          },
                        },
                    ],
                    'total': 5,
                    'page': 1,
                    'page_size': 20,
                  });
                }
                return null;
              },
            );
            m.language = language;
            final boundary = GlobalKey();
            await t.pumpWidget(
              f.upgradeApp(
                m,
                BrowseScreen(kind: kind),
                scale: scale,
                boundary: boundary,
              ),
            );
            await t.pumpAndSettle();
            expect(
              t.takeException(),
              isNull,
              reason: '$width $language $scale $kind',
            );
            expect(find.byType(NativeFilterPanel), findsNothing);
            if (width == 390 && language == 'zh' && scale == 1) {
              await t.tap(find.byKey(const ValueKey('browse-category-filter')));
              await t.pumpAndSettle();
              await t.tap(find.text('产品分类长名称示例').last);
              await t.pumpAndSettle();
              expect(
                requests
                    .lastWhere(
                      (u) => u.path.endsWith(
                        kind == 'product' ? '/products' : '/vendors',
                      ),
                    )
                    .queryParameters['category_id'],
                '101',
              );
              expect(t.takeException(), isNull);
            }
            if (capture && scale == 1 && width == 390) {
              final render =
                  boundary.currentContext!.findRenderObject()
                      as RenderRepaintBoundary;
              await t.runAsync(() async {
                final image = await render.toImage(pixelRatio: 1);
                final bytes = await image.toByteData(
                  format: ui.ImageByteFormat.png,
                );
                image.dispose();
                final file = File(
                  '../docs/screenshots/mobile-ui-20261024/$kind-${width.toInt()}-$language.png',
                );
                await file.parent.create(recursive: true);
                await file.writeAsBytes(bytes!.buffer.asUint8List());
              });
            }
            await t.pumpWidget(const SizedBox());
          }
        }
      }
    }
    t.view.resetPhysicalSize();
    t.view.resetDevicePixelRatio();
  });

  testWidgets(
    'showroom header element and request remain stable across all six tabs',
    (t) async {
      int details = 0;
      final m = await f.upgradeModel(
        handler: (r) async {
          if (r.url.path.contains('/showrooms/')) {
            details++;
            return f.jsonResponse({
              'item': {
                'id': 1,
                'hall_id': 1,
                'data': {
                  'name': '河北冀农农机具有限公司',
                  'main_products': '农用机械',
                  'description': '企业介绍',
                  'media_ids': <int>[],
                },
              },
              'categories': [],
              'resource_counts': {'video': 1, 'catalog': 1},
              'product_count': 1,
            });
          }
          return null;
        },
      );
      await t.pumpWidget(
        f.upgradeApp(m, const CompanyShowroomScreen(slug: 'jinong')),
      );
      await t.pumpAndSettle();
      final hero = t.element(find.byType(NativeCompanyHero));
      final scroll = t.element(find.byType(CustomScrollView).first);
      for (final title in ['企业介绍', '产品中心', '视频展示', '电子画册', '联系企业', '展厅首页']) {
        final tab = find.widgetWithText(ChoiceChip, title);
        await t.ensureVisible(tab);
        await t.pumpAndSettle();
        await t.tap(tab);
        await t.pumpAndSettle();
        expect(t.element(find.byType(NativeCompanyHero)), same(hero));
        expect(t.element(find.byType(CustomScrollView).first), same(scroll));
        expect(t.takeException(), isNull, reason: title);
      }
      expect(details, 1);
    },
  );
}
