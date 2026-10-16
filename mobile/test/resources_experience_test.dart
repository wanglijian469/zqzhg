import 'dart:convert';
import 'dart:io';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  Future<AppModel> model(
    Map<String, Object> prefs, {
    http.Client? client,
  }) async {
    SharedPreferences.setMockInitialValues(prefs);
    return AppModel(
      await SharedPreferences.getInstance(),
      {
        for (final lang in ['zh', 'en'])
          lang: asMap(
            jsonDecode(File('assets/i18n/$lang.json').readAsStringSync()),
          ),
      },
      client:
          client ??
          MockClient((_) async => http.Response('{"items":[],"total":0}', 200)),
    );
  }

  testWidgets(
    'Showroom defaults to portal, hides empty resource tabs and preserves product search',
    (tester) async {
      final calls = <Uri>[];
      final m = await model(
        {},
        client: MockClient((r) async {
          calls.add(r.url);
          dynamic data;
          if (r.url.path.contains('/showrooms/')) {
            data = {
              'item': {
                'id': 1,
                'showroom_slug': 'test',
                'data': {
                  'name': '测试厂商',
                  'name_en': 'Test vendor',
                  'main_products': '机械',
                  'region_name': '河北',
                  'description': '企业说明',
                  'phone': '13900000001',
                  'media_ids': [],
                  'company_profile': {'equipment': '生产线', 'capacity': '100台/月'},
                },
              },
              'categories': [
                {
                  'id': 1,
                  'name': '农业机械',
                  'name_en': 'Machinery',
                  'product_count': 45,
                },
              ],
              'product_count': 45,
              'resource_counts': {'video': 0, 'catalog': 0},
            };
          } else {
            data = {'items': [], 'total': 0, 'page': 1, 'page_size': 12};
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
          child: MaterialApp(home: const CompanyShowroomScreen(slug: 'test')),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(ChoiceChip), findsNWidgets(4));
      expect(
        (tester.widget<ChoiceChip>(
          find.widgetWithText(ChoiceChip, '展厅首页'),
        )).selected,
        isTrue,
      );
      await tester.tap(find.widgetWithText(ChoiceChip, '展品'));
      await tester.pumpAndSettle();
      await tester.scrollUntilVisible(
        find.byType(DropdownButtonFormField<int>),
        240,
        scrollable: find
            .descendant(
              of: find.byType(NestedScrollView),
              matching: find.byType(Scrollable),
            )
            .last,
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byType(DropdownButtonFormField<int>));
      await tester.pumpAndSettle();
      expect(find.text('农业机械 (45)'), findsOneWidget);
      await tester.tap(find.text('全部分类').last);
      await tester.pumpAndSettle();
      await tester.enterText(
        find.widgetWithText(TextField, '产品名称 / 型号'),
        '440',
      );
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(ChoiceChip, '企业介绍'));
      await tester.pumpAndSettle();
      await tester.scrollUntilVisible(
        find.textContaining('生产线'),
        180,
        scrollable: find
            .descendant(
              of: find.byType(NestedScrollView),
              matching: find.byType(Scrollable),
            )
            .last,
      );
      await tester.pumpAndSettle();
      expect(find.textContaining('生产线'), findsOneWidget);
      await tester.tap(find.widgetWithText(ChoiceChip, '展品'));
      await tester.pumpAndSettle();
      expect(find.text('440'), findsOneWidget);
      expect(calls.any((u) => u.queryParameters['q'] == '440'), isTrue);
      await tester.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Resource draft recovery retains uploaded IDs and files awaiting retry',
    (tester) async {
      final data = {
        'title': '原图册',
        'title_en': '',
        'description': '',
        'description_en': '',
        'purpose': '',
        'format': 'images',
        'product_id': 0,
        'file_media_id': 0,
        'cover_media_id': 0,
        'page_media_ids': [5, 6],
      };
      final m = await model({});
      final key = 'resource.draft.${m.scope}.catalog.0';
      await m.prefs.setString(
        key,
        jsonEncode({
          'revision_id': 0,
          'data': data,
          'uploads': [
            {'path': 'retained.pdf', 'name': '待重试文件', 'kind': 'pdf'},
          ],
        }),
      );
      await tester.pumpWidget(
        ModelScope(
          model: m,
          child: MaterialApp(home: const NativeResourceEditor(kind: 'catalog')),
        ),
      );
      await tester.pump();
      expect(find.text('原图册'), findsOneWidget);
      await tester.enterText(
        find.widgetWithText(TextField, '标题 / 图册名称'),
        '修改图册',
      );
      await tester.pump(const Duration(milliseconds: 100));
      final draft = asMap(jsonDecode(m.prefs.getString(key)!));
      expect(draft['data']['page_media_ids'], [5, 6]);
      expect(draft['uploads'][0]['path'], 'retained.pdf');
      expect(draft['data']['title'], '修改图册');
      await tester.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'A processing video preview waits instead of opening a broken player',
    (tester) async {
      final m = await model({});
      await tester.pumpWidget(
        ModelScope(
          model: m,
          child: MaterialApp(
            home: NativeResourceViewer(
              item: {
                'kind': 'video',
                'data': {
                  'title': '处理中视频',
                  'file_media_id': 8,
                  'description': '',
                  'product_id': 0,
                },
                'file': {'state': 'processing'},
              },
              private: true,
            ),
          ),
        ),
      );
      await tester.pump();
      expect(find.text('文件处理完成后可预览'), findsOneWidget);
      await tester.pumpWidget(const SizedBox());
    },
  );
  test(
    'Multipart upload reports monotonic progress including the final byte',
    () async {
      final events = <int>[];
      final request = ProgressMultipart(
        'POST',
        Uri.parse('http://localhost/upload'),
        (sent, total) {
          events.add(sent);
          expect(sent <= total, isTrue);
        },
      );
      request.files.add(
        http.MultipartFile.fromBytes(
          'file',
          List.filled(4096, 65),
          filename: 'clip.mp4',
        ),
      );
      final bytes = await request.finalize().toBytes();
      expect(events.last, bytes.length);
      expect(events, orderedEquals([...events]..sort()));
    },
  );
}
