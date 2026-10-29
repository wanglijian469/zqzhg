import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test(
    'Cloud upgrade migrates old endpoint once, clears old session and preserves old drafts',
    () async {
      final oldScope = '${Uri.encodeComponent('http://172.27.35.2:8080')}.7';
      final draftKey = 'draft.$oldScope.vendor';
      SharedPreferences.setMockInitialValues({
        'base_url': 'http://172.27.35.2:8080',
        'token': 'old-token',
        'account': jsonEncode({'id': 7}),
        draftKey: 'private draft',
      });
      final prefs = await SharedPreferences.getInstance();
      final m = AppModel(prefs, {});
      expect(m.baseUrl, 'https://zqzhg.com');
      expect(m.token, isEmpty);
      expect(m.account, isEmpty);
      expect(prefs.getString(draftKey), 'private draft');
      expect(prefs.getString('token'), isNull);
      await prefs.setString('base_url', 'http://172.27.35.2:8080');
      final restarted = AppModel(prefs, {});
      expect(restarted.baseUrl, productionApiBaseUrl);
    },
  );
  test(
    'Fixed cloud origin retires explicit custom endpoints and their sessions',
    () async {
      SharedPreferences.setMockInitialValues({
        'base_url': 'https://custom.example',
        'token': 'custom-token',
        'account': jsonEncode({'id': 8}),
      });
      final m = AppModel(await SharedPreferences.getInstance(), {});
      expect(m.baseUrl, productionApiBaseUrl);
      expect(m.token, isEmpty);
      expect(m.account, isEmpty);
    },
  );
  test('Official upgrade preserves session and source data', () async {
    final draft = 'draft.${Uri.encodeComponent(productionApiBaseUrl)}.8.vendor';
    SharedPreferences.setMockInitialValues({
      'base_url': productionApiBaseUrl,
      'token': 'official',
      'account': jsonEncode({'id': 8, 'role': 'visitor'}),
      draft: 'cloud draft',
    });
    final prefs = await SharedPreferences.getInstance();
    final model = AppModel(prefs, {});
    expect(model.baseUrl, productionApiBaseUrl);
    expect(model.token, 'official');
    expect(model.account['id'], 8);
    expect(prefs.getString(draft), 'cloud draft');
  });
}
