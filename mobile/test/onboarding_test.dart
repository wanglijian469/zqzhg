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
  test('Shared rules distinguish Unicode characters and UTF8 bytes', () {
    final data = <String, dynamic>{
      'name': List.filled(200, '😀').join(),
      'main_products': '机械',
      'region_name': '河北',
      'phone': '0319-1234567',
      'address': List.filled(166, '中').join(),
    };
    expect(validateFormData(data, 'vendor'), isEmpty);
    data['address'] += '中';
    expect(validateFormData(data, 'vendor')['address']?['code'], 'too_long');
    data['company_profile'] = {
      'website': 'www.example.com',
      'contacts': [
        {'name': '张经理'},
      ],
    };
    final issues = validateFormData(data, 'vendor');
    expect(
      issues.keys,
      containsAll([
        'company_profile.website',
        'company_profile.contacts.0.phone',
      ]),
    );
  });
  testWidgets(
    'Guest onboarding explains saving and identifies fields before login',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      final m = AppModel(
        await SharedPreferences.getInstance(),
        {
          for (final lang in ['zh', 'en'])
            lang: asMap(
              jsonDecode(File('assets/i18n/$lang.json').readAsStringSync()),
            ),
        },
        client: MockClient(
          (_) async => http.Response(
            '{"categories":[],"regions":[],"items":[],"total":0}',
            200,
          ),
        ),
      );
      await tester.pumpWidget(
        ModelScope(
          model: m,
          child: MaterialApp(
            theme: industrialTheme(),
            home: const EditorScreen(kind: 'vendor'),
          ),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(NativeJoinGuide), findsOneWidget);
      await tester.enterText(
        find.widgetWithText(TextField, '企业 / 工厂名称'),
        '保留企业',
      );
      await tester.scrollUntilVisible(
        find.widgetWithText(FilledButton, '提交审核'),
        400,
        scrollable: find.byType(Scrollable).first,
      );
      await tester.tap(find.widgetWithText(FilledButton, '提交审核'));
      await tester.pumpAndSettle();
      await tester.scrollUntilVisible(
        find.text('请修改以下项目，填写内容已保留'),
        -200,
        scrollable: find.byType(Scrollable).first,
      );
      expect(find.text('请修改以下项目，填写内容已保留'), findsOneWidget);
      expect(find.byType(LoginScreen), findsNothing);
      await tester.scrollUntilVisible(
        find.widgetWithText(TextField, '企业 / 工厂名称'),
        -250,
        scrollable: find.byType(Scrollable).first,
      );
      expect(
        tester
            .widget<TextField>(find.widgetWithText(TextField, '企业 / 工厂名称'))
            .controller!
            .text,
        '保留企业',
      );
      expect(tester.takeException(), isNull);
      await tester.pumpWidget(const SizedBox());
    },
  );
}
