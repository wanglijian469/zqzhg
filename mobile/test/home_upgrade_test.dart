import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'app_navigation_test.dart' as nav;

http.Response response(Object data, [int status = 200]) => http.Response(
  jsonEncode(data),
  status,
  headers: {'content-type': 'application/json; charset=utf-8'},
);

void viewport(WidgetTester tester, double width) {
  tester.view.physicalSize = Size(width, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}

void main() {
  test(
    'Product specifications use supplied parameters without inventing model fields',
    () {
      expect(
        productSpecification({'parameters': '品牌：冀丰\n型号：1LF-440\n重量：900kg'}),
        '型号：1LF-440',
      );
      expect(
        productSpecification({'name': '1LF-440', 'parameters': '品牌：冀丰'}),
        isEmpty,
      );
    },
  );

  testWidgets(
    'Home sends one read; initial search sends one filtered request; state survives tabs and language',
    (tester) async {
      viewport(tester, 390);
      final calls = <Uri>[];
      final m = await nav.fixture(signed: false, calls: calls);
      await tester.pumpWidget(nav.harness(m));
      await tester.pumpAndSettle();
      expect(
        calls.map((u) => u.path),
        unorderedEquals([
          '/api/v1/public/halls',
          '/api/v1/public/home',
          '/api/v1/public/taxonomy',
        ]),
      );
      expect(
        tester.getBottomLeft(find.byKey(const ValueKey('home-banner'))).dy -
            tester.getTopLeft(find.byKey(const ValueKey('home-banner'))).dy,
        inInclusiveRange(200, 270),
      );
      expect(find.byKey(const ValueKey('home-category-1')), findsNothing);
      await nav.tap(tester, find.byKey(const ValueKey('home-search-kind')));
      await nav.tap(tester, find.text('产品').last);
      await tester.enterText(find.byType(TextField).first, '1LF-440');
      await tester.testTextInput.receiveAction(TextInputAction.search);
      await tester.pumpAndSettle();
      final productReads = calls.where((u) => u.path.endsWith('/products'));
      expect(productReads, hasLength(1));
      expect(productReads.single.queryParameters['q'], '1LF-440');
      await nav.tap(tester, find.byKey(const ValueKey('app-tab-0')));
      await m.toggleLanguage();
      await tester.pumpAndSettle();
      await nav.tap(tester, find.byKey(const ValueKey('app-tab-2')));
      expect(
        tester.widget<TextField>(find.byType(TextField).first).controller!.text,
        '1LF-440',
      );
      expect(calls.where((u) => u.path.endsWith('/home')), hasLength(1));
      expect(calls.where((u) => u.path.endsWith('/taxonomy')), hasLength(1));
      expect(calls.where((u) => u.path.endsWith('/products')), hasLength(1));
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Directory debounces typing and submit does not repeat the same query',
    (tester) async {
      viewport(tester, 390);
      final calls = <Uri>[];
      final m = await nav.fixture(signed: false, calls: calls);
      await tester.pumpWidget(
        nav.harness(m, home: const BrowseScreen(kind: 'product')),
      );
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), '440');
      await tester.pump(const Duration(milliseconds: 349));
      expect(calls.where((u) => u.path.endsWith('/products')), hasLength(1));
      await tester.pump(const Duration(milliseconds: 1));
      await tester.pumpAndSettle();
      expect(calls.where((u) => u.path.endsWith('/products')), hasLength(1));
      await tester.testTextInput.receiveAction(TextInputAction.search);
      await tester.pumpAndSettle();
      expect(calls.where((u) => u.path.endsWith('/products')), hasLength(2));
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Home errors preserve search and banner; retry and pull refresh fetch fresh content',
    (tester) async {
      viewport(tester, 390);
      var reads = 0;
      final calls = <Uri>[];
      final m = await nav.fixture(
        signed: false,
        calls: calls,
        intercept: (r) async {
          if (!r.url.path.endsWith('/home')) return null;
          reads++;
          if (reads == 1) {
            return response({
              'error': {'code': 'network_error'},
            }, 503);
          }
          return response({'vendors': [], 'products': []});
        },
      );
      await tester.pumpWidget(nav.harness(m));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('home-search')), findsOneWidget);
      expect(find.byKey(const ValueKey('home-banner')), findsOneWidget);
      await nav.tap(tester, find.text('重试'));
      expect(reads, 2);
      await tester.drag(
        find.byType(CustomScrollView).first,
        const Offset(0, 500),
      );
      await tester.pumpAndSettle();
      expect(reads, 3);
      expect(calls.where((u) => u.path.endsWith('/taxonomy')), hasLength(1));
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Manufacturer name, photo and contact retain server showroom slug; favorite is independent',
    (tester) async {
      viewport(tester, 390);
      final m = await nav.fixture(signed: false);
      await tester.pumpWidget(nav.harness(m, home: const BrowseScreen()));
      await tester.pumpAndSettle();
      final card = find.byType(NativeVendorCard);
      expect(tester.getSize(card).height, lessThanOrEqualTo(202));
      await nav.tap(tester, find.byKey(const ValueKey('favorite-vendor-1')));
      expect(m.favorites.contains('vendor', 1), isTrue);
      expect(find.byType(CompanyShowroomScreen), findsNothing);
      await nav.tap(tester, find.byKey(const ValueKey('contact-vendor-1')));
      expect(
        tester
            .widget<CompanyShowroomScreen>(find.byType(CompanyShowroomScreen))
            .slug,
        'fixture',
      );
      expect(
        tester
            .widget<CompanyShowroomScreen>(find.byType(CompanyShowroomScreen))
            .initialTab,
        4,
      );
      await tester.tap(find.byType(BackButton).first);
      await tester.pumpAndSettle();
      await nav.tap(tester, find.text('河北冀农农机具有限公司'));
      expect(
        tester
            .widget<CompanyShowroomScreen>(find.byType(CompanyShowroomScreen))
            .initialTab,
        5,
      );
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Product rows align at 360dp with square contained photos and real model field',
    (tester) async {
      viewport(tester, 360);
      final m = await nav.fixture(
        signed: false,
        intercept: (r) async {
          if (!r.url.path.endsWith('/products')) return null;
          return response({
            'items': [
              {
                ...nav.product(),
                'id': 81,
                'data': {
                  'name': '设备',
                  'media_ids': [71],
                  'parameters': '型号：1LF-440',
                },
              },
              {
                ...nav.product(),
                'id': 82,
                'data': {
                  'name': '具有较长名称的工业产品及相关设备',
                  'media_ids': [72],
                },
              },
            ],
            'total': 2,
          });
        },
      );
      await tester.pumpWidget(
        nav.harness(m, home: const BrowseScreen(kind: 'product')),
      );
      await tester.pumpAndSettle();
      final cards = find.byType(ItemTile);
      expect(
        tester.getTopLeft(cards.at(0)).dy,
        tester.getTopLeft(cards.at(1)).dy,
      );
      expect(
        tester.getSize(cards.at(0)).height,
        tester.getSize(cards.at(1)).height,
      );
      expect(find.text('型号：1LF-440'), findsOneWidget);
      for (final photo in tester.widgetList<Photo>(find.byType(Photo))) {
        expect(photo.aspectRatio, 4 / 3);
        expect(photo.fit, BoxFit.contain);
      }
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Home caps recommendations at six each and does not request per-card details',
    (tester) async {
      viewport(tester, 390);
      final calls = <Uri>[];
      final m = await nav.fixture(
        signed: false,
        calls: calls,
        intercept: (r) async {
          if (!r.url.path.endsWith('/home')) return null;
          return response({
            'vendors': [
              for (var i = 1; i <= 12; i++) {...nav.company(), 'id': i},
            ],
            'products': [
              for (var i = 200; i < 208; i++) {...nav.product(), 'id': i},
            ],
          });
        },
      );
      await tester.pumpWidget(nav.harness(m));
      await tester.pumpAndSettle();
      final vendors = <int>{}, products = <int>{};
      for (var i = 0; i < 15; i++) {
        vendors.addAll(
          tester
              .widgetList<NativeVendorCard>(find.byType(NativeVendorCard))
              .map((v) => v.item['id'] as int),
        );
        products.addAll(
          tester
              .widgetList<ItemTile>(find.byType(ItemTile))
              .map((p) => p.item['id'] as int),
        );
        await tester.drag(
          find.byType(CustomScrollView).first,
          const Offset(0, -400),
        );
        await tester.pumpAndSettle();
      }
      expect(vendors, {1, 2, 3, 4});
      expect(products, {200, 201, 202, 203, 204, 205, 206, 207});
      expect(calls, hasLength(3));
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Manufacturer image priority reuses supplied media and products, then logo',
    (tester) async {
      viewport(tester, 390);
      final m = await nav.fixture(signed: false);
      final item = nav.company();
      item['data']['media_ids'] = <int>[];
      item['data']['company_profile'] = {
        'cover_media_id': 91,
        'logo_media_id': 93,
      };
      item['matching_products'] = [
        {
          ...nav.product(),
          'data': {
            'name': '设备',
            'media_ids': [92],
          },
        },
      ];
      await tester.pumpWidget(
        nav.harness(
          m,
          home: Scaffold(body: NativeVendorCard(item: item)),
        ),
      );
      await tester.pumpAndSettle();
      final first = tester.widget<Photo>(find.byType(Photo).first);
      expect(first.id, 91);
      expect(first.fallbackIDs, [92, 93]);
      expect(first.fit, BoxFit.contain);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );

  for (final width in [360.0, 390.0, 412.0]) {
    for (final language in ['zh', 'en']) {
      for (final scale in [1.0, 1.4, 2.0]) {
        testWidgets('Home and directory fit $width $language scale $scale', (
          tester,
        ) async {
          viewport(tester, width);
          final m = await nav.fixture(signed: false);
          m.language = language;
          await tester.pumpWidget(nav.harness(m, scale: scale));
          await tester.pumpAndSettle();
          expect(tester.takeException(), isNull);
          for (final tab in [1, 2, 3]) {
            await nav.tap(tester, find.byKey(ValueKey('app-tab-$tab')));
            expect(tester.takeException(), isNull);
          }
          await tester.pumpWidget(const SizedBox());
        });
      }
    }
  }
}
