import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'mobile_upgrade_test.dart' as f;

Future<void> enter(WidgetTester t, String key, String value) async {
  final item = find.byKey(ValueKey('auth-$key'));
  await t.ensureVisible(item);
  await t.enterText(item, value);
}

Future<void> click(WidgetTester t, String text) async {
  final item = find.text(text).last;
  await t.ensureVisible(item);
  await t.tap(item);
  await t.pumpAndSettle();
}

void main() {
  test('new password policy boundaries and visible characters agree', () {
    for (final v in ['12345678', 'Ab!23456', 'a' * 20]) {
      expect(validNewPassword(v), true);
    }
    for (final v in [
      '1234567',
      'a' * 21,
      'abc defg',
      'abcd1234\n',
      '中文abcd1234',
    ]) {
      expect(validNewPassword(v), false);
    }
  });
  testWidgets(
    'phone registration verifies first and creates an ordinary account only at completion',
    (t) async {
      final writes = <String>[];
      Json? registration;
      final m = await f.upgradeModel(
        handler: (r) async {
          if (r.method == 'POST') writes.add(r.url.path);
          if (r.url.path.endsWith('/auth/phone/verify')) {
            return f.jsonResponse({'action_token': 'proof', 'expires_in': 300});
          }
          if (r.url.path.endsWith('/auth/phone/register')) {
            registration = asMap(jsonDecode(r.body));
            return f.jsonResponse({
              'token': 'new-token',
              'account': {'id': 77, 'role': 'visitor', 'welcome_pending': true},
            }, 201);
          }
          return null;
        },
      );
      await t.pumpWidget(f.upgradeApp(m, const LoginScreen(register: true)));
      await t.pumpAndSettle();
      await enter(t, 'phone', '13900008811');
      await enter(t, 'code', '123456');
      await click(t, '下一步');
      expect(writes.where((s) => s.endsWith('/register')), isEmpty);
      await enter(t, 'phonePassword', '12345678');
      await enter(t, 'phoneConfirm', '12345678');
      await t.ensureVisible(find.byType(CheckboxListTile));
      await t.tap(find.byType(CheckboxListTile));
      await t.pumpAndSettle();
      await click(t, '注册并进入展馆');
      expect(registration?['action_token'], 'proof');
      expect(m.account['role'], 'visitor');
      expect(m.token, 'new-token');
      expect(t.takeException(), isNull);
      await t.pumpWidget(const SizedBox());
      m.dispose();
    },
  );
  testWidgets(
    'registration tabs keep values and default login has no SMS login choice',
    (t) async {
      final m = await f.upgradeModel();
      await t.pumpWidget(f.upgradeApp(m, const LoginScreen(register: true)));
      await t.pumpAndSettle();
      await enter(t, 'phone', '13900008812');
      await click(t, '账号密码注册');
      await enter(t, 'username', 'mycompany');
      await enter(t, 'accountPassword', 'abcdefgh');
      await click(t, '手机验证码注册');
      expect(
        t
            .widget<TextField>(find.byKey(const ValueKey('auth-phone')))
            .controller!
            .text,
        '13900008812',
      );
      await click(t, '账号密码注册');
      expect(
        t
            .widget<TextField>(
              find.byKey(const ValueKey('auth-accountPassword')),
            )
            .controller!
            .text,
        'abcdefgh',
      );
      await click(t, '已有账号，登录');
      expect(find.text('短信验证码登录'), findsNothing);
      expect(find.text('手机号或用户名'), findsOneWidget);
      await t.pumpWidget(const SizedBox());
      m.dispose();
    },
  );
  testWidgets(
    'historical password uses a single unified login call unchanged',
    (t) async {
      Json? payload;
      final m = await f.upgradeModel(
        handler: (r) async {
          if (r.url.path.endsWith('/auth/password')) {
            payload = asMap(jsonDecode(r.body));
            return f.jsonResponse({
              'token': 'old-role',
              'account': {'id': 7, 'role': 'merchant'},
            });
          }
          return null;
        },
      );
      await t.pumpWidget(
        f.upgradeApp(
          m,
          Builder(
            builder: (c) => Scaffold(
              body: TextButton(
                onPressed: () => Navigator.push(
                  c,
                  MaterialPageRoute<void>(builder: (_) => const LoginScreen()),
                ),
                child: const Text('open'),
              ),
            ),
          ),
        ),
      );
      await click(t, 'open');
      await enter(t, 'username', 'old-company');
      await enter(t, 'loginPassword', '  Legacy-Long-Password-2026!  ');
      await click(t, '登录');
      expect(payload?['password'], '  Legacy-Long-Password-2026!  ');
      expect(m.account['role'], 'merchant');
      await t.pumpWidget(const SizedBox());
      m.dispose();
    },
  );
  testWidgets(
    'forgotten password uses reset proof and returns to password sign-in',
    (t) async {
      Json? payload;
      final m = await f.upgradeModel(
        handler: (r) async {
          if (r.url.path.endsWith('/auth/phone/verify')) {
            expect(jsonDecode(r.body)['purpose'], 'reset_password');
            return f.jsonResponse({'action_token': 'reset-proof'});
          }
          if (r.url.path.endsWith('/auth/phone/password/reset')) {
            payload = asMap(jsonDecode(r.body));
            return f.jsonResponse({'reset': true});
          }
          return null;
        },
      );
      await t.pumpWidget(f.upgradeApp(m, const LoginScreen()));
      await t.pumpAndSettle();
      await click(t, '忘记密码？');
      await enter(t, 'phone', '13900008813');
      await enter(t, 'code', '123456');
      await click(t, '下一步');
      await enter(t, 'resetPassword', 'Reset!12');
      await enter(t, 'resetConfirm', 'Reset!12');
      await click(t, '确认重置密码');
      expect(payload?['action_token'], 'reset-proof');
      expect(m.token, isEmpty);
      expect(find.text('密码已重置，请使用新密码登录'), findsOneWidget);
      await t.pumpWidget(const SizedBox());
      m.dispose();
    },
  );

  testWidgets(
    'welcome is acknowledged once and stale cross-device pending state is refreshed',
    (t) async {
      bool closed = false;
      int acknowledgements = 0;
      final m = await f.upgradeModel(
        signed: true,
        handler: (r) async {
          if (r.url.path.endsWith('/auth/me')) {
            return f.jsonResponse({
              'id': 7,
              'role': 'visitor',
              'welcome_pending': !closed,
            });
          }
          if (r.url.path.endsWith('/auth/me/welcome')) {
            acknowledgements++;
            closed = true;
            return f.jsonResponse({'welcome_pending': false});
          }
          return null;
        },
      );
      m.account = {'id': 7, 'role': 'visitor', 'welcome_pending': true};
      await t.pumpWidget(f.upgradeApp(m, const AppShell()));
      await t.pumpAndSettle();
      expect(find.text('欢迎来到中企展馆'), findsOneWidget);
      await click(t, '跳过');
      expect(acknowledgements, 1);
      expect(m.account['welcome_pending'], false);
      m.account['welcome_pending'] = true;
      m.notifyListeners();
      await t.pumpAndSettle();
      expect(find.text('欢迎来到中企展馆'), findsNothing);
      expect(m.account['welcome_pending'], false);
      expect(acknowledgements, 1);
      await t.pumpWidget(const SizedBox());
      m.dispose();
    },
  );
}
