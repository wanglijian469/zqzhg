import 'native_field_finder.dart';
import 'service_fixture.dart';
import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

http.Response jsonResponse(dynamic value, [int status = 200]) => http.Response(
  jsonEncode(value),
  status,
  headers: {'content-type': 'application/json; charset=utf-8'},
);
Future<AppModel> upgradeModel({
  bool signed = false,
  Future<http.Response?> Function(http.Request)? handler,
}) async {
  SharedPreferences.setMockInitialValues({
    'base_url': 'http://127.0.0.1:8080',
    'endpoint_migration.aliyun_39_97_250_244_8089': true,
    if (signed) 'token': 'isolated-fixture',
    if (signed) 'account': jsonEncode({'id': 7, 'role': 'merchant'}),
  });
  return AppModel(
    await SharedPreferences.getInstance(),
    {
      for (final lang in ['zh', 'en'])
        lang: asMap(
          jsonDecode(File('assets/i18n/$lang.json').readAsStringSync()),
        ),
    },
    testBaseUrl: 'http://127.0.0.1:8080',
    analyticsEnabled: false,
    client: MockClient((r) async {
      final custom = await handler?.call(r);
      if (custom != null) return custom;
      if (r.url.path.endsWith('/public/service-documents')) {
        return jsonResponse({'items': serviceFixtureDocuments()});
      }
      if (r.url.path.endsWith('/public/home')) {
        return jsonResponse({'vendors': [], 'products': [], 'hero': {}});
      }
      if (r.url.path.endsWith('/public/config')) {
        return jsonResponse({
          'capabilities': {
            'administrative_regions': true,
            'industry_map': true,
            'product_name_only': true,
            'phone_password_auth': true,
            'phone_sms_register': true,
            'phone_sms_bind': true,
            'phone_sms_auth': true,
            'sms_challenge': false,
          },
        });
      }
      if (r.url.path.endsWith('/public/halls')) {
        return jsonResponse({
          'items': [
            {
              'id': 1,
              'slug': 'agriculture',
              'name': '农业机械厅',
              'name_en': 'Agricultural Machinery',
              'status': 'open',
              'icon': 'tractor',
              'theme': 'blue',
            },
          ],
        });
      }
      if (r.url.path.endsWith('/public/administrative-regions')) {
        return jsonResponse({
          'items': [],
          'trail': [],
          'selected': null,
          'version': '2023-06-30',
        });
      }
      if (r.url.path.endsWith('/regions')) {
        return jsonResponse({
          'items': [
            {
              'code': '13',
              'name': '河北省',
              'vendor_count': 0,
              'has_children': true,
            },
          ],
          'trail': [],
          'total': 0,
          'unassigned_count': 0,
        });
      }
      if (r.url.path == '/maps/manifest.json') {
        return jsonResponse({
          'version': 1,
          'catalog_version': '2023-06-30',
          'resources': {},
        });
      }
      return jsonResponse({
        'items': [],
        'categories': [],
        'regions': [],
        'total': 0,
        'products': {},
      });
    }),
  );
}

