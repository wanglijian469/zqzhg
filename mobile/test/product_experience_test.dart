import 'dart:convert';
import 'dart:io';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'legacy_hall_fixture.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  Future<AppModel> model() async {
    SharedPreferences.setMockInitialValues({'merchant_token': 'test'});
    return AppModel(
      await SharedPreferences.getInstance(),
      {
        for (final lang in ['zh', 'en'])
          lang: asMap(
            jsonDecode(File('assets/i18n/$lang.json').readAsStringSync()),
          ),
      },
      client: legacyHallClient(
        MockClient((r) async {
          if (r.url.path.contains('/showrooms/')) {
            return http.Response(
              jsonEncode({
                'item': {
                  'id': 21,
                  'kind': 'vendor',
                  'data': {'name': '测试厂商', 'media_ids': <int>[]},
                },
                'canonical_url': 'https://example.com/v/sample',
              }),
              200,
              headers: {'content-type': 'application/json; charset=utf-8'},
            );
          }
          if (r.url.path.endsWith('/public/config')) {
            return http.Response('{"capabilities":{"douyin_video":true}}', 200);
          }
          return http.Response('{"items":[],"total":0}', 200);
        }),
      ),
    );
  }

  test('domestic guides and official URLs keep six domestic channels only', () {
    expect(domesticShareKeys.length, 6);
    expect(domesticNativeShareGuide('wechatMoments', true), contains('不会自动发布'));
    expect(
      Uri.parse(
        domesticNativeSharePage('qq', '测试', 'https://example.com/v/a'),
      ).host,
      'connect.qq.com',
    );
    expect(
      domesticNativeSharePage('wechatFriend', '测试', 'https://example.com/v/a'),
      '',
    );
  });
  testWidgets(
    'showroom share copies inside the app without invoking system text/image share',
    (tester) async {
      final methods = <String>[];
      const channel = MethodChannel('com.zqzt.dalunongji/sharing');
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, (call) async {
            methods.add(call.method);
            return true;
          });
      final m = await model();
      await tester.pumpWidget(
        ModelScope(
          model: m,
          child: const MaterialApp(home: ShowroomPromotion(slug: 'sample')),
        ),
      );
      await tester.pumpAndSettle();
      for (final key in domesticShareKeys) {
        expect(find.widgetWithText(OutlinedButton, m.t(key)), findsOneWidget);
      }
      expect(find.text('WhatsApp'), findsNothing);
      expect(find.text('Telegram'), findsNothing);
      await tester.ensureVisible(find.text('复制展厅地址'));
      await tester.tap(find.text('复制展厅地址'));
      await tester.pumpAndSettle();
      expect(methods, isNot(contains('shareText')));
      expect(methods, isNot(contains('shareImage')));
      await tester.pumpWidget(const SizedBox());
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(channel, null);
    },
  );
  testWidgets(
    'product video add screen names the current context and offers no automatic source choice',
    (tester) async {
      final m = await model();
      await tester.pumpWidget(
        ModelScope(
          model: m,
          child: const MaterialApp(home: NativeVideoAddScreen(productID: 23)),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('添加产品视频'), findsOneWidget);
      expect(find.byType(NativeDouyinEditor), findsNothing);
      expect(find.byType(NativeResourceEditor), findsNothing);
      expect(find.text('粘贴视频链接'), findsOneWidget);
      expect(find.text('上传本地视频'), findsOneWidget);
      await tester.pumpWidget(const SizedBox());
    },
  );
  testWidgets('Douyin editor pins the product in product context', (
    tester,
  ) async {
    final m = await model();
    await tester.pumpWidget(
      ModelScope(
        model: m,
        child: const MaterialApp(home: NativeDouyinEditor(productID: 23)),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('关联当前产品：#23（不可更改）'), findsOneWidget);
    expect(find.byType(NativeProductPicker), findsNothing);
    await tester.pumpWidget(const SizedBox());
  });
  testWidgets('company Douyin editor offers the existing product picker', (
    tester,
  ) async {
    final m = await model();
    await tester.pumpWidget(
      ModelScope(
        model: m,
        child: const MaterialApp(home: NativeDouyinEditor()),
      ),
    );
    await tester.pumpAndSettle();
    await tester.ensureVisible(find.text('关联产品（选填）'));
    await tester.tap(find.text('关联产品（选填）'));
    await tester.pumpAndSettle();
    expect(find.byType(NativeProductPicker), findsOneWidget);
    await tester.pumpWidget(const SizedBox());
  });
}
