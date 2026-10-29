import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'mobile_upgrade_test.dart' as f;

void main() {
 TestWidgetsFlutterBinding.ensureInitialized();
 for(final signed in [false,true]) {
  testWidgets('Procurement entrances are hidden with signed=$signed',(t)async {
   final m=await f.upgradeModel(signed:signed);
   await t.pumpWidget(f.upgradeApp(m,const Scaffold(body:Column(children:[ProcurementEntry(),ProcurementEntry(supplier:true),ProcurementEntry(productID:7)]))));
   await t.pumpAndSettle();
   for(final label in ['采购中心','商机中心','立即询价']) {expect(find.text(label),findsNothing);}
   await t.pumpWidget(const SizedBox());
  });
 }
 for(final path in ['/buyer','/buyer/new?product=7','/merchant/opportunities']) {
  testWidgets('Legacy native destination $path is paused without opening a bridge',(t)async {
   var opens=0;final requests=<String>[];
   TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger.setMockMethodCallHandler(workspaceChannel,(call)async{opens++;return null;});
   final m=await f.upgradeModel(signed:true,handler:(r)async{requests.add(r.url.path);return null;});final original=m.token;
   await t.pumpWidget(f.upgradeApp(m,Scaffold(body:Builder(builder:(c)=>TextButton(onPressed:()=>openProcurementWorkspace(c,path:path),child:const Text('legacy'))))));
   await t.tap(find.text('legacy'));await t.pumpAndSettle();
   expect(find.text('功能暂时暂停'),findsOneWidget);expect(opens,0);expect(m.token,original);expect(requests.where((p)=>p.contains('/procurement')),isEmpty);
   await t.tap(find.text('关闭'));await t.pumpAndSettle();await t.pumpWidget(const SizedBox());
   TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger.setMockMethodCallHandler(workspaceChannel,null);
  });
 }
 test('bridge paths retain their allowlist',(){
  for(final p in ['/buyer','/procurement','/merchant/opportunities/projects/1']) {expect(validWorkspacePath(p),isTrue);}
  for(final p in ['https://bad.invalid/buyer','//bad.invalid/buyer','/admin','/buyer/%2e%2e/admin']) {expect(validWorkspacePath(p),isFalse);}
 });
  testWidgets(
    'SMS sign-in uses a login code and never registers a manufacturer',
    (t) async {
      final requests = <String>[];
      final m = await f.upgradeModel(
        handler: (r) async {
          requests.add(r.url.path);
          if (r.url.path.endsWith('/public/config')) {
            return f.jsonResponse({
              'capabilities': {
                'phone_sms_auth': true,
                'phone_password_auth': true,
              },
            });
          }
          if (r.url.path.endsWith('/auth/phone/code')) {
            expect(jsonDecode(r.body)['purpose'], 'login');
            return f.jsonResponse({
              'development_code': '246810',
              'retry_after': 60,
            });
          }
          if (r.url.path.endsWith('/auth/phone/login')) {
            expect(jsonDecode(r.body).containsKey('password'), isFalse);
            return f.jsonResponse({
              'token': 'verified-session',
              'account': {'id': 7, 'role': 'visitor', 'phone_verified': true},
            });
          }
          return null;
        },
      );
      await t.pumpWidget(f.upgradeApp(m, const LoginScreen(visitor: true)));
      await t.pumpAndSettle();
      await f.tapReachable(t, find.text('短信验证码登录'));
      await t.enterText(
        find.byKey(const ValueKey('auth-phone')),
        '13900001111',
      );
      await f.tapReachable(t, find.text('发送验证码'));
      await t.enterText(find.byKey(const ValueKey('auth-code')), '246810');
      await f.tapReachable(t, find.widgetWithText(FilledButton, '登录'));
      expect(m.token, 'verified-session');
      expect(
        requests.any(
          (p) =>
              p.endsWith('/auth/phone/register') ||
              p.endsWith('/auth/phone') ||
              p.endsWith('/auth/sms'),
        ),
        isFalse,
      );
      await t.pumpWidget(const SizedBox());
    },
  );
}