Widget upgradeApp(
  AppModel m,
  Widget child, {
  double scale = 1,
  Key? boundary,
}) => ModelScope(
  model: m,
  child: AnimatedBuilder(
    animation: m,
    builder: (context, _) => MaterialApp(
      theme: industrialTheme().copyWith(
        textTheme: industrialTheme().textTheme.apply(
          fontFamily: Platform.environment['APP_V18_CAPTURE'] == '1'
              ? 'MobileAcceptance'
              : null,
        ),
      ),
      builder: (c, w) => MediaQuery(
        data: MediaQuery.of(c).copyWith(textScaler: TextScaler.linear(scale)),
        child: RepaintBoundary(key: boundary, child: w!),
      ),
      home: child,
    ),
  ),
);
Future<void> tapReachable(WidgetTester t, Finder finder) async {
  FocusManager.instance.primaryFocus?.unfocus();
  await t.pumpAndSettle();

  await t.ensureVisible(finder.first);
  await t.pumpAndSettle();
  await t.tap(finder.first);
  await t.pumpAndSettle();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  testWidgets(
    'Existing-account adoption is explicit and cancellation keeps the guest draft isolated',
    (t) async {
      int writes = 0;
      final m = await upgradeModel(
        handler: (request) async {
          if (request.url.path.endsWith('/merchant/contents') &&
              request.method == 'POST') {
            writes++;
            return jsonResponse({
              'id': 55,
              'revision_id': 1,
              'status': 'draft',
              'kind': 'vendor',
              'data': jsonDecode(request.body)['data'],
            });
          }
          return null;
        },
      );
      final guest = 'draft.${m.scope}.vendor';
      await m.prefs.setString(
        guest,
        jsonEncode({
          'revision_id': 0,
          'photos': [],
          'data': {
            'name': '保留访客资料（仅测试）',
            'main_products': '测试配件',
            'region_name': '历史地区文字',
            'phone': '13900009999',
          },
        }),
      );
      await t.pumpWidget(upgradeApp(m, const EditorScreen(kind: 'vendor')));
      await t.pumpAndSettle();
      await m.signIn({
        'token': 'test-only-token',
        'account': {'id': 7, 'role': 'merchant'},
      });
      await t.pumpAndSettle();
      await tapReachable(t, find.widgetWithText(OutlinedButton, '保存云端草稿'));
      expect(writes, 0);
      await tapReachable(t, find.widgetWithText(TextButton, '取消'));
      expect(m.prefs.containsKey(guest), true);
      expect(m.prefs.containsKey('draft.${m.scope}.vendor'), false);
      await tapReachable(t, find.widgetWithText(OutlinedButton, '保存云端草稿'));
      await tapReachable(t, find.text('使用这份草稿'));
      expect(writes, 1);
      expect(m.prefs.containsKey(guest), false);
      expect(m.prefs.containsKey('draft.${m.scope}.55'), true);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets('Expanded optional fields keep their focus while typing', (
    t,
  ) async {
    final m = await upgradeModel(signed: true);
    await t.pumpWidget(upgradeApp(m, const EditorScreen(kind: 'product')));
    await t.pumpAndSettle();
    await tapReachable(t, find.text('关键词、说明和参数（选填）'));
    final input = nativeField('关键词（含英文，选填）');
    await t.ensureVisible(input);
    await t.enterText(input, '实际关键词');
    await t.pumpAndSettle();
    expect(t.widget<TextField>(input).focusNode!.hasFocus, isTrue);
    expect(t.widget<TextField>(input).controller!.text, '实际关键词');
    await t.pumpWidget(const SizedBox());
  });
  testWidgets(
    'Region choice commits a city and never invents a district or saves just its province',
    (t) async {
      Json? selected;
      final m = await upgradeModel(
        handler: (request) async {
          if (request.url.path.endsWith('/administrative-regions')) {
            final parent = request.url.queryParameters['parent_code'] ?? '';
            return jsonResponse({
              'items': [
                if (parent == '')
                  {
                    'code': '13',
                    'name': '河北省',
                    'full_name': '河北省',
                    'level': 1,
                    'has_children': true,
                  },
                if (parent == '13')
                  {
                    'code': '1305',
                    'name': '邢台市',
                    'full_name': '河北省 邢台市',
                    'level': 2,
                    'has_children': true,
                  },
                if (parent == '1305')
                  {
                    'code': '130528',
                    'name': '宁晋县',
                    'full_name': '河北省 邢台市 宁晋县',
                    'level': 3,
                    'has_children': false,
                  },
              ],
            });
          }
          return null;
        },
      );
      await t.pumpWidget(
        upgradeApp(
          m,
          Scaffold(
            body: NativeRegionField(
              data: const {},
              onChanged: (value) => selected = value,
            ),
          ),
        ),
      );
      await t.pumpAndSettle();
      await tapReachable(t, find.text('选择省／市／区县'));
      await tapReachable(t, find.text('河北省'));
      expect(selected, isNull);
      await tapReachable(t, find.text('邢台市'));
      expect(selected!['region_code'], '1305');
      await tapReachable(t, find.text('仅保留当前城市，不细分区县'));
      expect(selected!['region_code'], '1305');
      expect(selected!['region_name'], '河北省 邢台市');
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Product categories follow its company hall rather than the public hall',
    (t) async {
      final m = await upgradeModel(
        signed: true,
        handler: (request) async {
          if (request.url.path.endsWith('/merchant/taxonomy')) {
            return jsonResponse({
              'categories': [
                {'id': 1, 'hall_id': 1, 'name': '农业分类', 'enabled': true},
                {'id': 2, 'hall_id': 2, 'name': '电缆分类', 'enabled': true},
              ],
            });
          }
          return null;
        },
      );
      await t.pumpWidget(
        upgradeApp(
          m,
          const EditorScreen(
            kind: 'product',
            item: {
              'id': 44,
              'hall_id': 2,
              'kind': 'product',
              'status': 'draft',
              'revision_id': 1,
              'data': {'name': '分类归属（仅测试）', 'category_id': 2, 'media_ids': []},
            },
          ),
        ),
      );
      await t.pumpAndSettle();
      final menu = t.widget<DropdownButton<int>>(
        find.byWidgetPredicate((widget) => widget is DropdownButton<int>),
      );
      expect(menu.items!.map((item) => item.value), [0, 2]);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets('Image order and cover changes persist without deleting media', (
    t,
  ) async {
    Json? saved;
    final calls = <String>[];
    final m = await upgradeModel(
      signed: true,
      handler: (request) async {
        calls.add('${request.method} ${request.url.path}');
        if (request.url.path.endsWith('/44/draft')) {
          saved = {
            'id': 44,
            'kind': 'product',
            'status': 'draft',
            'revision_id': 2,
            'data': jsonDecode(request.body)['data'],
          };
          return jsonResponse(saved);
        }
        return null;
      },
    );
    await t.pumpWidget(
      upgradeApp(
        m,
        const EditorScreen(
          kind: 'product',
          item: {
            'id': 44,
            'kind': 'product',
            'status': 'draft',
            'revision_id': 1,
            'data': {
              'name': '排序验收（仅测试）',
              'media_ids': [1, 2, 3],
            },
          },
        ),
      ),
    );
    await t.pumpAndSettle();
    await tapReachable(t, find.textContaining('2. 图片'));
    await tapReachable(t, find.text('设为封面').last);
    await tapReachable(t, find.widgetWithText(OutlinedButton, '保存云端草稿'));
    expect(saved!['data']['media_ids'], [3, 1, 2]);
    expect(calls.any((call) => call.startsWith('DELETE')), isFalse);
    await t.pumpWidget(const SizedBox());
  });
  testWidgets(
    'Newly registered companies restore guest text and only clear it after cloud saving',
    (t) async {
      Json? saved;
      final m = await upgradeModel(
        signed: true,
        handler: (request) async {
          if (request.url.path.endsWith('/merchant/contents') &&
              request.method == 'POST') {
            saved = {
              'id': 55,
              'kind': 'vendor',
              'status': 'draft',
              'revision_id': 1,
              'data': jsonDecode(request.body)['data'],
            };
            return jsonResponse(saved, 201);
          }
          return null;
        },
      );
      final guest = 'draft.${Uri.encodeComponent(m.baseUrl)}.guest.vendor';
      await m.prefs.setString(
        guest,
        jsonEncode({
          'revision_id': 0,
          'photos': [],
          'data': {
            'name': '访客恢复（仅测试）',
            'main_products': '测试配件',
            'region_name': '历史地区文字',
            'phone': '13900009999',
          },
        }),
      );
      m.newlyRegistered = true;
      await t.pumpWidget(upgradeApp(m, const EditorScreen(kind: 'vendor')));
      await t.pumpAndSettle();
      expect(find.text('访客恢复（仅测试）'), findsOneWidget);
      expect(m.prefs.containsKey(guest), isTrue);
      await tapReachable(t, find.widgetWithText(OutlinedButton, '保存云端草稿'));
      expect(saved!['data']['region_name'], '历史地区文字');
      expect(m.prefs.containsKey(guest), isFalse);
      expect(find.byType(EditorScreen), findsOneWidget);
      await t.pumpWidget(const SizedBox());
    },
  );
  test(
    'Capabilities distinguish explicit absence from network failure and evict failed requests',
    () async {
      int count = 0;
      final m = await upgradeModel(
        handler: (r) async {
          if (r.url.path.endsWith('/config')) {
            count++;
            if (count == 1) throw SocketException('isolated failure');
            return http.Response('old server', 404);
          }
          return null;
        },
      );
      await expectLater(m.clientCapabilities(), throwsA(isA<ApiFailure>()));
      expect(await m.clientCapabilities(), isEmpty);
      expect(count, 2);
      final other = AppModel(
        m.prefs,
        m.resources,
        client: m.client,
        testBaseUrl: 'http://other.test',
      );
      await other.clientCapabilities();
      expect(count, 3);
    },
  );
  test(
    'A delayed capability result remains scoped to its immutable model',
    () async {
      final pending = Completer<http.Response>();
      final m = await upgradeModel(
        handler: (r) async =>
            r.url.path.endsWith('/config') ? pending.future : null,
      );
      final request = m.clientCapabilities();
      final other = AppModel(
        m.prefs,
        m.resources,
        testBaseUrl: 'http://other.test',
      );
      pending.complete(
        jsonResponse({
          'capabilities': {'industry_map': true},
        }),
      );
      expect((await request)['industry_map'], true);
      expect(other.clientCapabilityCache, isEmpty);
    },
  );
  test(
    'Name-only product validation and suggestions never add invented values',
    () {
      expect(validateFormData({'name': '测试产品（仅测试）'}, 'product'), isEmpty);
      expect(
        nativeKeywordSuggestions({
          'name': '设备',
          'parameters': '型号：ABC\n重量：123',
          'keywords': '设备',
        }, '配件'),
        ['配件', 'ABC'],
      );
      expect(nativeParameterTemplate('agriculture', true), contains('重量：'));
      expect(
        nativeParameterTemplate('agriculture', true),
        isNot(contains('123')),
      );
      expect(
        () => showroomShareURL('javascript:alert(1)', 'https://example.org'),
        throwsA(isA<ApiFailure>()),
      );
      expect(
        () => showroomShareURL(
          'https://u:p@example.org/v/x',
          'https://example.org',
        ),
        throwsA(isA<ApiFailure>()),
      );
      expect(
        showroomShareURL('/v/x?private=1#draft', 'https://example.org'),
        'https://example.org/v/x',
      );
    },
  );
  testWidgets(
    'Unified registration keeps independent visibility and prevents duplicate uncertain requests',
    (t) async {
      int writes = 0;
      final pending = Completer<http.Response>();
      final m = await upgradeModel(
        handler: (r) async {
          if (r.url.path.endsWith('/register')) {
            writes++;
            return pending.future;
          }
          return null;
        },
      );
      await t.pumpWidget(upgradeApp(m, const LoginScreen(register: true)));
      await t.pumpAndSettle();
      await tapReachable(t, find.text('账号密码注册'));
      await t.enterText(
        find.byKey(const ValueKey('auth-username')),
        'isolated-account',
      );
      await t.enterText(
        find.byKey(const ValueKey('auth-accountPassword')),
        'Entry-pass-2026!',
      );
      await t.enterText(
        find.byKey(const ValueKey('auth-accountConfirm')),
        'Entry-pass-2026!',
      );
      await tapReachable(t, find.byTooltip('显示设置密码'));
      expect(
        t
            .widget<TextField>(
              find.byKey(const ValueKey('auth-accountPassword')),
            )
            .obscureText,
        isFalse,
      );
      expect(
        t
            .widget<TextField>(
              find.byKey(const ValueKey('auth-accountConfirm')),
            )
            .obscureText,
        isTrue,
      );
      await tapReachable(t, find.byType(CheckboxListTile));
      final submit = find.widgetWithText(FilledButton, '注册并进入展馆');
      await t.ensureVisible(submit);
      await t.tap(submit);
      await t.tap(submit);
      await t.pump();
      expect(writes, 1);
      pending.completeError(SocketException('test disconnected'));
      await t.pumpAndSettle();
      expect(m.token, isEmpty);
      expect(find.byKey(const ValueKey('auth-error')), findsOneWidget);
      await tapReachable(t, find.text('已有账号，登录'));
      expect(
        t
            .widget<TextField>(find.byKey(const ValueKey('auth-loginPassword')))
            .controller!
            .text,
        isEmpty,
      );
      expect(
        m.prefs.getKeys().any(
          (k) => '${m.prefs.get(k)}'.contains('Entry-pass'),
        ),
        isFalse,
      );
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'A name-only product saves once, stays in the same editor and resumes its cloud ID',
    (t) async {
      int creates = 0, updates = 0;
      Json? saved;
      final m = await upgradeModel(
        signed: true,
        handler: (r) async {
          if (r.url.path.endsWith('/merchant/contents') && r.method == 'POST') {
            creates++;
            saved = {
              'id': 44,
              'kind': 'product',
              'status': 'draft',
              'revision_id': 1,
              'data': jsonDecode(r.body)['data'],
            };
            return jsonResponse(saved, 201);
          }
          if (r.url.path.endsWith('/44/draft')) {
            updates++;
            saved = {
              ...saved!,
              'revision_id': 2,
              'data': jsonDecode(r.body)['data'],
            };
            return jsonResponse(saved);
          }
          return null;
        },
      );
      await t.pumpWidget(upgradeApp(m, const EditorScreen(kind: 'product')));
      await t.pumpAndSettle();
      await t.enterText(nativeField('产品名称'), '入口验收产品（仅测试）');
      await tapReachable(t, find.widgetWithText(OutlinedButton, '保存云端草稿'));
      expect(creates, 1);
      expect(find.byType(EditorScreen), findsOneWidget);
      await t.pumpWidget(const SizedBox());
      await t.pumpWidget(upgradeApp(m, const EditorScreen(kind: 'product')));
      await t.pumpAndSettle();
      await tapReachable(t, find.widgetWithText(OutlinedButton, '保存云端草稿'));
      expect(creates, 1);
      expect(updates, 1);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Old-server name-only save is explained without inventing a category',
    (t) async {
      int writes = 0;
      final m = await upgradeModel(
        signed: true,
        handler: (r) async {
          if (r.url.path.endsWith('/config')) return jsonResponse({});
          if (r.method == 'POST') {
            writes++;
          }
          return null;
        },
      );
      await t.pumpWidget(upgradeApp(m, const EditorScreen(kind: 'product')));
      await t.pumpAndSettle();
      await t.enterText(nativeField('产品名称'), '旧服务兼容（仅测试）');
      await tapReachable(t, find.widgetWithText(OutlinedButton, '保存云端草稿'));
      expect(writes, 0);
      expect(find.textContaining('当前服务器尚未支持'), findsOneWidget);
      await t.pumpWidget(const SizedBox());
    },
  );
}
