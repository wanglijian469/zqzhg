import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'mobile_upgrade_test.dart' as fixture;

const categories = [
  {
    'id': 101,
    'hall_id': 1,
    'parent_id': 0,
    'name': '液压系统及零部件',
    'name_en': 'Hydraulics',
    'enabled': true,
  },
  {
    'id': 102,
    'hall_id': 1,
    'parent_id': 101,
    'name': '液压油缸',
    'name_en': 'Hydraulic cylinders',
    'enabled': true,
    'search_aliases': '油缸',
  },
  {
    'id': 103,
    'hall_id': 1,
    'parent_id': 101,
    'name': '液压阀',
    'name_en': 'Hydraulic valves',
    'enabled': true,
  },
];

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  testWidgets(
    'Product category search uses full paths and leaf selection; recommendations require confirmation',
    (tester) async {
      final m = await fixture.upgradeModel(
        signed: true,
        handler: (r) async {
          if (r.url.path.endsWith('/taxonomy')) {
            return fixture.jsonResponse({
              'categories': categories,
              'regions': [],
              'aliases': [],
            });
          }
          if (r.url.path.endsWith('/category-recommendations')) {
            return fixture.jsonResponse({
              'items': [
                {
                  'id': 103,
                  'path': '液压系统及零部件 / 液压阀',
                  'path_en': 'Hydraulics / Hydraulic valves',
                },
              ],
            });
          }
          return null;
        },
      );
      Json data = {
        'name': '液压油缸',
        'product_category_id': 102,
        'category_id': 0,
      };
      await tester.pumpWidget(
        fixture.upgradeApp(
          m,
          Scaffold(
            body: SingleChildScrollView(
              child: StatefulBuilder(
                builder: (context, change) => NativeTaxonomyFields(
                  data: data,
                  kind: 'product',
                  hallID: 1,
                  enabled: true,
                  onChanged: (patch) =>
                      change(() => data = {...data, ...patch}),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      final dropdown = tester.widget<DropdownButton<int>>(
        find.byType(DropdownButton<int>),
      );
      expect(
        dropdown.items!.firstWhere((v) => v.value == 101).enabled,
        isFalse,
      );
      await fixture.tapReachable(
        tester,
        find.widgetWithText(OutlinedButton, '推荐分类'),
      );
      expect(data['product_category_id'], 102);
      await tester.tap(find.text('液压系统及零部件 / 液压阀').last);
      await tester.pumpAndSettle();
      expect(data['product_category_id'], 103);
      await fixture.tapReachable(
        tester,
        find.widgetWithText(OutlinedButton, '搜索分类'),
      );
      await tester.enterText(find.widgetWithText(TextField, '分类名称或别名'), '油缸');
      await tester.pumpAndSettle();
      await tester.tap(find.text('液压系统及零部件 / 液压油缸').last);
      await tester.pumpAndSettle();
      expect(data['product_category_id'], 102);
      expect(nativeCategoryPath(categories, 102, m), '液压系统及零部件 / 液压油缸');
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Brand and multiple applicable models update the draft without changing raw parameters',
    (tester) async {
      final m = await fixture.upgradeModel(
        signed: true,
        handler: (r) async => r.url.path.endsWith('/taxonomy')
            ? fixture.jsonResponse({'categories': categories})
            : null,
      );
      Json data = {'name': '油缸', 'category_id': 0, 'parameters': '旧参数原文'};
      await tester.pumpWidget(
        fixture.upgradeApp(
          m,
          Scaffold(
            body: SingleChildScrollView(
              child: StatefulBuilder(
                builder: (context, change) => NativeTaxonomyFields(
                  data: data,
                  kind: 'product',
                  hallID: 1,
                  enabled: true,
                  onChanged: (patch) =>
                      change(() => data = {...data, ...patch}),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.enterText(find.widgetWithText(TextField, '产品品牌'), '品牌A');
      await tester.enterText(
        find.widgetWithText(TextField, '适用机型'),
        'A100，B200',
      );
      await tester.pump();
      expect(data['brand'], '品牌A');
      expect(data['applicable_models'], ['A100', 'B200']);
      expect(data['parameters'], '旧参数原文');
      expect(jsonEncode(data), contains('A100'));
      await tester.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Taxonomy fields fit 64 language, width, large-text and simulated-keyboard layouts',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetViewInsets);
      final m = await fixture.upgradeModel(
        signed: true,
        handler: (r) async => r.url.path.endsWith('/taxonomy')
            ? fixture.jsonResponse({'categories': categories})
            : null,
      );
      var checked = 0;
      for (final width in [360.0, 390.0, 768.0, 1440.0]) {
        for (final lang in ['zh', 'en']) {
          m.language = lang;
          for (final kind in ['vendor', 'product']) {
            for (final scale in [1.0, 1.5]) {
              for (final keyboard in [false, true]) {
                tester.view.physicalSize = Size(width, 900);
                tester.view.viewInsets = FakeViewPadding(
                  bottom: keyboard ? 300 : 0,
                );
                Json data = kind == 'vendor'
                    ? {
                        'primary_category_id': 102,
                        'secondary_category_ids': [103],
                        'business_nature': 'manufacturing_trading',
                      }
                    : {
                        'name': '油缸',
                        'product_category_id': 102,
                        'applicable_models': ['Model A100', 'Model B200'],
                      };
                await tester.pumpWidget(
                  fixture.upgradeApp(
                    m,
                    Scaffold(
                      body: SingleChildScrollView(
                        padding: const EdgeInsets.all(16),
                        child: StatefulBuilder(
                          builder: (context, change) => NativeTaxonomyFields(
                            data: data,
                            kind: kind,
                            hallID: 1,
                            enabled: true,
                            onChanged: (patch) =>
                                change(() => data = {...data, ...patch}),
                          ),
                        ),
                      ),
                    ),
                    scale: scale,
                  ),
                );
                await tester.pumpAndSettle();
                expect(
                  tester.takeException(),
                  isNull,
                  reason: '$width/$lang/$kind/$scale/keyboard=$keyboard',
                );
                if (keyboard && kind == 'product') {
                  final field = find.widgetWithText(
                    TextField,
                    lang == 'zh' ? '适用机型' : 'Compatible models',
                  );
                  await tester.ensureVisible(field);
                  await tester.pumpAndSettle();
                  final bounds = tester.getRect(field);
                  expect(bounds.left, greaterThanOrEqualTo(0));
                  expect(bounds.right, lessThanOrEqualTo(width));
                  expect(bounds.bottom, lessThanOrEqualTo(601));
                }
                checked++;
                await tester.pumpWidget(const SizedBox());
              }
            }
          }
        }
      }
      expect(checked, 64);
    },
  );
}
