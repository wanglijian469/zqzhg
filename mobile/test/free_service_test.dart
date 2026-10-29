import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'app_navigation_test.dart' as fixture;

final docs = <Json>[
  for (final key in ['service-info', 'terms', 'privacy', 'showroom-agreement'])
    {
      'id':
          [
            'service-info',
            'terms',
            'privacy',
            'showroom-agreement',
          ].indexOf(key) +
          1,
      'key': key,
      'version': 1,
      'digest': '$key-digest',
      'title': {
        'service-info': '企业入驻与服务说明',
        'terms': '用户协议',
        'privacy': '隐私政策',
        'showroom-agreement': '企业展厅服务协议',
      }[key],
      'title_en': key,
      'body': '基础入驻永久免费，默认免费存储200 MB（200,000,000字节）。',
      'body_en': 'Free basic admission. Default free storage 200 MB.',
      'effective_at': '2026-10-28T00:00:00Z',
    },
];
http.Response jsonResponse(dynamic data) => http.Response(
  jsonEncode(data),
  200,
  headers: {'content-type': 'application/json; charset=utf-8'},
);

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test(
    'analytics stays off by default, requires active opt-in and clears on withdrawal',
    () async {
      SharedPreferences.setMockInitialValues({
        'base_url': 'http://privacy.test',
        'zqzt.analytics.old.visitor': 'obsolete',
      });
      final prefs = await SharedPreferences.getInstance();
      var sent = 0;
      final m = AppModel(
        prefs,
        {'zh': {}, 'en': {}},
        testBaseUrl: 'http://privacy.test',
        analyticsEnabled: true,
        client: MockClient((request) async {
          sent++;
          return http.Response('{}', 202);
        }),
      );
      addTearDown(m.analytics.dispose);
      await Future<void>.delayed(Duration.zero);
      m.analytics.page('/');
      await m.analytics.flush();
      expect(sent, 0);
      expect(
        prefs.getKeys().where((k) => k.startsWith('zqzt.analytics.')),
        isEmpty,
      );
      await prefs.setBool('service.analytics_consent', true);
      m.analytics.page('/');
      await m.analytics.flush();
      expect(sent, 1);
      await prefs.setBool('service.analytics_consent', false);
      await m.analytics.withdraw();
      m.analytics.page('/');
      await m.analytics.flush();
      expect(sent, 1);
      expect(
        prefs.getKeys().where((k) => k.startsWith('zqzt.analytics.')),
        isEmpty,
      );
    },
  );
  testWidgets(
    'registration consent starts unchecked and binds active versions and digests',
    (t) async {
      final m = await fixture.fixture(
        signed: false,
        intercept: (r) async => r.url.path.contains('/service-documents')
            ? jsonResponse({'items': docs})
            : null,
      );
      List<Json> refs = [];
      await t.pumpWidget(
        fixture.harness(
          m,
          home: Scaffold(
            body: NativeRegistrationAgreements(
              onChanged: (value) => refs = value,
            ),
          ),
        ),
      );
      await t.pumpAndSettle();
      expect(
        t.widget<CheckboxListTile>(find.byType(CheckboxListTile)).value,
        isFalse,
      );
      await t.tap(find.byType(CheckboxListTile));
      await t.pumpAndSettle();
      expect(refs.length, 2);
      expect(refs.first['digest'], 'terms-digest');
      await t.tap(find.byType(CheckboxListTile));
      expect(refs, isEmpty);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'storage shows decimal MB and preserves existing files when over allowance',
    (t) async {
      final m = await fixture.fixture(
        intercept: (r) async => r.url.path.endsWith('/storage/usage')
            ? jsonResponse({
                'quota_bytes': 200000000,
                'used_bytes': 210000000,
                'reserved_bytes': 0,
                'remaining_bytes': 0,
                'state': 'exceeded',
              })
            : null,
      );
      await t.pumpWidget(
        fixture.harness(
          m,
          home: const Scaffold(
            body: SingleChildScrollView(child: NativeFreeStorage()),
          ),
        ),
      );
      await t.pumpAndSettle();
      expect(find.textContaining('210.00 MB'), findsOneWidget);
      expect(find.textContaining('200.00 MB'), findsOneWidget);
      expect(find.textContaining('剩余 0.00 MB'), findsOneWidget);
      expect(find.textContaining('不自动扣费'), findsOneWidget);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets('service information and history use backend documents', (
    t,
  ) async {
    final m = await fixture.fixture(
      signed: false,
      intercept: (r) async => r.url.path.contains('/service-documents')
          ? jsonResponse({'items': docs})
          : null,
    );
    await t.pumpWidget(fixture.harness(m, home: const NativeServicesScreen()));
    await t.pumpAndSettle();
    expect(find.textContaining('200,000,000字节'), findsOneWidget);
    expect(find.text('企业入驻与服务说明'), findsOneWidget);
    expect(find.textContaining('当前生效版本'), findsOneWidget);
    await t.pumpWidget(const SizedBox());
  });
  testWidgets(
    'anonymous complaint returns a credential and offers no company-rights choices',
    (t) async {
      Json? submitted;
      final m = await fixture.fixture(
        signed: false,
        intercept: (r) async {
          if (r.url.path.endsWith('/service-requests') && r.method == 'POST') {
            submitted = asMap(jsonDecode(r.body));
            return jsonResponse({
              'id': 7,
              ...submitted!,
              'status': 'received',
              'version': 1,
              'reply': '',
            });
          }
          return null;
        },
      );
      await t.pumpWidget(
        fixture.harness(m, home: const NativeServiceRequestScreen()),
      );
      await t.pumpAndSettle();
      final fields = find.byType(TextField);
      await t.enterText(fields.at(0), '企业图片投诉');
      await t.enterText(fields.at(1), '请核对图片权利。');
      final submit = find.widgetWithText(FilledButton, '提交申请');
      await t.scrollUntilVisible(
        submit,
        300,
        scrollable: find.byType(Scrollable).first,
      );
      await t.ensureVisible(submit);
      await t.tap(submit);
      await t.pumpAndSettle();
      expect(submitted?['credential'], matches(RegExp(r'^[a-f0-9]{64}$')));
      expect(submitted?['request_id'], matches(RegExp(r'^[a-f0-9]{64}$')));
      expect(submitted?['kind'], 'complaint');
      expect(find.textContaining('#7 · 企业图片投诉'), findsOneWidget);
      await t.pumpWidget(const SizedBox());
    },
  );
}
