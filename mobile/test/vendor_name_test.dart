import 'dart:convert';
import 'dart:io';
import 'package:dalunongji/main.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test('company conflict identifies an existing showroom in both languages', () async {
    SharedPreferences.setMockInitialValues({});
    final model = AppModel(await SharedPreferences.getInstance(), {
      for (final language in ['zh', 'en'])
        language: asMap(jsonDecode(File('assets/i18n/$language.json').readAsStringSync())),
    });
    expect(model.t('vendor_name_exists'), contains('企业名称已存在'));
    expect(fieldIssueText(model, {'field': 'name', 'code': 'vendor_name_exists'}), contains('企业名称已存在'));
    model.language = 'en';
    expect(model.t('vendor_name_exists'), contains('company name already exists'));
    expect(fieldIssueText(model, {'field': 'name', 'code': 'vendor_name_exists'}), contains('company name already exists'));
  });
}
