import 'native_field_finder.dart';
import 'legacy_hall_fixture.dart';
import 'dart:convert';
import 'dart:io';
import 'dart:ui' as ui;
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  Future<AppModel> model({
    Future<http.Response> Function(http.Request)? handler,
    bool signed = false,
  }) async {
    SharedPreferences.setMockInitialValues({
      'base_url': 'http://fixture',
      if (signed) 'token': 'fixture-token',
      if (signed) 'account': jsonEncode({'id': 7}),
    });
    return AppModel(
      await SharedPreferences.getInstance(),
      {
        for (final lang in ['zh', 'en'])
          lang: asMap(
            jsonDecode(File('assets/i18n/$lang.json').readAsStringSync()),
          ),
      },
      testBaseUrl: 'http://fixture',
      client: legacyHallClient(
        MockClient(
          handler ??
              (_) async => response({
                'items': [],
                'total': 0,
                'categories': [],
                'regions': [],
                'products': {},
              }),
        ),
      ),
    );
  }

  Json company() => {
    'name': '隔离验收制造企业',
    'name_en': 'Fixture manufacturer',
    'main_products': '农业设备',
    'region_name': '河北',
    'phone': '0319-1234567',
    'description': '企业介绍\n保留段落',
    'media_ids': <int>[],
    'company_profile': <String, dynamic>{},
  };
  Widget app(AppModel m, Widget home, {double scale = 1, Key? boundary}) =>
      ModelScope(
        model: m,
        child: MaterialApp(
          debugShowCheckedModeBanner: false,
          theme: industrialTheme().copyWith(
            textTheme: industrialTheme().textTheme.apply(
              fontFamily: 'Microsoft YaHei',
            ),
          ),
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(scale)),
            child: child!,
          ),
          home: RepaintBoundary(key: boundary, child: home),
        ),
      );
  Future<void> click(WidgetTester tester, Finder finder) async {
    if (finder.evaluate().isEmpty) {
      await tester.scrollUntilVisible(
        finder,
        350,
        scrollable: find.byType(Scrollable).first,
      );
    }
    await tester.ensureVisible(finder);
    await tester.pumpAndSettle();
    await tester.runAsync(() async {
      await tester.tap(finder);
      await Future<void>.delayed(const Duration(milliseconds: 120));
    });
    await tester.pumpAndSettle();
  }

  testWidgets(
    'Sparse portal hides empty sections and empty resource navigation',
    (tester) async {
      final m = await model(
        handler: (_) async => response({
          'item': {'id': 1, 'data': company()},
          'product_count': 0,
          'resource_counts': {'video': 0, 'catalog': 0},
          'representative_products': [],
        }),
      );
      await tester.pumpWidget(
        app(m, const CompanyShowroomScreen(slug: 'fixture')),
      );
      await tester.pumpAndSettle();
      expect(find.byType(ChoiceChip), findsNWidgets(3));
      expect(
        tester
            .widget<ChoiceChip>(find.widgetWithText(ChoiceChip, '展厅首页'))
            .selected,
        isTrue,
      );
      expect(find.text('企业资质'), findsNothing);
      expect(find.text('工厂与生产能力'), findsNothing);
      expect(find.text('暂无企业介绍'), findsNothing);
      expect(tester.takeException(), isNull);
    },
  );
  testWidgets(
    'Homepage landmark uses platform media and vendor cards keep their own covers',
    (tester) async {
      final d = company();
      d['company_profile'] = {'cover_media_id': 91};
      final m = await model(
        handler: (r) async => response(
          r.url.path.endsWith('/home')
              ? {
                  'vendors': [
                    {'id': 1, 'kind': 'vendor', 'data': d},
                  ],
                  'products': [],
                  'hero': {'media_id': 22},
                }
              : {'categories': []},
        ),
      );
      await tester.pumpWidget(app(m, const IndustrialHomeScreen()));
      await tester.pumpAndSettle();
      final photos = tester.widgetList<Photo>(find.byType(Photo));
      expect(photos.any((p) => p.id == 22 && !p.thumb), isTrue);
      expect(photos.any((p) => p.id == 91), isTrue);
      expect(tester.takeException(), isNull);
    },
  );
  testWidgets(
    'Manual representatives retain choices on version conflict, reorder and clear',
    (tester) async {
      final writes = <Json>[];
      var configVersion = 1;
      final product = <String, dynamic>{
        'id': 11,
        'kind': 'product',
        'public_visible': true,
        'data': {'name': '公开旧版产品', 'media_ids': []},
      };
      final m = await model(
        signed: true,
        handler: (r) async {
          if (r.url.path.endsWith('/showroom-display')) {
            if (r.method == 'PUT') {
              final data = asMap(jsonDecode(r.body));
              writes.add(data);
              if (data['version'] == 1) {
                configVersion = 2;
                return response({
                  'error': {'code': 'stale_revision'},
                }, 409);
              }
            }
            return response({
              'vendor_id': 1,
              'version': configVersion,
              'mode': 'manual',
              'product_ids': [11],
              'selected': [
                {'product_id': 11, 'public_visible': true, 'item': product},
              ],
              'preview_products': [product],
            });
          }
          expect(r.url.path, endsWith('/public/products'));
          expect(r.url.queryParameters['vendor_id'], '1');
          return response({
            'items': [product],
            'total': 1,
          });
        },
      );
      await tester.pumpWidget(app(m, const RepresentativeSettingsScreen()));
      await tester.pumpAndSettle();
      await click(tester, find.widgetWithText(FilledButton, '保存设置精选产品'));
      expect(find.textContaining('配置已更新'), findsOneWidget);
      expect(
        tester.widget<CheckboxListTile>(find.byType(CheckboxListTile)).value,
        isTrue,
      );
      await click(tester, find.widgetWithText(OutlinedButton, '刷新配置版本'));
      await click(tester, find.widgetWithText(TextButton, '移除').first);
      await click(tester, find.widgetWithText(FilledButton, '保存设置精选产品'));
      expect(writes.last['product_ids'], isEmpty);
      expect(writes.last['version'], 2);
      expect(tester.takeException(), isNull);
    },
  );
  testWidgets(
    'Registration continues original cloud save and does not store passwords',
    (tester) async {
      final saved = <Json>[];
      final m = await model(
        handler: (r) async {
          if (r.url.path.endsWith('/public/config')) {
            return response({
              'capabilities': {'phone_password_auth': true},
            });
          }
          if (r.url.path.endsWith('/register')) {
            return response({
              'token': 'registered-token',
              'account': {'id': 7},
            }, 201);
          }
          if (r.url.path.endsWith('/contents') && r.method == 'POST') {
            final body = asMap(jsonDecode(r.body));
            saved.add(body);
            return response({
              'id': 8,
              'revision_id': 9,
              'status': 'draft',
              'data': body['data'],
            });
          }
          return response({
            'categories': [],
            'regions': [],
            'items': [],
            'total': 0,
            'products': {},
          });
        },
      );
      final guestKey = 'draft.${m.scope}.vendor';
      await m.prefs.setString(
        guestKey,
        jsonEncode({'revision_id': 0, 'data': company(), 'photos': []}),
      );
      await tester.pumpWidget(app(m, const EditorScreen(kind: 'vendor')));
      await tester.pumpAndSettle();
      await click(tester, find.widgetWithText(OutlinedButton, '保存云端草稿'));
      expect(find.byType(LoginScreen), findsOneWidget);
      await tester.enterText(
        find.byKey(const ValueKey('auth-login_name')),
        'company-fixture-9991',
      );
      await tester.enterText(nativeField('设置密码'), 'fixture-pass-123');
      await tester.enterText(nativeField('确认密码'), 'fixture-pass-123');
      await tester.ensureVisible(find.byType(CheckboxListTile));
      await tester.tap(find.byType(CheckboxListTile));
      await tester.pumpAndSettle();
      await click(tester, find.widgetWithText(FilledButton, '注册并继续'));
      await tester.runAsync(() async {
        await Future<void>.delayed(const Duration(milliseconds: 300));
      });
      await tester.pumpAndSettle();
      expect(saved, hasLength(1));
      expect(saved.single['data']['name'], company()['name']);
      expect(m.account['id'], 7);
      expect(
        m.prefs.getKeys().any(
          (k) => (m.prefs.get(k) ?? '').toString().contains('fixture-pass-123'),
        ),
        isFalse,
      );
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Top-level category sends parent zero and selects returned category',
    (tester) async {
      Json? created;
      int? selected;
      final m = await model(
        signed: true,
        handler: (r) async {
          if (r.method == 'POST') {
            created = asMap(jsonDecode(r.body));
            return response({'id': 90, 'name': '新一级分类'});
          }
          return response({
            'categories': created == null
                ? []
                : [
                    {
                      'id': 90,
                      'name': '新一级分类',
                      'enabled': true,
                      'suggestion_status': 'pending',
                    },
                  ],
          });
        },
      );
      await tester.pumpWidget(
        app(
          m,
          Scaffold(
            body: NativeCategoryPicker(
              value: 0,
              enabled: true,
              onChanged: (id) => selected = id,
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await click(tester, find.text('＋ 新增经营分类'));
      expect(find.text('顶级（一级）分类'), findsOneWidget);
      await tester.enterText(nativeField('中文分类名称'), '新一级分类');
      await click(tester, find.widgetWithText(FilledButton, '保存'));
      expect(created?['parent_id'], 0);
      expect(selected, 90);
      expect(tester.takeException(), isNull);
    },
  );
  test(
    'Portal validations use Unicode limits and preserve unknown profile fields',
    () {
      final d = company();
      d['company_profile'] = {
        'custom_future_field': 'keep',
        'credentials': [
          {'kind': 'qualification', 'media_id': 1, 'title': '资质'},
        ],
      };
      final edited = credentialEditorData(d);
      expect(edited['company_profile']['custom_future_field'], 'keep');
      edited['company_profile']['credentials'][0]['title'] = List.filled(
        101,
        '😀',
      ).join();
      expect(
        validateFormData(
          edited,
          'vendor',
        )['company_profile.credentials.0.title']?['limit'],
        100,
      );
      expect(
        companyGallery({
          ...d,
          'media_ids': [1, 2],
          'company_profile': {
            'gallery_annotations': [
              {'media_id': 2, 'title': '第二张'},
            ],
          },
        }).last['title'],
        '第二张',
      );
    },
  );
  testWidgets(
    'Upload failure retains files, retry skips successful files and submit failure keeps content ID',
    (tester) async {
      final dir = Directory('../.tools/tmp/android-sync-test')
        ..createSync(recursive: true);
      final file = File('${dir.path}/fixture.png')
        ..writeAsBytesSync(
          base64Decode(
            'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl6LwAAAABJRU5ErkJggg==',
          ),
        );
      int upload = 0, creates = 0, updates = 0, submits = 0;
      final m = await model(
        signed: true,
        handler: (r) async {
          if (r.url.path.endsWith('/media')) {
            upload++;
            return upload == 2
                ? response({
                    'error': {'code': 'internal_error'},
                  }, 500)
                : response({'id': 100 + upload});
          }
          if (r.method == 'POST' && r.url.path.endsWith('/contents')) {
            creates++;
            return response({
              'id': 8,
              'revision_id': 9,
              'status': 'draft',
              'data': jsonDecode(r.body)['data'],
            });
          }
          if (r.method == 'PUT') {
            updates++;
            return response({
              'id': 8,
              'revision_id': 10,
              'status': 'draft',
              'data': jsonDecode(r.body)['data'],
            });
          }
          if (r.url.path.endsWith('/submit')) {
            submits++;
            return submits == 1
                ? response({
                    'error': {'code': 'internal_error'},
                  }, 500)
                : response({'ok': true});
          }
          return response({
            'categories': [],
            'regions': [],
            'items': [],
            'total': 0,
            'products': {},
          });
        },
      );
      final key = 'draft.${m.scope}.vendor';
      await m.prefs.setString(
        key,
        jsonEncode({
          'step': 2,
          'revision_id': 0,
          'data': company(),
          'photos': [
            {'path': file.absolute.path},
            {'path': file.absolute.path},
          ],
        }),
      );
      await tester.pumpWidget(app(m, const EditorScreen(kind: 'vendor')));
      await tester.pumpAndSettle();
      Future<void> waitForIO(bool Function() completed) async {
        for (var i = 0; i < 100 && !completed(); i++) {
          await tester.runAsync(
            () => Future<void>.delayed(const Duration(milliseconds: 20)),
          );
          await tester.pump();
        }
        expect(completed(), isTrue);
        await tester.pumpAndSettle();
      }

      await click(tester, find.widgetWithText(FilledButton, '提交审核'));
      await waitForIO(
        () =>
            upload == 2 &&
            find.widgetWithText(FilledButton, '提交审核').evaluate().isNotEmpty &&
            tester
                    .widget<FilledButton>(
                      find.widgetWithText(FilledButton, '提交审核'),
                    )
                    .onPressed !=
                null,
      );
      expect(upload, 2);
      expect(creates, 0);
      expect(jsonDecode(m.prefs.getString(key)!)['photos'][0]['id'], 101);
      await click(tester, find.widgetWithText(FilledButton, '提交审核'));
      await waitForIO(() => upload == 3 && submits == 1);
      expect(upload, 3);
      expect(creates, 1);
      expect(submits, 1);
      await click(tester, find.widgetWithText(FilledButton, '提交审核'));
      await waitForIO(() => submits == 2);
      expect(upload, 3);
      expect(creates, 1);
      expect(updates, 1);
      expect(submits, 2);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Session expiry preserves owner; signing in to another account does not carry editor data',
    (tester) async {
      final m = await model(signed: true);
      await tester.pumpWidget(app(m, const EditorScreen(kind: 'vendor')));
      await tester.pumpAndSettle();
      await tester.enterText(nativeField('企业 / 工厂名称'), '账号七私有草稿');
      await tester.pump(const Duration(milliseconds: 350));
      final oldScope = m.scope;
      await m.expireSession();
      await tester.pumpAndSettle();
      expect(m.scope, oldScope);
      await m.signIn({
        'token': 'new-token',
        'account': {'id': 9},
      });
      await tester.pumpAndSettle();
      expect(find.byType(EditorScreen), findsNothing);
      expect(m.prefs.getString('draft.$oldScope.vendor'), contains('账号七私有草稿'));
      expect(m.prefs.getString('draft.${m.scope}.vendor'), isNull);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Resource editor preserves files on expiry and exits after account change',
    (tester) async {
      final m = await model(signed: true);
      await tester.pumpWidget(
        app(m, const NativeResourceEditor(kind: 'catalog')),
      );
      await tester.pumpAndSettle();
      await tester.enterText(nativeField('标题 / 图册名称'), '账号七的私有图册');
      await tester.pump(const Duration(milliseconds: 350));
      final key = 'resource.draft.${m.scope}.catalog.0';
      expect(m.prefs.getString(key), contains('账号七的私有图册'));
      await m.expireSession();
      await tester.pumpAndSettle();
      expect(find.byType(NativeResourceEditor), findsOneWidget);
      await m.signIn({
        'token': 'account-nine-token',
        'account': {'id': 9},
      });
      await tester.pumpAndSettle();
      expect(find.byType(NativeResourceEditor), findsNothing);
      expect(m.prefs.getString(key), contains('账号七的私有图册'));
      expect(m.prefs.getString('resource.draft.${m.scope}.catalog.0'), isNull);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Portal layout and image failures at phone/tablet sizes, bilingual and large text',
    (tester) async {
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final m = await model();
      final d = company();
      d['media_ids'] = [8];
      d['company_profile'] = {
        'equipment': '生产线',
        'credentials': [
          {
            'kind': 'qualification',
            'media_id': 9,
            'title': '完整证书',
            'title_en': 'Full certificate',
          },
        ],
        'gallery_annotations': [
          {'media_id': 8, 'title': '工厂实景'},
        ],
      };
      final boundary = GlobalKey();
      await tester.runAsync(() async {
        final icons = FontLoader('MaterialIcons');
        icons.addFont(
          Future.value(
            ByteData.sublistView(
              File(
                '../.tools/flutter/bin/cache/artifacts/material_fonts/MaterialIcons-Regular.otf',
              ).readAsBytesSync(),
            ),
          ),
        );
        await icons.load();
        final loader = FontLoader('Microsoft YaHei');
        loader.addFont(
          Future.value(
            ByteData.sublistView(
              File('C:/Windows/Fonts/msyh.ttc').readAsBytesSync(),
            ),
          ),
        );
        await loader.load();
      });
      for (final width in [360.0, 390.0, 768.0]) {
        for (final language in ['zh', 'en']) {
          m.language = language;
          tester.view.physicalSize = Size(width, 844);
          tester.view.devicePixelRatio = 1;
          await tester.pumpWidget(
            app(
              m,
              Scaffold(
                body: SingleChildScrollView(
                  child: Padding(
                    padding: const EdgeInsets.all(20),
                    child: CompanyPortalBody(data: d, compact: true),
                  ),
                ),
              ),
              scale: 1.4,
              boundary: boundary,
            ),
          );
          await tester.pumpAndSettle();
          expect(tester.takeException(), isNull, reason: '$width $language');
          await tester.runAsync(() async {
            final render =
                boundary.currentContext!.findRenderObject()
                    as RenderRepaintBoundary;
            final image = await render.toImage();
            final bytes = await image.toByteData(
              format: ui.ImageByteFormat.png,
            );
            final output = File(
              '../docs/screenshots/android-sync-portal-${width.toInt()}-$language.png',
            );
            output.parent.createSync(recursive: true);
            output.writeAsBytesSync(bytes!.buffer.asUint8List());
            image.dispose();
          });
          await tester.pumpWidget(const SizedBox());
        }
      }
    },
  );
}

http.Response response(dynamic body, [int status = 200]) => http.Response(
  jsonEncode(body),
  status,
  headers: {'content-type': 'application/json; charset=utf-8'},
);
