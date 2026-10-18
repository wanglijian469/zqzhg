import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'app_navigation_test.dart' as nav;

void main() {
  testWidgets('App V1.7 large-text home layout', (tester) async {
    tester.view.physicalSize = const Size(360, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final original = FlutterError.onError;
    FlutterError.onError = (details) {
      FlutterError.dumpErrorToConsole(details, forceReport: true);
      original?.call(details);
    };
    addTearDown(() => FlutterError.onError = original);
    final m = await nav.fixture(signed: false);
    m.language = 'en';
    await tester.pumpWidget(nav.harness(m, scale: 2));
    await tester.pumpAndSettle();
    for (final tab in [1, 2, 3]) {
      await nav.tap(tester, find.byKey(ValueKey('app-tab-$tab')));
      expect(tester.takeException(), isNull);
    }
    await tester.pumpWidget(const SizedBox());
  });
  test('Dial links strip notes and support multiple real numbers', () {
    expect(dialNumbers('15031939188（王经理）'), ['15031939188']);
    expect(dialNumbers('0319-1234567 / 13800000001'), [
      '03191234567',
      '13800000001',
    ]);
    expect(dialNumbers('联系销售经理'), isEmpty);
  });
  testWidgets('Only a real 404 activates legacy public API compatibility', (
    tester,
  ) async {
    var status = 503;
    final calls = <Uri>[];
    final m = await nav.fixture(
      signed: false,
      calls: calls,
      intercept: (r) async {
        if (r.url.path.endsWith('/halls')) {
          return http.Response(
            jsonEncode({
              'error': {'code': 'internal_error'},
            }),
            status,
          );
        }
        return null;
      },
    );
    await m.loadHalls();
    expect(m.hallsReady, isFalse);
    expect(m.hallRegistry.legacy, isFalse);
    expect(calls.where((u) => u.path.endsWith('/home')), isEmpty);
    status = 404;
    await m.loadHalls(refresh: true);
    expect(m.hallsReady, isTrue);
    expect(m.hallRegistry.legacy, isTrue);
    expect(m.modernPublicApi, isFalse);
    await m.publicApi('/public/vendors?page_size=20');
    expect(calls.last.queryParameters['hall'], 'agriculture');
  });
  testWidgets('Hall public requests and taxonomy caches are isolated', (
    tester,
  ) async {
    final calls = <Uri>[];
    final m = await nav.fixture(
      signed: false,
      calls: calls,
      intercept: (r) async {
        if (r.url.path.endsWith('/halls')) {
          return http.Response(
            jsonEncode({
              'items': [
                {
                  'id': 1,
                  'slug': 'agriculture',
                  'name': '农业机械厅',
                  'name_en': 'Agriculture',
                  'status': 'open',
                },
                {
                  'id': 2,
                  'slug': 'cable',
                  'name': '电线电缆厅',
                  'name_en': 'Cable',
                  'status': 'open',
                },
              ],
            }),
            200,
            headers: {'content-type': 'application/json; charset=utf-8'},
          );
        }
        return null;
      },
    );
    await m.loadHalls();
    await m.publicTaxonomy();
    m.selectHall(m.availableHalls.last);
    await m.publicTaxonomy();
    expect(
      calls
          .where((u) => u.path.endsWith('/taxonomy'))
          .map((u) => u.queryParameters['hall']),
      ['agriculture', 'cable'],
    );
    m.selectHall(m.availableHalls.first);
    await m.publicTaxonomy();
    expect(calls.where((u) => u.path.endsWith('/taxonomy')), hasLength(2));
  });
  testWidgets(
    'Home products are bounded, unique and respect explicit manual clearing',
    (tester) async {
      final m = await nav.fixture(signed: false);
      final vendor = nav.company();
      for (final count in [12, 0]) {
        final products = List.generate(
          count,
          (i) => {...nav.product(), 'id': i ~/ 2 + 1},
        );
        await tester.pumpWidget(
          nav.harness(
            m,
            home: Scaffold(
              body: SingleChildScrollView(
                child: ShowroomHomeBody(
                  data: asMap(vendor['data']),
                  result: {
                    'item': vendor,
                    'home_products': products,
                    'representative_products': [nav.product()],
                    'product_count': 10,
                  },
                  onTab: (_) {},
                ),
              ),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byType(ItemTile), findsNWidgets(count == 0 ? 0 : 6));
        await tester.pumpWidget(const SizedBox());
      }
      await tester.pumpWidget(
        nav.harness(
          m,
          home: Scaffold(
            body: SingleChildScrollView(
              child: ShowroomHomeBody(
                data: asMap(vendor['data']),
                result: {
                  'item': vendor,
                  'home_products': List.generate(
                    12,
                    (i) => {...nav.product(), 'id': i + 1},
                  ),
                  'product_count': 12,
                },
                onTab: (_) {},
              ),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(ItemTile), findsNWidgets(8));
      await tester.pumpWidget(const SizedBox());
    },
  );
  testWidgets('Opening a ready video waits for an explicit play action', (
    tester,
  ) async {
    final calls = <Uri>[];
    final m = await nav.fixture(signed: false, calls: calls);
    await tester.pumpWidget(
      nav.harness(
        m,
        home: NativeResourceViewer(
          item: {
            'id': 4,
            'kind': 'video',
            'vendor_id': 1,
            'file': {'state': 'ready'},
            'data': {
              'title': 'Production demonstration',
              'file_media_id': 44,
              'cover_media_id': 0,
              'product_id': 0,
            },
          },
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('video-start')), findsOneWidget);
    expect(calls.where((u) => u.path.contains('/media/44')), isEmpty);
    expect(tester.takeException(), isNull);
    await tester.pumpWidget(const SizedBox());
  });
}
