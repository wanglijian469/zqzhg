import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'app_navigation_test.dart' as fixture;
import 'native_field_finder.dart';
import 'service_fixture.dart';

http.Response reply(dynamic body, [int status = 200]) => http.Response(
  jsonEncode(body),
  status,
  headers: {'content-type': 'application/json; charset=utf-8'},
);

void main() {
  test(
    'copy creates independent owned text and images without publishing or resource state',
    () {
      final original = <String, dynamic>{
        'name': '原产品',
        'media_ids': [71, 72],
        'applicable_models': ['A'],
        'description': '说明',
        'review_note': 'approved',
        'revision_id': 31,
        'videos': [5],
        'public_visible': true,
        'company_profile': {'phone': '13900000000'},
      };
      final copy = cloneProductDraft(original);
      expect(
        copy.keys,
        unorderedEquals([
          'name',
          'media_ids',
          'applicable_models',
          'description',
        ]),
      );
      (copy['media_ids'] as List).removeLast();
      expect(original['media_ids'], [71, 72]);
    },
  );

  testWidgets(
    'product first step is name category photos; details survive steps, folds and language',
    (t) async {
      final m = await fixture.fixture(
        intercept: (r) async => r.url.path.endsWith('/public/config')
            ? reply({
                'capabilities': {
                  'product_name_only': true,
                  'content_create_idempotency': true,
                },
              })
            : null,
      );
      await t.pumpWidget(
        fixture.harness(m, home: const EditorScreen(kind: 'product')),
      );
      await t.pumpAndSettle();
      expect(find.text('1 添加产品'), findsOneWidget);
      expect(find.text('产品图片'), findsOneWidget);
      expect(nativeField('产品品牌'), findsNothing);
      await t.enterText(nativeField(m.t('productName')), '翻转犁😀');
      await t.tap(find.text('2 完善详情'));
      await t.pumpAndSettle();
      await t.enterText(nativeField('产品品牌'), '冀农');
      await t.tap(find.text('1 添加产品'));
      await t.pumpAndSettle();
      expect(
        t.widget<TextField>(nativeField(m.t('productName'))).controller!.text,
        '翻转犁😀',
      );
      await t.tap(find.text('2 完善详情'));
      await t.pumpAndSettle();
      expect(t.widget<TextField>(nativeField('产品品牌')).controller!.text, '冀农');
      await m.toggleLanguage();
      await t.pumpAndSettle();
      expect(
        t.widget<TextField>(nativeField('Product brand')).controller!.text,
        '冀农',
      );
      await t.pump(const Duration(milliseconds: 300));
      final cached = asMap(
        jsonDecode(m.prefs.getString('draft.${m.scope}.product')!),
      );
      expect(cached['data']['name'], '翻转犁😀');
      expect(cached['data']['brand'], '冀农');
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'uncertain creation retries original request before saving newer edits',
    (t) async {
      final creates = <Json>[], updates = <Json>[];
      final m = await fixture.fixture(
        intercept: (r) async {
          if (r.url.path.endsWith('/public/config')) {
            return reply({
              'capabilities': {
                'product_name_only': true,
                'content_create_idempotency': true,
              },
            });
          }
          if (r.method == 'POST' && r.url.path.endsWith('/merchant/contents')) {
            final body = asMap(jsonDecode(r.body));
            creates.add(body);
            if (creates.length == 1) {
              return reply({
                'error': {'code': 'network_error'},
              }, 503);
            }
            return reply({
              'id': 50,
              'revision_id': 1,
              'kind': 'product',
              'status': 'draft',
              'data': body['data'],
            });
          }
          if (r.method == 'PUT' && r.url.path.endsWith('/50/draft')) {
            final body = asMap(jsonDecode(r.body));
            updates.add(body);
            return reply({
              'id': 50,
              'revision_id': 2,
              'kind': 'product',
              'status': 'draft',
              'data': body['data'],
            });
          }
          return null;
        },
      );
      await t.pumpWidget(
        fixture.harness(m, home: const EditorScreen(kind: 'product')),
      );
      await t.pumpAndSettle();
      await t.enterText(nativeField(m.t('productName')), '第一次名称');
      await t.tap(find.widgetWithText(OutlinedButton, m.t('saveCloud')));
      await t.pumpAndSettle();
      expect(creates, hasLength(1));
      final key = creates.first['client_request_id'];
      expect(key, isNotEmpty);
      await t.enterText(nativeField(m.t('productName')), '新编辑名称');
      await t.tap(find.widgetWithText(OutlinedButton, m.t('saveCloud')));
      await t.pumpAndSettle();
      expect(creates, hasLength(2));
      expect(creates[1], creates[0]);
      expect(updates.single['data']['name'], '新编辑名称');
      expect(updates.single['revision_id'], 1);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'contact section saves merged full snapshot with revision and hides accepted agreements',
    (t) async {
      final company = fixture.company();
      company['data']['company_profile'] = {
        'established': '1985',
        'cover_media_id': 0,
        'equipment': '原设备',
      };
      Json? update;
      final docs = serviceFixtureDocuments()
          .where((d) => d['key'] != 'service-info')
          .toList();
      final m = await fixture.fixture(
        intercept: (r) async {
          if (r.url.path.endsWith('/merchant/verification')) {
            return reply({
              'documents': docs,
              'acceptances': [
                for (final d in docs)
                  {'document_id': d['id'], 'digest': d['digest']},
              ],
            });
          }
          if (r.method == 'PUT') {
            update = asMap(jsonDecode(r.body));
            return reply({
              ...company,
              'revision_id': 13,
              'data': update!['data'],
            });
          }
          return null;
        },
      );
      await t.pumpWidget(
        fixture.harness(
          m,
          home: EditorScreen(
            kind: 'vendor',
            item: company,
            initialSection: 'contact',
          ),
        ),
      );
      await t.pumpAndSettle();
      expect(find.text('服务协议确认'), findsNothing);
      expect(find.text('1 企业资料'), findsNothing);
      await t.enterText(nativeField(m.t('phone')), '13911112222');
      await t.tap(find.widgetWithText(OutlinedButton, m.t('saveCloud')));
      await t.pumpAndSettle();
      expect(update!['revision_id'], 12);
      expect(update!['data']['phone'], '13911112222');
      expect(update!['data']['description'], '农业机械制造与生产。');
      expect(update!['data']['company_profile']['equipment'], '原设备');
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'existing company opens six-entry management home instead of admission wizard',
    (t) async {
      final m = await fixture.fixture();
      await t.pumpWidget(
        fixture.harness(
          m,
          home: EditorScreen(kind: 'vendor', item: fixture.company()),
        ),
      );
      await t.pumpAndSettle();
      for (final label in ['企业资料', '封面与图片', '企业视频', '产品管理', '联系方式', '更多资料']) {
        expect(find.text(label), findsOneWidget);
      }
      expect(find.text('协议与提交'), findsNothing);
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
    },
  );
}
