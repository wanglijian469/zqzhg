import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

Json company({String status = 'approved'}) => {
  'id': 1,
  'kind': 'vendor',
  'revision_id': 12,
  'status': status,
  'public_visible': true,
  'showroom_slug': 'fixture',
  'review_note': status == 'rejected' ? '请补充企业资料' : '',
  'data': {
    'name': '河北冀农农机具有限公司',
    'name_en': 'Jinong Agricultural Machinery',
    'region_name': '河北 · 宁晋',
    'main_products': '农业机械、耕整地设备及配件',
    'phone': '0319-1234567',
    'description': '农业机械制造与生产。',
    'category_id': 1,
    'media_ids': <int>[],
    'company_profile': <String, dynamic>{},
  },
  'matching_products': [product()],
};
Json product() => {
  'id': 8,
  'kind': 'product',
  'data': {'name': '测试旋耕机 440', 'media_ids': <int>[]},
};

Future<AppModel> fixture({
  bool signed = true,
  List<Uri>? calls,
  Json? vendor,
  Future<http.Response?> Function(http.Request)? intercept,
}) async {
  SharedPreferences.setMockInitialValues({
    'base_url': 'http://fixture',
    if (signed) 'token': 'test-token',
    if (signed) 'account': jsonEncode({'id': 7}),
  });
  return AppModel(
    await SharedPreferences.getInstance(),
    {
      for (final lang in ['zh', 'en'])
        lang: asMap(
          jsonDecode(File('assets/i18n/$lang.json').readAsStringSync()),
        ),
    },
    client: MockClient((r) async {
      calls?.add(r.url);
      final override = await intercept?.call(r);
      if (override != null) return override;
      final path = r.url.path;
      final v = vendor ?? company();
      dynamic data = {'items': [], 'total': 0};
      if (path.endsWith('/home')) {
        data = {
          'vendors': [v],
          'products': [product()],
          'hero': {},
        };
      } else if (path.endsWith('/taxonomy')) {
        data = {
          'categories': [
            {'id': 1, 'parent_id': 0, 'name': '农业机械', 'enabled': true},
          ],
          'regions': [
            {'id': 1, 'name': '河北'},
          ],
        };
      } else if (path.endsWith('/dashboard')) {
        data = {
          'products': {
            'total': 45,
            'public': 30,
            'pending': 8,
            'rejected': 3,
            'draft': 4,
          },
        };
      } else if (path.endsWith('/contents') || path.endsWith('/vendors')) {
        data = {
          'items': [v],
          'total': 41,
        };
      } else if (path.endsWith('/products')) {
        data = {
          'items': [product()],
          'total': 41,
        };
      } else if (path.endsWith('/products/8')) {
        data = {'item': product(), 'vendor': v};
      } else if (path.contains('/showrooms/')) {
        data = {
          'item': v,
          'categories': [],
          'resource_counts': {},
          'product_count': 1,
          'representative_products': [product()],
        };
      }
      return http.Response(
        jsonEncode(data),
        200,
        headers: {'content-type': 'application/json; charset=utf-8'},
      );
    }),
  );
}

Widget harness(
  AppModel m, {
  Widget home = const HomeScreen(),
  double scale = 1,
  Key? boundary,
}) => ModelScope(
  model: m,
  child: AnimatedBuilder(
    animation: m,
    builder: (context, _) => MaterialApp(
      debugShowCheckedModeBanner: false,
      theme: industrialTheme().copyWith(
        textTheme: industrialTheme().textTheme.apply(
          fontFamily: 'NavigationQA',
        ),
      ),
      locale: Locale(m.language),
      supportedLocales: const [Locale('zh'), Locale('en')],
      localizationsDelegates: GlobalMaterialLocalizations.delegates,
      builder: (context, child) => MediaQuery(
        data: MediaQuery.of(
          context,
        ).copyWith(textScaler: TextScaler.linear(scale)),
        child: RepaintBoundary(key: boundary, child: child!),
      ),
      home: home,
    ),
  ),
);

