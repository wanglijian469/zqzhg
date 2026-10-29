import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:http/http.dart' as http;
import 'app_navigation_test.dart' as f;

Future<void> tapVisible(WidgetTester t, Finder item) async {
  await t.ensureVisible(item);
  await t.tap(item);
  await t.pumpAndSettle();
}

void main() {
  testWidgets(
    'Workspace keeps four primary actions, storage warning and optional details',
    (t) async {
      final m = await f.fixture(
        intercept: (r) async {
          if (!r.url.path.endsWith('/storage/usage')) return null;
          return http.Response(
            jsonEncode({
              'used_bytes': 190000000,
              'reserved_bytes': 1000000,
              'quota_bytes': 200000000,
              'remaining_bytes': 9000000,
            }),
            200,
          );
        },
      );
      await t.pumpWidget(f.harness(m, home: const MerchantWorkspace()));
      await t.pumpAndSettle();
      for (final label in ['企业资料', '产品管理', '预览官网', '分享官网']) {
        expect(find.text(label), findsOneWidget);
      }
      expect(find.text('新增产品'), findsNothing);
      expect(find.textContaining('免费存储接近'), findsOneWidget);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Default launch enters agriculture without preparing hall selection',
    (t) async {
      final m = await f.fixture(signed: false);
      await t.pumpWidget(f.harness(m, home: const AppShell()));
      await t.pumpAndSettle();
      expect(find.text('中企展馆-农业机械厅'), findsOneWidget);
      expect(find.byType(ExhibitionPortalScreen), findsNothing);
      expect(find.byKey(const ValueKey('home-hall-selector')), findsNothing);
      expect(find.byKey(const ValueKey('home-map')), findsOneWidget);
      expect(find.byKey(const ValueKey('home-admission')), findsOneWidget);
      expect(t.getSize(find.byKey(const ValueKey('home-banner'))).height, 0);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Mine has one login; settings has language and legal links without endpoint',
    (t) async {
      final m = await f.fixture(signed: false);
      await t.pumpWidget(f.harness(m));
      await t.pumpAndSettle();
      await tapVisible(t, find.byKey(const ValueKey('app-tab-3')));
      expect(find.text('登录／注册'), findsOneWidget);
      expect(find.text('手机号登录 / 注册'), findsNothing);
      expect(find.text('厂商账号登录'), findsNothing);
      await tapVisible(t, find.byKey(const ValueKey('my-settings')));
      expect(find.text('语言'), findsOneWidget);
      expect(find.text('API 地址'), findsNothing);
      expect(find.text(m.baseUrl), findsNothing);
      expect(find.text('入驻与服务说明'), findsOneWidget);
      expect(find.text('隐私政策'), findsOneWidget);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets('A saved preparing hall returns to the only open hall', (
    t,
  ) async {
    final m = await f.fixture(signed: false);
    await m.prefs.setString(
      'active_hall.${Uri.encodeComponent(m.baseUrl)}',
      'cable',
    );
    await m.loadHalls();
    expect(m.activeHall.slug, 'agriculture');
    expect(m.isHallPreview, false);
    expect(m.showHallSelector, false);
  });
  testWidgets('Two open halls expose selection and keep preparing halls out', (
    t,
  ) async {
    final m = await f.fixture(
      signed: false,
      intercept: (r) async {
        if (!r.url.path.endsWith('/halls')) return null;
        return http.Response(
          jsonEncode({
            'items': [
              {
                'id': 1,
                'slug': 'agriculture',
                'name': '农业机械厅',
                'status': 'open',
              },
              {'id': 2, 'slug': 'cable', 'name': '电线电缆厅', 'status': 'open'},
              {
                'id': 3,
                'slug': 'preparing',
                'name': '筹备厅',
                'status': 'preparing',
              },
            ],
          }),
          200,
          headers: {'content-type': 'application/json; charset=utf-8'},
        );
      },
    );
    await t.pumpWidget(f.harness(m));
    await t.pumpAndSettle();
    await tapVisible(t, find.byKey(const ValueKey('home-hall-selector')));
    expect(find.text('电线电缆厅'), findsOneWidget);
    expect(find.textContaining('筹备厅'), findsNothing);
    await t.pumpWidget(const SizedBox());
  });
  testWidgets(
    'Empty contact/profile fields do not create blank vendor controls',
    (t) async {
      final m = await f.fixture(signed: false);
      final vendor = f.company();
      vendor['data'] = {'name': '测试厂家'};
      await t.pumpWidget(
        f.harness(
          m,
          home: Scaffold(body: NativeVendorCard(item: vendor)),
        ),
      );
      await t.pumpAndSettle();
      expect(find.byKey(const ValueKey('showroom-1')), findsOneWidget);
      expect(find.byKey(const ValueKey('contact-vendor-1')), findsNothing);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
  for (final width in [360.0, 390.0, 844.0, 1024.0]) {
    for (final scale in [1.0, 1.8]) {
      testWidgets('Home/mine width=$width scale=$scale in both languages', (
        t,
      ) async {
        t.view.physicalSize = Size(width, width == 844 ? 390 : 900);
        t.view.devicePixelRatio = 1;
        addTearDown(t.view.resetPhysicalSize);
        addTearDown(t.view.resetDevicePixelRatio);
        final m = await f.fixture(signed: false);
        await t.pumpWidget(f.harness(m, scale: scale));
        await t.pumpAndSettle();
        expect(t.takeException(), isNull);
        await m.toggleLanguage();
        await t.pumpAndSettle();
        expect(t.takeException(), isNull);
        await tapVisible(t, find.byKey(const ValueKey('app-tab-3')));
        expect(t.takeException(), isNull);
        await t.pumpWidget(const SizedBox());
      });
    }
  }
  testWidgets('Capture simplified Android pages with readable Chinese font', (
    t,
  ) async {
    t.view.physicalSize = const Size(390, 844);
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.resetPhysicalSize);
    addTearDown(t.view.resetDevicePixelRatio);
    await t.runAsync(() async {
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
    final boundary = GlobalKey();
    final m = await f.fixture(signed: false);
    await t.pumpWidget(f.harness(m, boundary: boundary));
    await t.pumpAndSettle();
    Future<void> capture(String name) async {
      expect(t.takeException(), isNull);
      final render =
          boundary.currentContext!.findRenderObject() as RenderRepaintBoundary;
      await t.runAsync(() async {
        final image = await render.toImage(pixelRatio: 1.5);
        final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
        final file = File('../docs/screenshots/android-1.10.3/$name.png');
        file.parent.createSync(recursive: true);
        file.writeAsBytesSync(bytes!.buffer.asUint8List());
        image.dispose();
      });
    }

    await capture('home');
    await tapVisible(t, find.byKey(const ValueKey('app-tab-1')));
    await capture('manufacturers');
    await tapVisible(t, find.byKey(const ValueKey('app-tab-2')));
    await capture('products');
    await tapVisible(t, find.byKey(const ValueKey('app-tab-3')));
    await capture('mine');
    await tapVisible(t, find.byKey(const ValueKey('my-settings')));
    await capture('settings');
    await t.pumpWidget(const SizedBox());
    final merchant = await f.fixture();
    await t.pumpWidget(
      f.harness(merchant, home: const MerchantWorkspace(), boundary: boundary),
    );
    await t.pumpAndSettle();
    await capture('workspace');
    await tapVisible(t, find.text('企业资料'));
    await capture('company-basics');
    await tapVisible(t, find.widgetWithText(ChoiceChip, '2. 展示资料'));
    await capture('company-presentation');
    await t.pumpWidget(const SizedBox());
  }, skip: Platform.environment['APP103_CAPTURE'] != '1');
}
