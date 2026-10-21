import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'mobile_upgrade_test.dart' as fixture;

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  for (final kind in ['vendor', 'product']) {
    testWidgets(
      '$kind browse discards obsolete hidden filters and cached page',
      (tester) async {
        final requests = <Uri>[];
        final m = await fixture.upgradeModel(
          handler: (r) async {
            if (r.url.path.endsWith('/taxonomy')) {
              return fixture.jsonResponse({
                'categories': [
                  {
                    'id': 101,
                    'name': '整机与作业机械',
                    'name_en': 'Machinery',
                    'parent_id': 0,
                  },
                ],
                'regions': [],
                'industrial_belts': [
                  {'id': 77, 'name': '不再检索的产业带'},
                ],
              });
            }
            if (r.url.path.endsWith(
              kind == 'vendor' ? '/vendors' : '/products',
            )) {
              requests.add(r.url);
              return fixture.jsonResponse({
                'items': [],
                'total': 0,
                'page': 1,
                'page_size': 20,
              });
            }
            return null;
          },
        );
        await m.loadHalls();
        final scope = '${m.baseUrl}:agriculture:$kind';
        m.browseMemory[scope] = {
          'query': '保留关键词',
          'term': '保留关键词',
          'category': 101,
          'belt': 77,
          'business_nature': 'manufacturer',
          'page': 3,
        };
        await tester.pumpWidget(
          fixture.upgradeApp(m, BrowseScreen(kind: kind)),
        );
        await tester.pumpAndSettle();
        expect(requests, isNotEmpty);
        for (final uri in requests) {
          expect(
            uri.queryParameters.containsKey('industrial_belt_id'),
            isFalse,
          );
          expect(uri.queryParameters.containsKey('business_nature'), isFalse);
          expect(uri.queryParameters['page'], '1');
          expect(uri.queryParameters['q'], '保留关键词');
          expect(uri.queryParameters['category_id'], '101');
        }
        expect(find.text('产业带'), findsNothing);
        expect(find.text('经营性质'), findsNothing);
        expect(tester.takeException(), isNull);
        await tester.pumpWidget(const SizedBox());
        expect(m.browseMemory[scope]!.containsKey('belt'), isFalse);
        expect(m.browseMemory[scope]!.containsKey('business_nature'), isFalse);
      },
    );
  }
}