Future<void> tap(WidgetTester tester, Finder finder) async {
  await Scrollable.ensureVisible(tester.element(finder.first), alignment: .5);
  await tester.pumpAndSettle();
  await tester.tap(finder.first);
  await tester.pumpAndSettle();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  void viewport(WidgetTester tester, double width) {
    tester.view.physicalSize = Size(width, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
  }

  testWidgets(
    'Four destinations retain independent searches, filters, page and scroll after details',
    (tester) async {
      viewport(tester, 390);
      final calls = <Uri>[];
      final m = await fixture(calls: calls);
      await tester.pumpWidget(harness(m));
      await tester.pumpAndSettle();
      for (var i = 0; i < 4; i++) {
        expect(find.byKey(ValueKey('app-tab-$i')), findsOneWidget);
      }
      expect(calls.any((u) => u.path.endsWith('/vendors')), isFalse);
      await tap(tester, find.byKey(const ValueKey('app-tab-1')));
      await tester.enterText(find.byType(TextField).first, '旋耕机');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      await tap(tester, find.byType(DropdownButtonFormField<int>).last);
      await tap(tester, find.text('河北').last);
      await tap(tester, find.text('下一页'));
      expect(
        calls.lastWhere((u) => u.path.endsWith('/vendors')).queryParameters,
        containsPair('page', '2'),
      );
      final scroll = tester
          .widget<CustomScrollView>(find.byType(CustomScrollView).last)
          .controller!;
      final offset = scroll.offset;
      await tap(tester, find.byKey(const ValueKey('app-tab-2')));
      await tester.enterText(find.byType(TextField).first, '440');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      await tap(tester, find.text('测试旋耕机 440'));
      expect(find.byType(DetailScreen), findsOneWidget);
      await tester.tap(find.byType(BackButton).first);
      await tester.pumpAndSettle();
      expect(
        tester.widget<TextField>(find.byType(TextField).first).controller!.text,
        '440',
      );
      await tap(tester, find.byKey(const ValueKey('app-tab-1')));
      expect(
        tester.widget<TextField>(find.byType(TextField).first).controller!.text,
        '旋耕机',
      );
      expect(scroll.offset, offset);
      expect(find.text('2'), findsOneWidget);
      expect(find.text('河北'), findsOneWidget);
      expect(find.text('展品速览'), findsNothing);
      expect(find.text('匹配展品'), findsNothing);
      expect(find.text('测试旋耕机 440'), findsNothing);
      await tap(tester, find.text('进入展厅'));
      expect(find.byType(CompanyShowroomScreen), findsOneWidget);
      expect(find.text('测试旋耕机 440'), findsOneWidget);
      await tester.tap(find.byType(BackButton).first);
      await tester.pumpAndSettle();
      expect(find.text('2'), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Home search selects an existing destination and applies the new query',
    (tester) async {
      viewport(tester, 390);
      final calls = <Uri>[];
      final m = await fixture(calls: calls);
      await tester.pumpWidget(harness(m));
      await tester.pumpAndSettle();
      await tap(tester, find.byKey(const ValueKey('home-search-kind')));
      await tap(tester, find.text('产品').last);
      await tester.enterText(find.byType(TextField).first, '440');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      expect(find.byType(BrowseScreen), findsOneWidget);
      expect(
        calls
            .lastWhere((u) => u.path.endsWith('/products'))
            .queryParameters['q'],
        '440',
      );
      expect(find.byKey(const ValueKey('app-tab-2')), findsOneWidget);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Account menu confirms logout, retains drafts and resets the workspace for another account',
    (tester) async {
      viewport(tester, 390);
      final m = await fixture();
      final draftKey = 'draft.${m.scope}.1';
      await m.prefs.setString(draftKey, 'retained-local-draft');
      await tester.pumpWidget(harness(m));
      await tester.pumpAndSettle();
      await tap(tester, find.byKey(const ValueKey('app-tab-3')));
      await tap(tester, find.byKey(const ValueKey('my-booth')));
      expect(find.text('退出登录'), findsNothing);
      await tap(tester, find.byTooltip('设置'));
      await tap(tester, find.text('退出登录'));
      await tap(tester, find.text('取消'));
      expect(m.token, 'test-token');
      await tap(tester, find.text('退出登录'));
      await tap(tester, find.widgetWithText(FilledButton, '退出登录'));
      expect(m.token, isEmpty);
      expect(find.byType(MerchantWorkspace), findsNothing);
      expect(m.prefs.getString(draftKey), 'retained-local-draft');
      await m.signIn({
        'token': 'other-token',
        'account': {'id': 8},
      });
      await tester.pumpAndSettle();
      expect(m.prefs.getString('draft.${m.scope}.1'), isNull);
      await tap(tester, find.byKey(const ValueKey('my-booth')));
      expect(find.byType(MerchantWorkspace), findsOneWidget);
      expect(find.byType(MerchantManagementScreen), findsNothing);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Booth task shortcuts locate forms, restore drafts and keep pending versions read-only',
    (tester) async {
      viewport(tester, 390);
      final m = await fixture();
      await tester.pumpWidget(harness(m));
      await tester.pumpAndSettle();
      await tap(tester, find.byKey(const ValueKey('app-tab-3')));
      await tap(tester, find.byKey(const ValueKey('my-booth')));
      await tap(tester, find.text(m.t('boothManage')));
      await tap(tester, find.text('Logo与封面'));
      expect(
        tester.widget<EditorScreen>(find.byType(EditorScreen)).initialSection,
        'brand',
      );
      expect(find.byType(NativeBrandImageEditor).hitTestable(), findsWidgets);
      await tester.tap(find.byType(BackButton).first);
      await tester.pumpAndSettle();
      await tap(tester, find.text('基本信息'));
      await tester.enterText(
        find.widgetWithText(TextField, '企业 / 工厂名称'),
        '修改后企业',
      );
      await tester.pump(const Duration(milliseconds: 500));
      await tester.tap(find.byType(BackButton).first);
      await tester.pumpAndSettle();
      await tap(tester, find.text('基本信息'));
      expect(find.text('修改后企业'), findsOneWidget);
      await tester.tap(find.byType(BackButton).first);
      await tester.pumpAndSettle();
      await tester.scrollUntilVisible(
        find.text('企业视频'),
        400,
        scrollable: find.byType(Scrollable).last,
      );
      await tap(tester, find.text('企业视频'));
      expect(find.byType(MerchantResourcesScreen), findsOneWidget);
      await tester.tap(find.byType(BackButton).first);
      await tester.pumpAndSettle();
      await tester.pump(const Duration(seconds: 4));
      expect(find.byType(NativeResourceList), findsNothing);
      await tester.pumpWidget(const SizedBox());
      final pending = await fixture(vendor: company(status: 'pending'));
      await tester.pumpWidget(
        harness(
          pending,
          home: EditorScreen(
            kind: 'vendor',
            item: company(status: 'pending'),
            initialSection: 'brand',
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(
        tester.widget<TextField>(find.byType(TextField).first).enabled,
        isFalse,
      );
      expect(
        tester
            .widget<FilledButton>(find.widgetWithText(FilledButton, '提交审核'))
            .onPressed,
        isNull,
      );
      expect(
        tester
            .widget<PromotionalProfileEditor>(
              find.byType(PromotionalProfileEditor),
            )
            .enabled,
        isFalse,
      );
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'First booth creation returns to the existing shell and reloads the new booth',
    (tester) async {
      viewport(tester, 390);
      var created = false;
      var boothReads = 0;
      final m = await fixture(
        intercept: (r) async {
          if (r.url.path.endsWith('/merchant/contents')) {
            if (r.method == 'POST') {
              created = true;
              return http.Response(
                jsonEncode(company()),
                200,
                headers: {'content-type': 'application/json; charset=utf-8'},
              );
            }
            boothReads++;
            return http.Response(
              jsonEncode({
                'items': created ? [company()] : [],
                'total': created ? 1 : 0,
              }),
              200,
              headers: {'content-type': 'application/json; charset=utf-8'},
            );
          }
          return null;
        },
      );
      await m.prefs.setString(
        'draft.${m.scope}.vendor',
        jsonEncode({'revision_id': 0, 'data': company()['data'], 'photos': []}),
      );
      await tester.pumpWidget(harness(m));
      await tester.pumpAndSettle();
      await tap(tester, find.byKey(const ValueKey('app-tab-3')));
      await tap(tester, find.byKey(const ValueKey('my-booth')));
      await tap(tester, find.text('免费入驻展馆'));
      await tap(tester, find.widgetWithText(OutlinedButton, '保存云端草稿'));
      expect(created, isTrue);
      expect(boothReads, greaterThanOrEqualTo(2));
      expect(find.byType(EditorScreen), findsNothing);
      expect(find.byType(MerchantWorkspace), findsOneWidget);
      expect(find.text('免费入驻展馆'), findsNothing);
      await tester.tap(find.byType(BackButton).first);
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('app-tab-3')), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );

  for (final width in [360.0, 390.0, 768.0]) {
    for (final language in ['zh', 'en']) {
      testWidgets(
        'Navigation and task layout $width $language at 1.4 text scale',
        (tester) async {
          viewport(tester, width);
          final m = await fixture();
          m.language = language;
          await tester.pumpWidget(harness(m, scale: 1.4));
          await tester.pumpAndSettle();
          for (var tab = 0; tab < 4; tab++) {
            await tap(tester, find.byKey(ValueKey('app-tab-$tab')));
            expect(tester.takeException(), isNull);
          }
          await tap(tester, find.byKey(const ValueKey('my-booth')));
          await tap(tester, find.text(m.t('boothManage')));
          await tester.drag(find.byType(ListView).last, const Offset(0, -500));
          await tester.pumpAndSettle();
          expect(tester.takeException(), isNull);
          await tester.pumpWidget(const SizedBox());
        },
      );
    }
  }

  testWidgets(
    'Capture navigation, manufacturer, booth tasks, editor shortcut and account screenshots',
    (tester) async {
      viewport(tester, 390);
      await tester.runAsync(() async {
        final font = FontLoader('NavigationQA');
        font.addFont(
          Future.value(
            ByteData.sublistView(
              File('C:/Windows/Fonts/msyh.ttc').readAsBytesSync(),
            ),
          ),
        );
        await font.load();
        final icons = FontLoader('MaterialIcons');
        icons.addFont(
          Future.value(
            ByteData.sublistView(
              File(
                '../.tools/flutter/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
              ).readAsBytesSync(),
            ),
          ),
        );
        await icons.load();
      });
      final m = await fixture();
      final boundary = GlobalKey();
      await tester.pumpWidget(harness(m, boundary: boundary));
      await tester.pumpAndSettle();
      Future<void> capture(String name) async {
        expect(tester.takeException(), isNull);
        final render =
            boundary.currentContext!.findRenderObject()
                as RenderRepaintBoundary;
        await tester.runAsync(() async {
          final img = await render.toImage(pixelRatio: 1.5);
          final bytes = await img.toByteData(format: ui.ImageByteFormat.png);
          File(
            '../docs/screenshots/app-navigation-$name.png',
          ).writeAsBytesSync(bytes!.buffer.asUint8List());
          img.dispose();
        });
      }

      await capture('home');
      await tap(tester, find.byKey(const ValueKey('app-tab-1')));
      await capture('manufacturers');
      await tap(tester, find.byKey(const ValueKey('app-tab-2')));
      await capture('products');
      await tap(tester, find.byKey(const ValueKey('app-tab-3')));
      await tap(tester, find.byKey(const ValueKey('my-booth')));
      await tap(tester, find.text(m.t('boothManage')));
      await capture('booth');
      await tap(tester, find.text('Logo与封面'));
      await capture('brand-editor');
      await tester.tap(find.byType(BackButton).first);
      await tester.pumpAndSettle();
      await tap(tester, find.byTooltip('账号'));
      await capture('account-menu');
      await tap(tester, find.text('退出登录'));
      await capture('logout');
      await tester.pumpWidget(const SizedBox());
    },
    skip: Platform.environment['RUN_NAVIGATION_SCREENSHOTS'] != '1',
  );
}
