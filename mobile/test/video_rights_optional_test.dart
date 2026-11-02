import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'app_navigation_test.dart' as f;
import 'service_fixture.dart';

void main() {
  testWidgets(
    'accepted service agreements remove repeated rights checkbox without sending invented consent',
    (tester) async {
      tester.view.physicalSize = const Size(800, 2400);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      final docs = serviceFixtureDocuments();
      Map<String, dynamic>? saved;
      final m = await f.fixture(
        intercept: (r) async {
          if (r.url.path.endsWith('/merchant/verification')) {
            return http.Response(
              jsonEncode({
                'documents': docs,
                'acceptances': [
                  for (final d in docs)
                    {'document_id': d['id'], 'digest': d['digest']},
                ],
              }),
              200,
              headers: {'content-type': 'application/json; charset=utf-8'},
            );
          }
          if (r.url.path.endsWith('/merchant/resources') &&
              r.method == 'POST') {
            saved = jsonDecode(r.body) as Map<String, dynamic>;
            return http.Response(
              jsonEncode({
                'id': 10,
                'revision_id': 11,
                'kind': 'video',
                'status': 'draft',
                'data': saved!['data'],
              }),
              201,
              headers: {'content-type': 'application/json; charset=utf-8'},
            );
          }
          return null;
        },
      );
      await tester.pumpWidget(f.harness(m, home: const NativeDouyinEditor()));
      await tester.pumpAndSettle();
      expect(find.byType(CheckboxListTile), findsNothing);
      expect(find.text('服务协议确认'), findsNothing);
      Finder field(String label) => find.byWidgetPredicate(
        (w) => w is TextField && w.decoration?.labelText == label,
      );
      await tester.enterText(
        field('视频分享链接'),
        'https://www.douyin.com/video/7136314708138724640',
      );
      await tester.enterText(field('视频名称'), '正常保存');
      await tester.ensureVisible(find.text('保存云端草稿'));
      await tester.tap(find.text('保存云端草稿'));
      await tester.pumpAndSettle();
      expect(saved, isNotNull);
      expect((saved!['data'] as Map).containsKey('rights_confirmed'), isFalse);
      expect(find.text('草稿已保存，提交审核通过后才会公开。'), findsOneWidget);
    },
  );
}
