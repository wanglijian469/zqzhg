import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'mobile_upgrade_test.dart' as f;

Future<void> fill(WidgetTester t, {bool registration = false}) async {
  await t.enterText(find.byKey(const ValueKey('auth-phone')), '13900001111');
  if (registration) {
    await t.enterText(find.byKey(const ValueKey('auth-code')), '246810');
  }
  await t.enterText(
    find.byKey(const ValueKey('auth-password')),
    'Phone-Password-2026!',
  );
  if (registration) {
    await t.enterText(
      find.byKey(const ValueKey('auth-confirmation')),
      'Phone-Password-2026!',
    );
    await t.ensureVisible(find.byType(CheckboxListTile));
    await t.tap(find.byType(CheckboxListTile));
    await t.pumpAndSettle();
  }
}

Json identity(String role) => {
  'token': 'new-session',
  'account': {
    'id': 7,
    'role': role,
    'phone': '13900001111',
    'phone_verified': true,
    'requires_phone_binding': false,
  },
};
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  for (final visitor in [false, true]) {
    testWidgets(
      'Phone registration sends verified fields for visitor=$visitor',
      (t) async {
        Json? payload;
        int sends = 0;
        final m = await f.upgradeModel(
          handler: (r) async {
            if (r.url.path.endsWith('/auth/phone/code')) {
              sends++;
              expect(jsonDecode(r.body)['purpose'], 'register');
              return f.jsonResponse({
                'sent': true,
                'development_code': '246810',
              });
            }
            if (r.url.path.endsWith('/auth/phone/register')) {
              payload = asMap(jsonDecode(r.body));
              return f.jsonResponse(
                identity(visitor ? 'visitor' : 'merchant'),
                201,
              );
            }
            return null;
          },
        );
        await t.pumpWidget(
          f.upgradeApp(
            m,
            LoginScreen(
              companyAccount: false,
              visitor: visitor,
              register: true,
            ),
          ),
        );
        await t.pumpAndSettle();
        expect(find.byType(ChoiceChip), findsNothing);
        await fill(t, registration: true);
        await f.tapReachable(t, find.text('发送验证码'));
        expect(sends, 1);
        final button = t.widget<OutlinedButton>(
          find.widgetWithText(OutlinedButton, '60s'),
        );
        expect(button.onPressed, isNull);
        await f.tapReachable(t, find.widgetWithText(FilledButton, '注册并继续'));
        expect(payload!['role'], 'visitor');
        expect(payload!['phone'], '13900001111');
        expect(payload!['code'], '246810');
        expect(payload!.containsKey('confirmation'), isFalse);
        expect(
          m.prefs.getKeys().any(
            (k) => '${m.prefs.get(k)}'.contains('Phone-Password'),
          ),
          isFalse,
        );
        await t.pumpWidget(const SizedBox());
      },
    );
  }
  testWidgets(
    'Visitor must explicitly confirm upgrade; account ID stays the same',
    (t) async {
      int upgrades = 0;
      final m = await f.upgradeModel(
        handler: (r) async {
          if (r.url.path.endsWith('/auth/phone/password')) {
            return f.jsonResponse(identity('visitor'));
          }
          if (r.url.path.endsWith('/auth/me/merchant-upgrade')) {
            upgrades++;
            expect(jsonDecode(r.body)['confirmed'], true);
            return f.jsonResponse(identity('merchant'));
          }
          return null;
        },
      );
      await t.pumpWidget(
        f.upgradeApp(m, const LoginScreen(companyAccount: false)),
      );
      await t.pumpAndSettle();
      await fill(t);
      await f.tapReachable(t, find.widgetWithText(FilledButton, '登录'));
      expect(upgrades, 0);
      expect(find.byType(AlertDialog), findsOneWidget);
      await f.tapReachable(t, find.text('取消'));
      expect(m.account['role'], 'visitor');
      expect(upgrades, 0);
      await fill(t);
      await f.tapReachable(t, find.widgetWithText(FilledButton, '登录'));
      await f.tapReachable(t, find.text('确认升级'));
      expect(upgrades, 1);
      expect(m.account['id'], 7);
      expect(m.account['role'], 'merchant');
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets(
    'Manufacturer at visitor entry stays in favorite flow and does not upgrade',
    (t) async {
      final calls = <String>[];
      final m = await f.upgradeModel(
        handler: (r) async {
          calls.add(r.url.path);
          if (r.url.path.endsWith('/auth/phone/password')) {
            return f.jsonResponse(identity('merchant'));
          }
          return null;
        },
      );
      await t.pumpWidget(
        f.upgradeApp(
          m,
          const LoginScreen(companyAccount: false, visitor: true),
        ),
      );
      await t.pumpAndSettle();
      await fill(t);
      await f.tapReachable(t, find.widgetWithText(FilledButton, '登录'));
      expect(m.account['role'], 'merchant');
      expect(find.byType(MerchantWorkspace), findsNothing);
      expect(calls.any((p) => p.contains('merchant-upgrade')), isFalse);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets('Old account binding rotates session and sends no password', (
    t,
  ) async {
    Json? payload;
    final m = await f.upgradeModel(
      signed: true,
      handler: (r) async {
        if (r.url.path.endsWith('/auth/me/phone')) {
          expect(r.method, 'PUT');
          payload = asMap(jsonDecode(r.body));
          return f.jsonResponse(identity('merchant'));
        }
        return null;
      },
    );
    await t.pumpWidget(
      f.upgradeApp(m, const LoginScreen(companyAccount: false, binding: true)),
    );
    await t.pumpAndSettle();
    expect(find.byKey(const ValueKey('auth-password')), findsNothing);
    await t.enterText(find.byKey(const ValueKey('auth-phone')), '13900001111');
    await t.enterText(find.byKey(const ValueKey('auth-code')), '246810');
    await f.tapReachable(t, find.widgetWithText(FilledButton, '验证并绑定'));
    expect(payload!.keys.toSet(), {'phone', 'code'});
    expect(m.token, 'new-session');
    await t.pumpWidget(const SizedBox());
  });
  testWidgets(
    'Unsupported backend is explicit and no authentication write is sent',
    (t) async {
      int writes = 0;
      final m = await f.upgradeModel(
        handler: (r) async {
          if (r.url.path.endsWith('/public/config')) {
            return f.jsonResponse({'capabilities': {}});
          }
          if (r.method == 'POST') writes++;
          return null;
        },
      );
      await t.pumpWidget(
        f.upgradeApp(m, const LoginScreen(companyAccount: false)),
      );
      await t.pumpAndSettle();
      await fill(t);
      await f.tapReachable(t, find.widgetWithText(FilledButton, '登录'));
      expect(writes, 0);
      expect(find.textContaining('尚未支持手机号'), findsOneWidget);
      await t.pumpWidget(const SizedBox());
    },
  );
  testWidgets('Bad phone password does not expire an existing session', (
    t,
  ) async {
    final m = await f.upgradeModel(
      signed: true,
      handler: (r) async {
        if (r.url.path.endsWith('/auth/phone/password')) {
          return f.jsonResponse({
            'error': {'code': 'invalid_credentials'},
          }, 401);
        }
        return null;
      },
    );
    await expectLater(
      m.api('/auth/phone/password', method: 'POST', data: {}),
      throwsA(isA<ApiFailure>()),
    );
    expect(m.token, 'isolated-fixture');
  });
  testWidgets('Home and vendor list have no map entry or map requests', (
    t,
  ) async {
    final calls = <String>[];
    final m = await f.upgradeModel(
      handler: (r) async {
        calls.add(r.url.path);
        if (r.url.path.endsWith('/public/home')) {
          return f.jsonResponse({'vendors': [], 'products': [], 'hero': {}});
        }
        return null;
      },
    );
    await t.pumpWidget(f.upgradeApp(m, const AppShell()));
    await t.pumpAndSettle();
    expect(find.text('厂商地图'), findsNothing);
    await f.tapReachable(t, find.byKey(const ValueKey('app-tab-1')));
    expect(find.byIcon(Icons.map_outlined), findsNothing);
    expect(
      calls.where(
        (p) =>
            p.contains('industry-map') ||
            p.contains('/maps/') ||
            p.contains('map-preview'),
      ),
      isEmpty,
    );
    await t.pumpWidget(const SizedBox());
  });
  testWidgets('Production image challenge must be confirmed before the SMS request', (t) async {
    Json? sent;
    final m = await f.upgradeModel(handler: (r) async {
      if (r.url.path.endsWith('/public/config')) return f.jsonResponse({'capabilities': {'phone_password_auth':true,'phone_sms_register':true,'phone_sms_auth':true,'sms_challenge':true}});
      if (r.url.path.endsWith('/auth/phone/challenge')) return f.jsonResponse({'challenge_id':'image-test','image':''});
      if (r.url.path.endsWith('/auth/phone/code')) { sent=asMap(jsonDecode(r.body));return f.jsonResponse({'sent':true,'retry_after':60}); }
      return null;
    });
    await t.pumpWidget(f.upgradeApp(m,const LoginScreen(visitor:true,companyAccount:false,register:true)));
    await t.pumpAndSettle();
    await t.enterText(find.byKey(const ValueKey('auth-phone')),'13900008888');
    await f.tapReachable(t,find.text('发送验证码'));
    expect(find.byType(AlertDialog),findsOneWidget);expect(sent,isNull);
    await t.enterText(find.byKey(const ValueKey('sms-image-answer-image-test')),'54321');
    await f.tapReachable(t,find.text('验证并发送'));
    expect(sent!['purpose'],'register');expect(sent!['challenge_id'],'image-test');expect(sent!['challenge_answer'],'54321');expect((sent!['request_id'] as String).length,64);
    expect(t.takeException(),isNull);
    await t.pumpWidget(const SizedBox());
  });
}