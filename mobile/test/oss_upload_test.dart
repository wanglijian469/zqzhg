import 'dart:convert';
import 'dart:io';
import 'package:dalunongji/main.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  test(
    'cloud upload uses file bytes and no website bearer; signed access remains ephemeral',
    () async {
      SharedPreferences.setMockInitialValues({
        'base_url': 'https://oss-upload.test',
        'token': 'site-bearer',
        'account': jsonEncode({'id': 9, 'role': 'merchant'}),
        'retired_boundary_cache_cleared': true,
      });
      final prefs = await SharedPreferences.getInstance();
      final dir = await Directory.systemTemp.createTemp('zqzt-oss-test-');
      addTearDown(() => dir.delete(recursive: true));
      final file = File('${dir.path}/photo.png');
      await file.writeAsBytes([1, 2, 3, 4]);
      http.Response json(dynamic value) => http.Response(
        jsonEncode(value),
        200,
        headers: {'content-type': 'application/json'},
      );
      final model = AppModel(
        prefs,
        {},
        testBaseUrl: 'https://oss-upload.test',
        analyticsEnabled: false,
        client: MockClient((request) async {
          if (request.url.path.endsWith('/public/config')) {
            return json({
              'capabilities': {
                'oss_direct_upload': true,
                'oss_file_access': true,
              },
            });
          }
          expect(request.headers['Authorization'], 'Bearer site-bearer');
          if (request.url.path.endsWith('/uploads')) {
            final data = jsonDecode(request.body);
            expect(data['size'], 4);
            return json({
              'session': {'id': 'upload-session', 'state': 'uploading'},
              'mode': 'single',
              'url': 'https://upload.zqzhg.com/private?signature=temporary',
              'headers': {'Content-Length': '4', 'Content-Type': 'image/png'},
            });
          }
          if (request.url.path.endsWith('/access')) {
            return json({
              'url': 'https://upload.zqzhg.com/private?signature=temporary',
              'direct': true,
            });
          }
          if (request.url.path.endsWith('/resource-media/42')) {
            return json({'media_id': 42, 'kind': 'image', 'state': 'ready'});
          }
          return json({
            'session': {
              'id': 'upload-session',
              'state': 'completed',
              'media_id': 42,
            },
          });
        }),
      );
      var putCount = 0;
      final transport = MockClient((request) async {
        putCount++;
        expect(request.method, 'PUT');
        expect(request.headers.containsKey('Authorization'), false);
        expect(request.contentLength, 4);
        expect(request.bodyBytes, [1, 2, 3, 4]);
        return http.Response('', 200);
      });
      addTearDown(transport.close);
      final result = await model.tryCloudUpload(
        file.path,
        uploadClient: transport,
      );
      expect(result?['media_id'], 42);
      expect(putCount, 1);
      expect(
        prefs.getKeys().where((k) => k.startsWith('oss.upload.')),
        isEmpty,
      );
      final access = await model.privateFileAccess(42);
      expect(access['direct'], true);
      expect(prefs.getKeys().any((k) => k.contains('signature=')), false);
    },
  );
}
