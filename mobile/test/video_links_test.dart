import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:dalunongji/main.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  const id = '7690195495465012480';
  const url = 'https://open.douyin.com/player/video?vid=$id&autoplay=0';
  test(
    'only official player with matching string identity and no autoplay',
    () {
      expect(trustedDouyinPlayer(url, id), isNotNull);
      for (final raw in [
        url.replaceFirst('open.douyin.com', 'evil.test'),
        url.replaceFirst('autoplay=0', 'autoplay=1'),
        url.replaceFirst('https:', 'http:'),
        url.replaceFirst(id, '123'),
        url.replaceFirst('.com/', '.com:444/'),
      ]) {
        expect(trustedDouyinPlayer(raw, id), isNull);
      }
    },
  );
  test('historical video without source remains local', () {
    expect(
      isDouyinVideo({
        'kind': 'video',
        'data': {'file_media_id': 42},
      }),
      isFalse,
    );
    expect(
      isDouyinVideo({
        'kind': 'video',
        'data': {'video_source': 'DOUYIN'},
      }),
      isTrue,
    );
    expect(
      videoV2('/public/resources?kind=video'),
      endsWith('&video_api_version=2'),
    );
  });
  test('native channel receives only public player and dimensions', () async {
    MethodCall? sent;
    const channel = MethodChannel('com.zqzt.dalunongji/video');
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, (call) async {
          sent = call;
          return null;
        });
    await openOfficialVideo({
      'status': 'READY',
      'video_id': id,
      'player_url': url,
      'width': 1920,
      'height': 1080,
      'token': 'must-not-cross',
    });
    expect(sent?.method, 'open');
    expect(
      (sent?.arguments as Map).keys,
      unorderedEquals(['url', 'width', 'height']),
    );
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(channel, null);
  });
}
