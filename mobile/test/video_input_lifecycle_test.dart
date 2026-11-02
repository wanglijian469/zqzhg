import 'dart:convert';
import 'dart:async';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'app_navigation_test.dart' as f;

void main() {
  testWidgets('editing during a check discards the old response and never retries automatically', (tester) async {
    tester.view.physicalSize = const Size(800, 2200);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);
    final pending = Completer<http.Response>();
    var checks = 0;
    final m = await f.fixture(intercept: (r) async {
      if (!r.url.path.endsWith('/merchant/video-links/check')) return null;
      checks++;
      return pending.future;
    });
    await tester.pumpWidget(f.harness(m, home: const NativeDouyinEditor()));
    await tester.pumpAndSettle();
    final input = find.byWidgetPredicate((w) => w is TextField && w.decoration?.labelText == '视频分享链接');
    await tester.enterText(input, 'https://www.douyin.com/video/7690195495465012480');
    await tester.tap(find.text('检查链接'));
    await tester.pump();
    await tester.enterText(input, 'https://www.douyin.com/video/7382166514377510182');
    pending.complete(http.Response(jsonEncode({'check': {'status':'READY','video_id':'7690195495465012480','title':'过时响应','player_url':'https://open.douyin.com/player/video?vid=7690195495465012480&autoplay=0'}}), 200,headers:{'content-type':'application/json; charset=utf-8'}));
    await tester.pumpAndSettle();
    expect(tester.widget<OutlinedButton>(find.widgetWithText(OutlinedButton, '预览播放')).onPressed, isNull);
    expect(tester.widget<TextField>(input).controller!.text, contains('7382166514377510182'));
    expect(find.text('过时响应'), findsNothing);
    await tester.pump(const Duration(seconds: 30));
    expect(checks, 1);
    await tester.pumpWidget(const SizedBox());
  });

  for (final code in ['invalid_video_link','video_not_public','network_error']) {
    testWidgets('$code stops checking and preserves typed input without loading a player', (tester) async {
      tester.view.physicalSize = const Size(800,2200);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      var checks=0;
      final m=await f.fixture(intercept:(r) async {
        if(!r.url.path.endsWith('/merchant/video-links/check')) return null;
        checks++;
        return http.Response(jsonEncode({'error':{'code':code}}),code=='network_error'?503:400);
      });
      await tester.pumpWidget(f.harness(m,home:const NativeDouyinEditor()));
      await tester.pumpAndSettle();
      final input=find.byWidgetPredicate((w)=>w is TextField&&w.decoration?.labelText=='视频分享链接');
      await tester.enterText(input,'https://www.douyin.com/video/7690195495465012480');
      await tester.tap(find.text('检查链接'));
      await tester.pumpAndSettle();
      expect(tester.widget<TextField>(input).controller!.text,contains('7690195495465012480'));
      expect(find.byType(NativeOfficialInline),findsNothing);
      expect(tester.widget<OutlinedButton>(find.widgetWithText(OutlinedButton,'预览播放')).onPressed,isNull);
      await tester.pump(const Duration(seconds:30));
      expect(checks,1);
      expect(tester.takeException(),isNull);
      await tester.pumpWidget(const SizedBox());
    });
  }
  testWidgets(
    'cursor and repeated paste do not invalidate a successful check or rights',
    (tester) async {
      tester.view.physicalSize = const Size(800, 2200);
      tester.view.devicePixelRatio = 1;
      addTearDown(tester.view.resetPhysicalSize);
      addTearDown(tester.view.resetDevicePixelRatio);
      var checks = 0;
      final m = await f.fixture(
        intercept: (r) async {
          if (!r.url.path.endsWith('/merchant/video-links/check')) return null;
          checks++;
          return http.Response(
            jsonEncode({
              'check': {
                'source': 'DOUYIN',
                'video_id': '7690195495465012480',
                'status': 'READY',
                'title': '测试视频',
                'player_url':
                    'https://open.douyin.com/player/video?vid=7690195495465012480&autoplay=0',
                'width': 1920,
                'height': 1080,
              },
            }),
            200,
            headers: {'content-type': 'application/json; charset=utf-8'},
          );
        },
      );
      await tester.pumpWidget(f.harness(m, home: const NativeDouyinEditor()));
      await tester.pumpAndSettle();
      final input = find.byWidgetPredicate(
        (w) => w is TextField && w.decoration?.labelText == '视频分享链接',
      );
      await tester.enterText(
        input,
        'https://www.douyin.com/video/7690195495465012480',
      );
      await tester.pumpAndSettle();
      expect(checks, 0);
      await tester.tap(find.text('检查链接'));
      await tester.pumpAndSettle();
      expect(checks, 1);
      await tester.ensureVisible(find.byType(CheckboxListTile));
      await tester.tap(find.byType(CheckboxListTile));
      await tester.pumpAndSettle();
      final controller = tester.widget<TextField>(input).controller!;
      controller.selection = const TextSelection.collapsed(offset: 5);
      await tester.pumpAndSettle();
      expect(
        tester.widget<CheckboxListTile>(find.byType(CheckboxListTile)).value,
        isTrue,
      );
      expect(
        tester
            .widget<OutlinedButton>(find.widgetWithText(OutlinedButton, '预览播放'))
            .onPressed,
        isNotNull,
      );
      expect(checks, 1);
      await tester.pumpWidget(const SizedBox());
    },
  );
}
