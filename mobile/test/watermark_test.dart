import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'mobile_upgrade_test.dart' as f;

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  testWidgets(
    'Watermarked server image IDs keep cover order across saves and draft restore',
    (t) async {
      final writes = <Json>[];
      final m = await f.upgradeModel(
        signed: true,
        handler: (r) async {
          if (r.url.path.endsWith('/44/draft')) {
            final data = asMap(jsonDecode(r.body)['data']);
            writes.add(data);
            return f.jsonResponse({
              'id': 44,
              'kind': 'product',
              'status': 'draft',
              'revision_id': writes.length + 1,
              'data': {
                ...data,
                'media_ids': writes.length == 1
                    ? [103, 101, 102]
                    : data['media_ids'],
              },
            });
          }
          return null;
        },
      );
      await t.pumpWidget(
        f.upgradeApp(
          m,
          const EditorScreen(
            kind: 'product',
            item: {
              'id': 44,
              'kind': 'product',
              'status': 'draft',
              'revision_id': 1,
              'data': {
                'name': '水印排序验收',
                'media_ids': [1, 2, 3],
              },
            },
          ),
        ),
      );
      await t.pumpAndSettle();
      await f.tapReachable(t, find.textContaining('2. 图片'));
      await f.tapReachable(t, find.text('设为封面').last);
      await f.tapReachable(t, find.widgetWithText(OutlinedButton, '保存云端草稿'));
      expect(writes.first['media_ids'], [3, 1, 2]);
      await f.tapReachable(t, find.widgetWithText(OutlinedButton, '保存云端草稿'));
      expect(writes.last['media_ids'], [103, 101, 102]);
      final draft = jsonDecode(m.prefs.getString('draft.${m.scope}.44')!);
      expect(draft['data']['media_ids'], [103, 101, 102]);
      expect(draft['gallery_order'], ['m:103', 'm:101', 'm:102']);
      await t.pumpWidget(const SizedBox());
    },
  );
}
