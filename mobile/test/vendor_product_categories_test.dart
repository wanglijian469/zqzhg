import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'mobile_upgrade_test.dart' as fixture;

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  final root = {
    'id': 13,
    'name': '液压泵、阀、输出装置零部件',
    'name_en': 'Hydraulic components',
  };
  Json cat() => {
    'id': 51,
    'vendor_id': 21,
    'name': '液压回转马达',
    'name_en': '',
    'industry_category_id': 13,
    'industry_name': root['name'],
    'enabled': true,
    'sort_order': 0,
  };
  testWidgets(
    'new company category is immediately selected under the preferred industry',
    (t) async {
      final categories = <Json>[];
      final writes = <Json>[];
      int selected = 0;
      final m = await fixture.upgradeModel(
        signed: true,
        handler: (r) async {
          if (!r.url.path.contains('/product-categories')) return null;
          if (r.method == 'POST') {
            final data = asMap(jsonDecode(r.body));
            writes.add(data);
            final saved = {...cat(), ...data};
            categories.add(saved);
            return fixture.jsonResponse(saved, 201);
          }
          return fixture.jsonResponse({
            'items': categories,
            'industry_categories': [root],
            'preferred_industry_ids': [13],
          });
        },
      );
      await t.pumpWidget(
        fixture.upgradeApp(
          m,
          Scaffold(
            body: SingleChildScrollView(
              child: NativeVendorProductCategoryPicker(
                value: 0,
                onChanged: (id) => selected = id,
              ),
            ),
          ),
        ),
      );
      await t.pumpAndSettle();
      await fixture.tapReachable(t, find.text('新增产品分类'));
      await t.enterText(find.byType(TextField).at(1), '新建液压阀组');
      await fixture.tapReachable(t, find.text('保存分类'));
      await t.pumpAndSettle();
      expect(selected, 51);
      expect(writes.single['industry_category_id'], 13);
      expect(writes.single['name'], '新建液压阀组');
      expect(find.textContaining('待管理员规范'), findsNothing);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
  for (final width in [360.0, 390.0, 768.0]) {
    for (final lang in ['zh', 'en']) {
      testWidgets('company category management $width $lang large text', (
        t,
      ) async {
        t.view.physicalSize = Size(width, 900);
        t.view.devicePixelRatio = 1;
        addTearDown(t.view.resetPhysicalSize);
        addTearDown(t.view.resetDevicePixelRatio);
        final longName = List.filled(6, '液压回转马达与非标定制总成').join();
        final longEnglish = List.filled(
          7,
          'HydraulicRotaryMotorCustomAssembly',
        ).join();
        final m = await fixture.upgradeModel(
          signed: true,
          handler: (r) async => r.url.path.contains('/product-categories')
              ? fixture.jsonResponse({
                  'items': [
                    {...cat(), 'name': longName, 'name_en': longEnglish},
                  ],
                  'industry_categories': [root],
                  'preferred_industry_ids': [13],
                })
              : null,
        );
        m.language = lang;
        await t.pumpWidget(
          fixture.upgradeApp(
            m,
            const VendorProductCategoriesScreen(),
            scale: 1.7,
          ),
        );
        await t.pumpAndSettle();
        expect(
          find.text(lang == 'en' ? longEnglish : longName),
          findsOneWidget,
        );
        expect(t.takeException(), isNull);
        await fixture.tapReachable(t, find.text(m.t('edit')));
        expect(find.byType(AlertDialog), findsOneWidget);
        expect(t.takeException(), isNull);
        await t.pumpWidget(const SizedBox());
      });
    }
  }
  testWidgets('management saves renamed category, order and inactive state', (
    t,
  ) async {
    Json saved = cat();
    final writes = <Json>[];
    final m = await fixture.upgradeModel(
      signed: true,
      handler: (r) async {
        if (!r.url.path.contains('/product-categories')) return null;
        if (r.method == 'PUT') {
          final data = asMap(jsonDecode(r.body));
          writes.add(data);
          saved = {...saved, ...data};
          return fixture.jsonResponse(saved);
        }
        return fixture.jsonResponse({
          'items': [saved],
          'industry_categories': [root],
          'preferred_industry_ids': [13],
        });
      },
    );
    await t.pumpWidget(
      fixture.upgradeApp(m, const VendorProductCategoriesScreen()),
    );
    await t.pumpAndSettle();
    await fixture.tapReachable(t, find.text(m.t('edit')));
    final dialog = find.byType(AlertDialog);
    final fields = find.descendant(
      of: dialog,
      matching: find.byType(TextField),
    );
    await t.enterText(fields.at(0), '液压回转马达与定制总成');
    await t.enterText(fields.at(2), '25');
    await fixture.tapReachable(t, find.byType(SwitchListTile));
    await fixture.tapReachable(t, find.text('保存分类'));
    expect(writes.single['name'], '液压回转马达与定制总成');
    expect(writes.single['sort_order'], 25);
    expect(writes.single['enabled'], false);
    expect(find.text('液压回转马达与定制总成'), findsOneWidget);
    expect(find.textContaining('已停用'), findsOneWidget);
    expect(t.takeException(), isNull);
    await t.pumpWidget(const SizedBox());
  });
  testWidgets(
    'showroom keeps company filter, keyword and page across remount',
    (t) async {
      final requests = <Uri>[];
      final m = await fixture.upgradeModel(
        handler: (r) async {
          if (!r.url.path.endsWith('/public/products')) return null;
          requests.add(r.url);
          return fixture.jsonResponse({
            'items': [],
            'total': 26,
            'page_size': 24,
          });
        },
      );
      final memory = <String, dynamic>{
        'category': 51,
        'page': 2,
        'query': '回转马达',
        'term': '回转马达',
      };
      Widget page() => fixture.upgradeApp(
        m,
        Scaffold(
          body: SingleChildScrollView(
            child: NativeCompanyProducts(
              vendorID: 21,
              hall: ExhibitionHall.agriculture,
              categories: [
                {...cat(), 'product_count': 26},
              ],
              companyCategories: true,
              memory: memory,
            ),
          ),
        ),
      );
      await t.pumpWidget(page());
      await t.pumpAndSettle();
      expect(requests.last.queryParameters['vendor_id'], '21');
      expect(requests.last.queryParameters['vendor_product_category_id'], '51');
      expect(requests.last.queryParameters.containsKey('category_id'), false);
      expect(requests.last.queryParameters['q'], '回转马达');
      expect(requests.last.queryParameters['page'], '2');
      await t.enterText(find.byType(TextField).first, '液压阀组');
      await t.testTextInput.receiveAction(TextInputAction.search);
      await t.pumpAndSettle();
      expect(memory['term'], '液压阀组');
      expect(memory['page'], 1);
      expect(requests.last.queryParameters['q'], '液压阀组');
      await t.pumpWidget(const SizedBox());
      await t.pumpWidget(page());
      await t.pumpAndSettle();
      expect(requests.last.queryParameters['vendor_product_category_id'], '51');
      expect(requests.last.queryParameters['q'], '液压阀组');
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets('draft copy preserves the company category identifier', (
    t,
  ) async {
    final source = {'name': '产品', 'vendor_product_category_id': 51};
    expect(cloneProductDraft(source)['vendor_product_category_id'], 51);
  });
}
