import 'native_field_finder.dart';
import 'dart:convert';
import 'dart:io';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  Future<AppModel> model() async {
    SharedPreferences.setMockInitialValues({});
    return AppModel(await SharedPreferences.getInstance(), {
      for (final lang in ['zh', 'en'])
        lang: asMap(
          jsonDecode(File('assets/i18n/$lang.json').readAsStringSync()),
        ),
    });
  }

  testWidgets('Language switching preserves the private local booth draft', (
    tester,
  ) async {
    final m = await model();
    await tester.pumpWidget(
      ModelScope(
        model: m,
        child: AnimatedBuilder(
          animation: m,
          builder: (context, _) =>
              MaterialApp(home: const EditorScreen(kind: 'vendor')),
        ),
      ),
    );
    await tester.pump();
    await tester.enterText(find.byType(TextField).at(0), '测试草稿仅本机');
    await tester.tap(find.text('历史或未覆盖地区，保留文字'));
    await tester.pumpAndSettle();
    await tester.enterText(nativeField('所在地区'), '河北');
    await tester.tap(find.widgetWithText(FilledButton, '保存'));
    await tester.pumpAndSettle();
    await tester.pump(const Duration(milliseconds: 350));
    final draftKey = 'draft.${m.scope}.vendor';
    expect(m.prefs.getString(draftKey), contains('测试草稿仅本机'));
    await tester.tap(find.text('English'));
    await tester.pump();
    expect(m.language, 'en');
    expect(find.text('测试草稿仅本机'), findsOneWidget);
    expect(find.text('河北'), findsOneWidget);
    expect(m.prefs.getString(draftKey), contains('测试草稿仅本机'));
    await tester.pumpWidget(const SizedBox());
  });
  test(
    'An old foreign endpoint is retired without deleting source drafts',
    () async {
      SharedPreferences.setMockInitialValues({
        'base_url': 'http://old-fixture',
        'token': 'old',
        'account': jsonEncode({'id': 7}),
        'draft.old.vendor': 'saved',
      });
      final prefs = await SharedPreferences.getInstance();
      final m = AppModel(prefs, {});
      expect(m.baseUrl, productionApiBaseUrl);
      expect(m.token, isEmpty);
      expect(m.account, isEmpty);
      expect(prefs.getString('draft.old.vendor'), 'saved');
    },
  );
}
