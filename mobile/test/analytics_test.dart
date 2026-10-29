import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'app_navigation_test.dart' as fixture;

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test(
    'anonymous events use stable IDs, exclude credentials, and stop after three failures',
    () async {
      SharedPreferences.setMockInitialValues({
        'base_url': 'http://analytics.test',
        'service.analytics_consent': true,
      });
      final sent = <Json>[];
      var fail = false;
      var attempts = 0;
      final model = AppModel(
        await SharedPreferences.getInstance(),
        {'zh': {}, 'en': {}},
        testBaseUrl: 'http://analytics.test',
    analyticsEnabled: true,
        client: MockClient((r) async {
          attempts++;
          expect(r.headers.containsKey('Authorization'), isFalse);
          expect(r.url.path, '/api/v1/public/analytics/events');
          sent.addAll(rows(jsonDecode(r.body)['events']));
          if (fail) throw Exception('offline');
          return http.Response('{}', 202);
        }),
      );
      addTearDown(model.analytics.dispose);
      model.analytics.page('/');
      model.analytics.event('search', '/browse', keyword: '电话13900001234');
      await model.analytics.flush();
      expect(sent.map((e) => e['event_name']), ['page_view', 'search']);
      expect(sent.last['keyword'], '[已脱敏]');
      expect(sent.first['visitor_id'], sent.last['visitor_id']);
      expect(sent.first['event_id'], isNot(sent.last['event_id']));
      expect(sent.first['source'], 'android');
      model.analytics.event('page_view', '/admin/dashboard');
      await model.analytics.flush();
      expect(attempts, 1);
      fail = true;
      model.analytics.page('/products/4', kind: 'product', id: 4);
      await model.analytics.flush();
      await model.analytics.flush();
      await model.analytics.flush();
      await model.analytics.flush();
      expect(attempts, 4);
    },
  );
  testWidgets(
    'only visible tabs and opened detail routes emit page views; requests and rebuilds do not',
    (tester) async {
      final events = <Json>[];
      final model = await fixture.fixture(
        signed: false,
        analyticsEnabled: true,
        intercept: (r) async {
          if (r.url.path.endsWith('/public/analytics/events')) {
            events.addAll(rows(jsonDecode(r.body)['events']));
            return http.Response('{}', 202);
          }
          return null;
        },
      );
      addTearDown(model.analytics.dispose);
      await tester.pumpWidget(ExhibitionApp(model: model));
      await tester.pumpAndSettle();
      await tester.runAsync(() => model.analytics.flush());
      expect(
        events
            .where((e) => e['event_name'] == 'page_view')
            .map((e) => e['page']),
        ['/'],
      );
      await fixture.tap(tester, find.byKey(const ValueKey('app-tab-2')));
      await tester.runAsync(() => model.analytics.flush());
      expect(
        events
            .where((e) => e['event_name'] == 'page_view')
            .map((e) => e['page']),
        ['/', '/browse/products'],
      );
      model.toggleLanguage();
      await tester.pumpAndSettle();
      await tester.runAsync(() => model.analytics.flush());
      expect(events.where((e) => e['event_name'] == 'page_view').length, 2);
      await tester.pumpWidget(const SizedBox());
    },
  );
}
