import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'app_navigation_test.dart' as nav;

http.Response response(dynamic data, [int status = 200]) => http.Response(
  jsonEncode(data),
  status,
  headers: {'content-type': 'application/json; charset=utf-8'},
);
Json equipment({
  String name = '审核中的设备修改',
  String status = 'pending',
  bool visible = true,
}) => {
  'id': 8,
  'kind': 'product',
  'status': status,
  'visible': visible,
  'public_visible': visible,
  'revision_id': 1,
  'data': {
    'name': name,
    'name_en': 'Industrial equipment with a very long product name',
    'parameters': '型号：1LF-440\n规格：液压翻转设备',
    'media_ids': <int>[],
  },
};
Future<AppModel> model({
  List<Uri>? calls,
  Future<http.Response?> Function(http.Request)? intercept,
}) => nav.fixture(
  calls: calls,
  intercept: (r) async {
    final override = await intercept?.call(r);
    if (override != null) return override;
    if (r.url.path.endsWith('/merchant/contents') &&
        r.url.queryParameters['kind'] == 'product') {
      return response({
        'items': [
          equipment(),
          equipment(name: '产品二', status: 'draft', visible: false),
        ],
        'total': 41,
      });
    }
    if (r.url.path.contains('/public/showrooms/')) {
      return response({
        'item': nav.company(),
        'canonical_url': 'https://business.example/v/fixture?private=1#draft',
      });
    }
    return null;
  },
);
void viewport(WidgetTester t, double width) {
  t.view.physicalSize = Size(width, 900);
  t.view.devicePixelRatio = 1;
  addTearDown(t.view.resetPhysicalSize);
  addTearDown(t.view.resetDevicePixelRatio);
}

