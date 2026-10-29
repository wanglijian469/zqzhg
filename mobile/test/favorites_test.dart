import 'dart:async';
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
import 'app_navigation_test.dart' as nav;

http.Response response(Json value, [int status = 200]) => http.Response(
  jsonEncode(value),
  status,
  headers: {'content-type': 'application/json; charset=utf-8'},
);
Json user(String token, {int id = 7, String role = 'visitor'}) => {
  'token': token,
  'account': {
    'id': id,
    'role': role,
    'login_name': role == 'visitor' ? '普通访客' : '厂商账号',
  },
};

class Cloud {
  final saved = <String, Map<String, Json>>{};
  final ledger = <String, Set<String>>{};
  final syncTokens = <String>[];
  bool offline = false,
      loseAcknowledgment = false,
      unavailable = false,
      oldServer = false;
  Completer<void>? gate, started;
  int tick = 0;
  Map<String, Json> account(String token) =>
      saved.putIfAbsent(token.split(':').first, () => {});
  Json resolve(String kind, int id) => {
    'kind': kind,
    'content_id': id,
    'available': !unavailable,
    if (!unavailable) 'item': kind == 'vendor' ? nav.company() : nav.product(),
  };
  Future<http.Response?> handle(http.Request r) async {
    final path = r.url.path;
    final token = (r.headers['authorization'] ?? '').replaceFirst(
      'Bearer ',
      '',
    );
    if (path.endsWith('/public/config')) {
      return response({
        'capabilities': {'phone_password_auth': true},
      });
    }
    if (path.endsWith('/auth/phone/register')) {
      return response(user('a:first'), 201);
    }
    if (path.endsWith('/auth/phone/password')) {
      return response(user('manufacturer:first', id: 9, role: 'merchant'));
    }
    if (path.endsWith('/auth/visitor/register') ||
        path.endsWith('/auth/visitor')) {
      return response(user('a:first'), 201);
    }
    if (path.endsWith('/auth/merchant')) {
      return response(user('manufacturer:first', id: 9, role: 'merchant'));
    }
    if (!path.contains('/favorites')) return null;
    if (oldServer) {
      return response({
        'error': {'code': 'not_found'},
      }, 404);
    }
    if (offline) throw const SocketException('fixture offline');
    if (path.endsWith('/favorites/sync')) {
      syncTokens.add(token);
      started?.complete();
      started = null;
      if (gate != null) await gate!.future;
      final operations = rows(jsonDecode(r.body)['operations']);
      final seen = ledger.putIfAbsent(token.split(':').first, () => {});
      for (final op in operations) {
        if (!seen.add(op['operation_id'])) continue;
        final key = favoriteIdentity(op['kind'], op['content_id']);
        if (op['favorited'] == true) {
          account(token).putIfAbsent(
            key,
            () => {
              'kind': op['kind'],
              'content_id': op['content_id'],
              'collected_at': DateTime.utc(
                2026,
                10,
                15,
              ).add(Duration(milliseconds: tick++)).toIso8601String(),
            },
          );
        } else {
          account(token).remove(key);
        }
      }
      if (loseAcknowledgment) {
        loseAcknowledgment = false;
        throw const SocketException('lost response after commit');
      }
      return response({
        'acknowledged': [for (final op in operations) op['operation_id']],
      });
    }
    if (path.endsWith('/favorites/resolve')) {
      return response({
        'items': [
          for (final item in rows(jsonDecode(r.body)['items']))
            resolve(item['kind'], item['content_id']),
        ],
      });
    }
    final entries = account(token).values.toList();
    return response({
      'items': [
        for (final item in entries)
          {...item, ...resolve(item['kind'], item['content_id'])},
      ],
      'total': entries.length,
      'page': 1,
      'page_size': 100,
    });
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test(
    'Visitor auth on an older backend reports an upgrade and retains local favorites',
    () async {
      final m = await nav.fixture(
        signed: false,
        intercept: (r) async {
          if (r.url.path.contains('/auth/visitor')) {
            return response({
              'error': {'code': 'not_found'},
            }, 404);
          }
          return null;
        },
      );
      await m.favorites.toggle(nav.company());
      await expectLater(
        m.api(
          '/auth/visitor',
          method: 'POST',
          data: {'username': 'test', 'password': 'test'},
        ),
        throwsA(
          predicate(
            (e) => e is ApiFailure && e.code == 'favoritesUpgradeRequired',
          ),
        ),
      );
      expect(m.favorites.contains('vendor', 1), isTrue);
    },
  );
  void size(WidgetTester tester, double width) {
    tester.view.physicalSize = Size(width, 900);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
  }

  test(
    'Guest collection persists across restart, language and endpoint changes',
    () async {
      final cloud = Cloud();
      final m = await nav.fixture(signed: false, intercept: cloud.handle);
      await m.favorites.toggle(nav.company());
      await m.favorites.toggle(nav.product());
      await Future.wait([
        m.favorites.toggle(nav.product()),
        m.favorites.toggle(nav.product()),
      ]);
      final restarted = AppModel(
        m.prefs,
        m.resources,
        testBaseUrl: 'http://fixture',
        client: MockClient((r) async => await cloud.handle(r) ?? response({})),
      );
      expect(restarted.favorites.entries, hasLength(2));
      expect(restarted.favorites.contains('vendor', 1), isTrue);
      await restarted.toggleLanguage();
      expect(restarted.favorites.entries, hasLength(2));
      final other = AppModel(
        m.prefs,
        m.resources,
        testBaseUrl: 'http://other-fixture',
      );
      expect(other.favorites.entries, isEmpty);
      final restored = AppModel(
        m.prefs,
        m.resources,
        testBaseUrl: 'http://fixture',
      );
      expect(restored.favorites.entries, hasLength(2));
      expect(cloud.syncTokens, isEmpty);
    },
  );

  test(
    'Guest merge is acknowledged before cleanup and syncs across two devices',
    () async {
      final cloud = Cloud();
      final m = await nav.fixture(signed: false, intercept: cloud.handle);
      await m.favorites.toggle(nav.company());
      await m.favorites.toggle(nav.product());
      await m.signIn(user('a:first'));
      expect(m.favorites.pendingCount, 2);
      final guestKey = m.favorites.keyFor(m.baseUrl, 'guest');
      expect(
        asMap(jsonDecode(m.prefs.getString(guestKey)!))['entries'],
        hasLength(2),
      );
      await m.favorites.refresh();
      expect(m.favorites.pendingCount, 0);
      expect(cloud.account('a:first'), hasLength(2));
      expect(
        asMap(jsonDecode(m.prefs.getString(guestKey)!))['entries'],
        isEmpty,
      );
      final second = await nav.fixture(signed: false, intercept: cloud.handle);
      await second.signIn(user('a:second'));
      await second.favorites.refresh();
      expect(second.favorites.entries, hasLength(2));
      await second.favorites.toggle(nav.product());
      await second.favorites.refresh();
      await m.favorites.refresh();
      expect(m.favorites.contains('product', 8), isFalse);
      await m.signOut();
      expect(m.favorites.entries, isEmpty);
      await m.signIn(user('b:first', id: 8));
      await m.favorites.refresh();
      expect(m.favorites.entries, isEmpty);
      expect(cloud.account('b:first'), isEmpty);
    },
  );

  test(
    'Offline operations survive restart and a lost acknowledgment cannot resurrect a canceled item',
    () async {
      final cloud = Cloud();
      final m = await nav.fixture(signed: false, intercept: cloud.handle);
      await m.signIn(user('a:first'));
      await m.favorites.refresh();
      await m.favorites.toggle(nav.company());
      await m.favorites.toggle(nav.product());
      await m.favorites.toggle(nav.product());
      cloud.offline = true;
      await m.favorites.refresh();
      expect(m.favorites.pendingCount, 3);
      expect(m.favorites.error, 'network_error');
      final restarted = AppModel(
        m.prefs,
        m.resources,
        testBaseUrl: 'http://fixture',
        client: MockClient((r) async => await cloud.handle(r) ?? response({})),
      );
      expect(restarted.favorites.pendingCount, 3);
      cloud.offline = false;
      await restarted.favorites.refresh();
      expect(restarted.favorites.pendingCount, 0);
      expect(cloud.account('a:first').keys, ['vendor:1']);
      await restarted.favorites.toggle(nav.product());
      cloud.loseAcknowledgment = true;
      await restarted.favorites.refresh();
      expect(restarted.favorites.pendingCount, 1);
      cloud
          .account('a:second')
          .remove(
            'product:8',
          ); // A second device cancels after the first request committed.
      await restarted.favorites.refresh();
      expect(restarted.favorites.pendingCount, 0);
      expect(restarted.favorites.contains('product', 8), isFalse);
    },
  );

  test(
    'Switching accounts during an in-flight request never sends the first account queue with a new token',
    () async {
      final cloud = Cloud();
      final m = await nav.fixture(signed: false, intercept: cloud.handle);
      await m.signIn(user('a:first'));
      await m.favorites.toggle(nav.company());
      cloud.gate = Completer<void>();
      cloud.started = Completer<void>();
      final entered = cloud.started!.future;
      final running = m.favorites.refresh();
      await entered;
      await m.signIn(user('b:first', id: 8));
      expect(m.favorites.entries, isEmpty);
      cloud.gate!.complete();
      await running;
      cloud.gate = null;
      await m.favorites.refresh();
      expect(cloud.syncTokens, ['a:first']);
      expect(cloud.account('b:first'), isEmpty);
      await m.signIn(user('a:first'));
      await m.favorites.refresh();
      expect(m.favorites.pendingCount, 0);
      expect(m.favorites.contains('vendor', 1), isTrue);
    },
  );

  test(
    'A replacement session waits for the old sync and then retries with its own token',
    () async {
      final cloud = Cloud();
      final m = await nav.fixture(signed: false, intercept: cloud.handle);
      await m.signIn(user('a:first'));
      await m.favorites.toggle(nav.company());
      cloud.gate = Completer<void>();
      cloud.started = Completer<void>();
      final entered = cloud.started!.future;
      final first = m.favorites.refresh();
      await entered;
      await m.signIn(user('a:second'));
      final replacement = m.favorites.refresh();
      cloud.gate!.complete();
      await first;
      await replacement;
      expect(cloud.syncTokens, ['a:first', 'a:second']);
      expect(m.favorites.pendingCount, 0);
      expect(m.favorites.contains('vendor', 1), isTrue);
    },
  );

  test(
    'Resolution distinguishes network failure, unavailable content and an older server',
    () async {
      final cloud = Cloud();
      final m = await nav.fixture(signed: false, intercept: cloud.handle);
      await m.favorites.toggle(nav.company());
      cloud.offline = true;
      await m.favorites.refresh();
      expect(m.favorites.entries.single['available'], isTrue);
      expect(m.favorites.error, 'network_error');
      cloud.offline = false;
      cloud.unavailable = true;
      await m.favorites.refresh();
      expect(m.favorites.entries.single['available'], isFalse);
      cloud.oldServer = true;
      await m.favorites.refresh();
      expect(m.favorites.error, 'favoritesUpgradeRequired');
      expect(m.favorites.entries, hasLength(1));
      await m.favorites.remove(m.favorites.entries.single);
      expect(m.favorites.entries, isEmpty);
    },
  );

  test(
    'A failed account login does not expire the existing visitor session',
    () async {
      final m = await nav.fixture(
        signed: false,
        intercept: (r) async {
          if (r.url.path.endsWith('/auth/merchant')) {
            return response({
              'error': {'code': 'invalid_credentials'},
            }, 401);
          }
          return null;
        },
      );
      await m.signIn(user('a:first'));
      await expectLater(
        m.api(
          '/auth/merchant',
          method: 'POST',
          data: {'username': 'wrong', 'password': 'wrong'},
        ),
        throwsA(isA<ApiFailure>()),
      );
      expect(m.token, 'a:first');
      expect(m.account['role'], 'visitor');
    },
  );

  testWidgets('Returning from a main tab goes home before exiting', (
    tester,
  ) async {
    size(tester, 390);
    final cloud = Cloud();
    final m = await nav.fixture(signed: false, intercept: cloud.handle);
    await tester.pumpWidget(nav.harness(m));
    await tester.pumpAndSettle();
    await nav.tap(tester, find.byKey(const ValueKey('app-tab-2')));
    await tester.binding.handlePopRoute();
    await tester.pumpAndSettle();
    expect(find.byType(IndustrialHomeScreen), findsOneWidget);
    expect(find.byType(BrowseScreen), findsNothing);
    await tester.pumpWidget(const SizedBox());
  });

  testWidgets(
    'Card and detail buttons share state without opening the card; favorites is available from My',
    (tester) async {
      size(tester, 390);
      final cloud = Cloud();
      final m = await nav.fixture(signed: false, intercept: cloud.handle);
      await tester.pumpWidget(nav.harness(m));
      await tester.pumpAndSettle();
      expect(find.byType(IndustrialHomeScreen), findsOneWidget);
      await nav.tap(tester, find.byKey(const ValueKey('app-tab-1')));
      await nav.tap(tester, find.byKey(const ValueKey('favorite-vendor-1')));
      expect(find.byType(CompanyShowroomScreen), findsNothing);
      expect(m.favorites.contains('vendor', 1), isTrue);
      await nav.tap(tester, find.byKey(const ValueKey('app-tab-3')));
      await nav.tap(tester, find.byKey(const ValueKey('my-favorites')));
      expect(find.byType(FavoritesScreen), findsOneWidget);
      expect(find.text('河北冀农农机具有限公司'), findsOneWidget);
      await nav.tap(tester, find.byKey(const ValueKey('favorite-vendor-1')));
      expect(find.text(m.t('emptyFavoriteVendors')), findsOneWidget);
      await tester.tap(find.byType(BackButton).first);
      await tester.pumpAndSettle();
      await nav.tap(tester, find.byKey(const ValueKey('app-tab-2')));
      await nav.tap(tester, find.byKey(const ValueKey('favorite-product-8')));
      expect(find.byType(DetailScreen), findsNothing);
      await nav.tap(tester, find.text('测试旋耕机 440'));
      expect(find.byType(DetailScreen), findsOneWidget);
      expect(find.text('已收藏'), findsOneWidget);
      await nav.tap(tester, find.byKey(const ValueKey('favorite-product-8')));
      await tester.tap(find.byType(BackButton));
      await tester.pumpAndSettle();
      expect(m.favorites.contains('product', 8), isFalse);
      await nav.tap(tester, find.byTooltip('返回首页'));
      expect(find.byType(IndustrialHomeScreen), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );

  testWidgets(
    'Unified registration merges favorites and ordinary users keep optional company entry',
    (tester) async {
      size(tester, 390);
      final cloud = Cloud();
      final m = await nav.fixture(signed: false, intercept: cloud.handle);
      await m.favorites.toggle(nav.company());
      await tester.pumpWidget(nav.harness(m));
      await tester.pumpAndSettle();
      await nav.tap(tester, find.byKey(const ValueKey('app-tab-3')));
      await nav.tap(tester, find.byKey(const ValueKey('account-summary')));
      await nav.tap(tester, find.text('免费注册'));
      await nav.tap(tester, find.text('账号密码注册'));
      await tester.enterText(
        find.byKey(const ValueKey('auth-username')),
        'ordinary-user',
      );
      await tester.enterText(
        find.byKey(const ValueKey('auth-accountPassword')),
        'Visitor-pass-2026!',
      );
      await tester.enterText(
        find.byKey(const ValueKey('auth-accountConfirm')),
        'Visitor-pass-2026!',
      );
      await nav.tap(tester, find.byType(CheckboxListTile));
      await nav.tap(tester, find.widgetWithText(FilledButton, '注册并进入展馆'));
      expect(m.account['role'], 'visitor');
      await m.favorites.refresh();
      expect(cloud.account('a:first'), hasLength(1));
      await nav.tap(tester, find.byKey(const ValueKey('app-tab-3')));
      expect(find.byKey(const ValueKey('my-booth')), findsOneWidget);
      expect(find.text('厂商账号登录'), findsNothing);
      expect(find.byType(MerchantWorkspace), findsNothing);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );

  for (final width in [360.0, 390.0, 768.0]) {
    for (final lang in ['zh', 'en']) {
      testWidgets(
        'Favorites $width $lang supports large text and independent manufacturer/product tabs',
        (tester) async {
          size(tester, width);
          final cloud = Cloud();
          final m = await nav.fixture(signed: false, intercept: cloud.handle);
          m.language = lang;
          await m.favorites.toggle(nav.company());
          await m.favorites.toggle(nav.product());
          await tester.pumpWidget(nav.harness(m, scale: 1.4));
          await tester.pumpAndSettle();
          await nav.tap(tester, find.byKey(const ValueKey('app-tab-3')));
          await nav.tap(tester, find.byKey(const ValueKey('my-favorites')));
          await nav.tap(
            tester,
            find.byKey(const ValueKey('favorites-kind-product')),
          );
          expect(find.text('测试旋耕机 440'), findsOneWidget);
          expect(tester.takeException(), isNull);
          await tester.pumpWidget(const SizedBox());
        },
      );
    }
  }

  for (final lang in ['zh', 'en']) {
    testWidgets('Visitor login at 360px $lang with large text', (tester) async {
      size(tester, 360);
      final m = await nav.fixture(signed: false);
      m.language = lang;
      await tester.pumpWidget(
        nav.harness(m, home: const LoginScreen(visitor: true), scale: 1.4),
      );
      await tester.pumpAndSettle();
      await nav.tap(tester, find.text(lang == 'zh' ? '免费注册' : 'Register free'));
      expect(find.byKey(const ValueKey('auth-phone')), findsOneWidget);
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    });
  }

  testWidgets('Capture favorites UI evidence', (tester) async {
    size(tester, 390);
    await tester.runAsync(() async {
      final font = FontLoader('NavigationQA');
      font.addFont(
        Future.value(
          ByteData.sublistView(
            File('C:/Windows/Fonts/msyh.ttc').readAsBytesSync(),
          ),
        ),
      );
      await font.load();
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
    });
    final cloud = Cloud();
    final m = await nav.fixture(signed: false, intercept: cloud.handle);
    await m.favorites.toggle(nav.company());
    await m.favorites.toggle(nav.product());
    final boundary = GlobalKey();
    await tester.pumpWidget(nav.harness(m, boundary: boundary));
    await tester.pumpAndSettle();
    Future<void> capture(String name) async {
      expect(tester.takeException(), isNull);
      final render =
          boundary.currentContext!.findRenderObject() as RenderRepaintBoundary;
      await tester.runAsync(() async {
        final image = await render.toImage(pixelRatio: 1.5),
            bytes = await image.toByteData(format: ui.ImageByteFormat.png);
        File(
          '../docs/screenshots/favorites-$name.png',
        ).writeAsBytesSync(bytes!.buffer.asUint8List());
        image.dispose();
      });
    }

    await capture('home');
    await nav.tap(tester, find.byKey(const ValueKey('app-tab-1')));
    await capture('manufacturer');
    await nav.tap(tester, find.byKey(const ValueKey('app-tab-3')));
    await nav.tap(tester, find.byKey(const ValueKey('my-favorites')));
    await capture('guest');
    await nav.tap(tester, find.byKey(const ValueKey('favorites-kind-product')));
    await capture('products');
    await nav.tap(tester, find.text('登录／注册并同步'));
    await nav.tap(tester, find.text('注册访客账号'));
    await capture('visitor-register');
    await tester.tap(find.byType(BackButton));
    await tester.pumpAndSettle();
    await m.signIn(user('a:first'));
    await tester.pumpAndSettle();
    await capture('account');
    cloud.unavailable = true;
    await m.favorites.refresh();
    await tester.pumpAndSettle();
    await capture('unavailable');
    await m.toggleLanguage();
    tester.view.physicalSize = const Size(360, 900);
    await tester.pumpWidget(nav.harness(m, scale: 1.4, boundary: boundary));
    await tester.pumpAndSettle();
    await capture('360-en-large');
    await tester.pumpWidget(const SizedBox());
  }, skip: Platform.environment['RUN_FAVORITES_SCREENSHOTS'] != '1');
}
