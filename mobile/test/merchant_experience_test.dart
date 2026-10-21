import 'dart:convert';
import 'dart:io';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  Future<AppModel> model(http.Client client) async {
    SharedPreferences.setMockInitialValues({});
    return AppModel(await SharedPreferences.getInstance(), {
      for (final lang in ['zh', 'en'])
        lang: asMap(
          jsonDecode(File('assets/i18n/$lang.json').readAsStringSync()),
        ),
    }, client: client);
  }

  final vendor = {
    'id': 1,
    'kind': 'vendor',
    'showroom_slug': 'company',
    'status': 'approved',
    'public_visible': true,
    'data': {
      'name': '测试农机厂家',
      'region_name': '河北',
      'main_products': '测试产品',
      'media_ids': <int>[],
    },
  };
  testWidgets(
    'Company login opens the four-section workspace with account-wide counts',
    (tester) async {
      tester.view.physicalSize = const Size(390, 844);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final calls = <String>[];
      final m = await model(
        MockClient((r) async {
          calls.add(r.url.path);
          dynamic data;
          if (r.url.path.endsWith('/auth/merchant')) {
            expect(jsonDecode(r.body), {
              'username': '公司账号',
              'password': 'Example-Pass-2026!',
            });
            data = {
              'token': 'test-token',
              'account': {'id': 9},
            };
          } else if (r.url.path.endsWith('/merchant/dashboard')) {
            data = {
              'products': {
                'total': 45,
                'public': 30,
                'draft': 4,
                'pending': 8,
                'rejected': 3,
              },
            };
          } else if (r.url.path.endsWith('/merchant/contents')) {
            data = {
              'items': [vendor],
              'total': 1,
              'page': 1,
              'page_size': 20,
            };
          } else {
            data = {};
          }
          return http.Response(
            jsonEncode(data),
            200,
            headers: {'content-type': 'application/json; charset=utf-8'},
          );
        }),
      );
      await tester.pumpWidget(
        ModelScope(
          model: m,
          child: AnimatedBuilder(
            animation: m,
            builder: (c, _) => MaterialApp(
              builder: (c, child) => MediaQuery(
                data: MediaQuery.of(
                  c,
                ).copyWith(textScaler: TextScaler.linear(1.4)),
                child: child!,
              ),
              home: const MerchantScreen(),
            ),
          ),
        ),
      );
      await tester.tap(find.text('已有公司账号登录'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('原账号登录'));
      await tester.pumpAndSettle();
      await tester.enterText(find.widgetWithText(TextField, '原登录账号'), '公司账号');
      await tester.enterText(
        find.widgetWithText(TextField, '密码'),
        'Example-Pass-2026!',
      );
      await tester.tap(find.widgetWithText(FilledButton, '登录'));
      await tester.pumpAndSettle();
      expect(m.account['id'], 9);
      await tester.pump(const Duration(milliseconds: 500));
      await tester.pumpAndSettle();
      await tester.drag(find.byType(ListView).first, const Offset(0, -350));
      await tester.pumpAndSettle();
      expect(calls, contains('/api/v1/merchant/dashboard'));
      expect(find.text('30'), findsOneWidget);
      for (final label in ['编辑展厅', '添加产品', '产品管理', '分享企业展厅', '展厅管理']) {
        expect(find.text(label), findsWidgets);
      }
      expect(calls, contains('/api/v1/merchant/dashboard'));
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );
  testWidgets('Profile edits retain existing contacts and unedited fields', (
    tester,
  ) async {
    final m = await model(MockClient((_) async => http.Response('{}', 200)));
    Json d = {
      'name': '测试企业',
      'media_ids': [1],
      'company_profile': {
        'summary': '原简介',
        'capacity': '供货能力原值',
        'contacts': [
          {'area': '河北', 'name': '原联系人', 'phone': '13900000001'},
        ],
        'cover_media_id': 1,
      },
    };
    await tester.pumpWidget(
      ModelScope(
        model: m,
        child: MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(
              child: StatefulBuilder(
                builder: (c, set) => PromotionalProfileEditor(
                  data: d,
                  enabled: true,
                  onChanged: (v) => set(() => d = v),
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.tap(find.text('宣传资料'));
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.widgetWithText(TextFormField, '宣传简介'));
    await tester.enterText(find.widgetWithText(TextFormField, '宣传简介'), '新简介');
    expect(asMap(d['company_profile'])['capacity'], '供货能力原值');
    expect(
      rows(asMap(d['company_profile'])['contacts']).single['phone'],
      '13900000001',
    );
    await tester.pumpWidget(const SizedBox());
  });
  testWidgets(
    'Native QR and poster contain the canonical URL without private filters',
    (tester) async {
      final m = await model(MockClient((_) async => http.Response('{}', 200)));
      const url = 'http://172.27.35.2:8080/v/company';
      expect(showroomShareURL('/v/company', 'http://172.27.35.2:8080'), url);
      expect(
        showroomShareURL(
          '/v/company?q=private#products',
          'http://172.27.35.2:8080',
        ),
        url,
      );
      expect(showroomAddressScope(url), 'lanShare');
      await tester.runAsync(() async {
        final font = File('C:/Windows/Fonts/msyh.ttc');
        if (await font.exists()) {
          await (FontLoader('sans-serif')..addFont(
                font.readAsBytes().then((b) => ByteData.sublistView(b)),
              ))
              .load();
        }
        final qr = await showroomPNG(m, vendor, url, poster: false);
        final poster = await showroomPNG(m, vendor, url, poster: true);
        expect(qr.take(8).toList(), [137, 80, 78, 71, 13, 10, 26, 10]);
        await File('../.local/mobile-share-qr.png').writeAsBytes(qr);
        await File('../.local/mobile-share-poster.png').writeAsBytes(poster);
      });
    },
  );
}