void main() {
  testWidgets(
    'Zero-valued logo falls back to a real public cover and existing introduction is recognized',
    (t) async {
      final company = nav.company();
      company['data'] = {
        ...asMap(company['data']),
        'company_profile': {
          'logo_media_id': 0,
          'cover_media_id': 71,
          'summary': '',
        },
      };
      final m = await model(
        intercept: (r) async {
            if (r.url.path.contains('/public/showrooms/')) {
            return response({
              'item': company,
              'canonical_url': 'https://business.example/v/fixture',
            });
            }
          return null;
        },
      );
      await t.pumpWidget(
        nav.harness(m, home: const ShowroomPromotion(slug: 'fixture')),
      );
      await t.pumpAndSettle();
      expect(t.widget<Photo>(find.byType(Photo)).id, 71);
      expect(find.text('农业机械制造与生产。'), findsOneWidget);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'My company and independent workspace share the vendor request across language and routes',
    (t) async {
      viewport(t, 390);
      final calls = <Uri>[];
      final m = await model(calls: calls);
      await t.pumpWidget(nav.harness(m));
      await t.pumpAndSettle();
      await nav.tap(t, find.byKey(const ValueKey('app-tab-3')));
      expect(find.text(m.t('myBooth')), findsOneWidget);
      await nav.tap(t, find.byKey(const ValueKey('my-booth')));
      for (final label in [
        'editShowroom',
        'addProduct',
        'productManage',
        'shareShowroom',
      ]) {
        expect(find.text(m.t(label)), findsWidgets);
      }
      expect(find.byKey(const ValueKey('merchant-tab-0')), findsNothing);
      await m.toggleLanguage();
      await t.pumpAndSettle();
      await nav.tap(t, find.text(m.t('boothManage')));
      expect(find.byType(MerchantManagementScreen), findsOneWidget);
      expect(
        calls
            .where(
              (u) =>
                  u.path.endsWith('/merchant/contents') &&
                  u.queryParameters['kind'] == 'vendor',
            )
            .length,
        1,
      );
      expect(
        calls.where((u) => u.path.endsWith('/merchant/dashboard')).length,
        1,
      );
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Products debounce, preserve independent review and visibility filters and avoid identical submissions',
    (t) async {
      viewport(t, 390);
      final calls = <Uri>[];
      final m = await model(calls: calls);
      await t.pumpWidget(nav.harness(m, home: const MerchantProductsScreen()));
      await t.pumpAndSettle();
      int reads() => calls
          .where(
            (u) =>
                u.path.endsWith('/merchant/contents') &&
                u.queryParameters['kind'] == 'product',
          )
          .length;
      expect(reads(), 1);
      await t.enterText(find.byType(TextField), '1LF');
      await t.pump(const Duration(milliseconds: 349));
      expect(reads(), 1);
      await t.pump(const Duration(milliseconds: 1));
      await t.pumpAndSettle();
      expect(reads(), 2);
      await t.testTextInput.receiveAction(TextInputAction.done);
      await t.pumpAndSettle();
      expect(reads(), 2);
      expect(find.text('型号：1LF-440 · 规格：液压翻转设备'), findsWidgets);
      await nav.tap(t, find.text(m.t('published')).first);
      expect(calls.last.queryParameters['public_state'], 'public');
      expect(calls.last.queryParameters['q'], '1LF');
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'A delayed old search response is never rendered after a newer query',
    (t) async {
      final delayed = Completer<http.Response>();
      final m = await model(
        intercept: (r) async {
          if (r.url.queryParameters['q'] == 'old') return delayed.future;
          if (r.url.queryParameters['q'] == 'new') {
            return response({
              'items': [equipment(name: '新搜索结果')],
              'total': 1,
            });
          }
          return null;
        },
      );
      await t.pumpWidget(nav.harness(m, home: const MerchantProductsScreen()));
      await t.pumpAndSettle();
      await t.enterText(find.byType(TextField), 'old');
      await t.testTextInput.receiveAction(TextInputAction.done);
      await t.pump();
      await t.enterText(find.byType(TextField), 'new');
      await t.testTextInput.receiveAction(TextInputAction.done);
      await t.pumpAndSettle();
      delayed.complete(
        response({
          'items': [equipment(name: '迟到的旧结果')],
          'total': 1,
        }),
      );
      await t.pumpAndSettle();
      expect(find.text('新搜索结果'), findsOneWidget);
      expect(find.text('迟到的旧结果'), findsNothing);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Shared failures can be retried and an account switch clears company data',
    (t) async {
      var fails = true;
      final m = await model(
        intercept: (r) async {
          if (r.url.path.endsWith('/contents') &&
              r.url.queryParameters['kind'] == 'vendor') {
            if (fails) {
              return response({
                'error': {'code': 'network_error'},
              }, 503);
            }
            final company = nav.company();
            company['data'] = {
              ...asMap(company['data']),
              'name': r.headers['Authorization'] == 'Bearer other'
                  ? '其他账号企业'
                  : '原账号企业',
            };
            return response({
              'items': [company],
            });
          }
          return null;
        },
      );
      await t.pumpWidget(nav.harness(m, home: const PersonalScreen()));
      await t.pumpAndSettle();
      fails = false;
      await nav.tap(t, find.text(m.t('retry')));
      expect(find.text('原账号企业'), findsOneWidget);
      await m.signIn({
        'token': 'other',
        'account': {'id': 11, 'role': 'merchant'},
      });
      await t.pumpAndSettle();
      expect(find.text('其他账号企业'), findsOneWidget);
      expect(find.text('原账号企业'), findsNothing);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Product sharing reads only the public revision and configured canonical origin',
    (t) async {
      viewport(t, 390);
      final shared = <MethodCall>[];
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(sharingBridge, (call) async {
            shared.add(call);
            return true;
          });
      addTearDown(
        () => TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
            .setMockMethodCallHandler(sharingBridge, null),
      );
      final m = await model(
        intercept: (r) async {
          if (r.url.path.endsWith('/public/products/8')) {
            return response({
              'item': equipment(name: '正式公开设备'),
              'vendor': nav.company(),
            });
          }
          return null;
        },
      );
      await t.pumpWidget(nav.harness(m, home: const MerchantProductsScreen()));
      await t.pumpAndSettle();
      await nav.tap(t, find.byType(PopupMenuButton<String>).first);
      await nav.tap(t, find.text(m.t('shareProduct')));
      expect(shared.single.arguments['text'], contains('正式公开设备'));
      expect(shared.single.arguments['text'], isNot(contains('审核中的')));
      expect(
        shared.single.arguments['text'],
        contains('https://business.example/products/8'),
      );
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Share bridge failure offers a selectable real link and no unpublished data',
    (t) async {
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(sharingBridge, (_) async {
            throw PlatformException(code: 'unavailable');
          });
      addTearDown(
        () => TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
            .setMockMethodCallHandler(sharingBridge, null),
      );
      final m = await model();
      await t.pumpWidget(
        nav.harness(m, home: const ShowroomPromotion(slug: 'fixture')),
      );
      await t.pumpAndSettle();
      await nav.tap(t, find.widgetWithText(FilledButton, m.t('shareShowroom')));
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.text('https://business.example/v/fixture'), findsWidgets);
      expect(
        find.text('https://business.example/v/fixture?private=1#draft'),
        findsNothing,
      );
      await t.pumpWidget(const SizedBox());
    },
  );
  for (final width in [360.0, 390.0, 430.0]) {
    for (final lang in ['zh', 'en']) {
      for (final scale in [1.0, 1.4, 2.0]) {
        testWidgets('Manufacturer screens $width $lang scale $scale', (
          t,
        ) async {
          viewport(t, width);
          final m = await model();
          m.language = lang;
          for (final screen in <Widget>[
            const MerchantWorkspace(),
            const MerchantProductsScreen(),
            const MerchantManagementScreen(),
            const ShowroomPromotion(slug: 'fixture'),
          ]) {
            await t.pumpWidget(const SizedBox());
            await t.pumpWidget(nav.harness(m, home: screen, scale: scale));
            await t.pumpAndSettle();
            expect(t.takeException(), isNull);
            await t.drag(find.byType(ListView).last, const Offset(0, -500));
            await t.pumpAndSettle();
            expect(t.takeException(), isNull);
          }
          await t.pumpWidget(const SizedBox());
        });
      }
    }
  }
  testWidgets('Capture current manufacturer interfaces (test fixtures only)', (
    t,
  ) async {
    await t.runAsync(() async {
      for (final e in {
        'NavigationQA': 'C:/Windows/Fonts/msyh.ttc',
        'MaterialIcons':
            '../.tools/flutter/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
      }.entries) {
        await (FontLoader(e.key)..addFont(
              Future.value(
                ByteData.sublistView(File(e.value).readAsBytesSync()),
              ),
            ))
            .load();
      }
    });
    for (final width in [360.0, 390.0, 430.0]) {
      viewport(t, width);
      final m = await model();
      for (final entry in <String, Widget>{
        'mine': const PersonalScreen(),
        'dashboard': const MerchantWorkspace(),
        'products': const MerchantProductsScreen(),
        'showroom': const MerchantManagementScreen(),
        'promotion': const ShowroomPromotion(slug: 'fixture'),
      }.entries) {
        final key = GlobalKey();
        await t.pumpWidget(const SizedBox());
        await t.pumpWidget(
          nav.harness(
            m,
            home: Stack(
              children: [
                entry.value,
                Positioned(
                  left: 8,
                  bottom: 4,
                  child: Material(
                    color: Colors.white,
                    child: Text(
                      "测试数据 · UI 验收",
                      style: TextStyle(
                        fontFamily: 'NavigationQA',
                        fontSize: 11,
                        color: Colors.red,
                      ),
                    ),
                  ),
                ),
              ],
            ),
            boundary: key,
          ),
        );
        await t.pumpAndSettle();
        expect(t.takeException(), isNull);
        await t.runAsync(() async {
          final image =
              await (key.currentContext!.findRenderObject()
                      as RenderRepaintBoundary)
                  .toImage(pixelRatio: 1.5);
          final bytes = await image.toByteData(format: ui.ImageByteFormat.png);
          File(
            '../docs/screenshots/merchant-ui-app-${width.toInt()}-${entry.key}.png',
          ).writeAsBytesSync(bytes!.buffer.asUint8List());
          image.dispose();
        });
      }
    }
    await t.pumpWidget(const SizedBox());
  }, skip: Platform.environment['MERCHANT_CAPTURE'] != '1');
}
