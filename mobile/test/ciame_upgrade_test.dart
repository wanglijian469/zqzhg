import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'app_navigation_test.dart' as nav;
import 'mobile_upgrade_test.dart' as f;

void main() {
  testWidgets(
    'CIAME category search narrows secondary options by primary category',
    (t) async {
      final m = await f.upgradeModel();
      int? selected;
      final cats = <Json>[
        {'id': 1, 'parent_id': 0, 'name': '耕整地机械及零部件', 'enabled': true},
        {'id': 2, 'parent_id': 1, 'name': '耕地机械', 'enabled': true},
        {'id': 3, 'parent_id': 0, 'name': '收获机械及零部件', 'enabled': true},
        {'id': 4, 'parent_id': 3, 'name': '谷物收获机械', 'enabled': true},
        {'id': 5, 'parent_id': 3, 'name': '玉米收获机械', 'enabled': true},
      ];
      await t.pumpWidget(
        f.upgradeApp(
          m,
          Scaffold(
            body: Builder(
              builder: (c) => TextButton(
                onPressed: () async {
                  selected = await searchNativeCategories(
                    c,
                    cats,
                    'product',
                    [],
                  );
                },
                child: const Text('choose'),
              ),
            ),
          ),
        ),
      );
      await t.tap(find.text('choose'));
      await t.pumpAndSettle();
      await t.tap(find.byType(DropdownButtonFormField<int>));
      await t.pumpAndSettle();
      await t.tap(find.text('收获机械及零部件').last);
      await t.pumpAndSettle();
      expect(find.text('耕整地机械及零部件 / 耕地机械'), findsNothing);
      expect(find.text('收获机械及零部件 / 谷物收获机械'), findsOneWidget);
      await t.enterText(find.byType(TextField), '玉米');
      await t.pumpAndSettle();
      await t.tap(find.text('收获机械及零部件 / 玉米收获机械'));
      await t.pumpAndSettle();
      expect(selected, 5);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Default portal enters a hall with the website identity and no procurement entry',
    (t) async {
      t.view.physicalSize = const Size(390, 844);
      t.view.devicePixelRatio = 1;
      addTearDown(t.view.resetPhysicalSize);
      addTearDown(t.view.resetDevicePixelRatio);
      final calls = <Uri>[];
      final m = await nav.fixture(signed: false, calls: calls);
      await t.pumpWidget(nav.harness(m, home: const AppShell()));
      await t.pumpAndSettle();
      expect(find.byType(ExhibitionPortalScreen), findsOneWidget);
      expect(find.text('行业展厅'), findsOneWidget);
      expect(calls.where((u) => u.path.endsWith('/home')), isEmpty);
      await t.tap(find.text('农业机械厅'));
      await t.pumpAndSettle();
      expect(find.text('中企展馆-农业机械厅'), findsOneWidget);
      expect(find.byType(ProcurementEntry), findsNothing);
      expect(calls.where((u) => u.path.endsWith('/home')), hasLength(1));
      await t.tap(find.byTooltip('展馆总页'));
      await t.pumpAndSettle();
      expect(find.byType(ExhibitionPortalScreen), findsOneWidget);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Manufacturer company registration uses the shared account API without SMS',
    (t) async {
      Json? request;
      String? path;
      bool continued = false;
      final m = await f.upgradeModel(
        handler: (r) async {
          if (r.url.path.endsWith('/auth/merchant/register')) {
            path = r.url.path;
            request = asMap(jsonDecode(r.body));
            return f.jsonResponse({
              'token': 'registered-company',
              'account': {'id': 77, 'role': 'merchant'},
            });
          }
          return null;
        },
      );
      await t.pumpWidget(
        f.upgradeApp(
          m,
          LoginScreen(register: true, onRegistered: () => continued = true),
        ),
      );
      await t.pumpAndSettle();
      expect(find.byKey(const ValueKey('auth-code')), findsNothing);
      await t.enterText(
        find.byKey(const ValueKey('auth-login_name')),
        'ciame-company',
      );
      await t.enterText(
        find.byKey(const ValueKey('auth-password')),
        'Company-2026-Test!',
      );
      await t.enterText(
        find.byKey(const ValueKey('auth-confirmation')),
        'Company-2026-Test!',
      );
      await t.ensureVisible(find.byType(CheckboxListTile));
      await t.tap(find.byType(CheckboxListTile));
      await t.pumpAndSettle();
      await t.ensureVisible(find.widgetWithText(FilledButton, '注册并继续'));
      await t.tap(find.widgetWithText(FilledButton, '注册并继续'));
      await t.pumpAndSettle();
      expect(path, '/api/v1/auth/merchant/register');
      expect(request?['login_name'], 'ciame-company');
      expect(request?.containsKey('code'), isFalse);
      expect(continued, isTrue);
      expect(m.account['id'], 77);
      await t.pumpWidget(const SizedBox());
    },
  );
}
