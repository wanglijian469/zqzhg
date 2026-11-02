import 'dart:async';
import 'dart:convert';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
// ignore: depend_on_referenced_packages
import 'package:video_player_platform_interface/video_player_platform_interface.dart';
import 'app_navigation_test.dart' as f;

class PopupVideoPlatform extends VideoPlayerPlatform {
  int creates = 0, plays = 0, disposals = 0;
  final volumes = <double>[];
  Completer<void>? initialization, disposal;
  @override
  Future<void> init() async {}
  @override
  Future<int?> createWithOptions(VideoCreationOptions options) async =>
      ++creates;
  @override
  Stream<VideoEvent> videoEventsFor(int playerId) {
    final event = VideoEvent(
      eventType: VideoEventType.initialized,
      duration: const Duration(minutes: 1),
      size: const Size(640, 360),
    );
    if (initialization == null) return Stream.value(event);
    final events = StreamController<VideoEvent>();
    initialization!.future.then((_) {
      if (!events.isClosed) events.add(event);
    });
    return events.stream;
  }

  @override
  Future<void> dispose(int playerId) async {
    disposals++;
    if (disposal != null) await disposal!.future;
  }

  @override
  Future<void> play(int playerId) async {
    plays++;
  }

  @override
  Future<void> pause(int playerId) async {}
  @override
  Future<void> setLooping(int playerId, bool looping) async {}
  @override
  Future<void> setVolume(int playerId, double volume) async {
    volumes.add(volume);
  }

  @override
  Future<void> setPlaybackSpeed(int playerId, double speed) async {}
  @override
  Future<Duration> getPosition(int playerId) async => Duration.zero;
  @override
  Widget buildViewWithOptions(VideoViewOptions options) => const SizedBox();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  const channel = MethodChannel('com.zqzt.dalunongji/video');
  final messenger =
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  setUp(() {
    final view =
        TestWidgetsFlutterBinding.instance.platformDispatcher.views.first;
    view.physicalSize = const Size(800, 2200);
    view.devicePixelRatio = 1;
    addTearDown(view.resetPhysicalSize);
    addTearDown(view.resetDevicePixelRatio);
  });
  const id = '7690195495465012480';
  const ready = {
    'status': 'READY',
    'video_id': id,
    'player_url': 'https://open.douyin.com/player/video?vid=$id&autoplay=0',
    'width': 1920,
    'height': 1080,
  };
  Json item({bool official = false}) => {
    'id': 8,
    'kind': 'video',
    'data': {
      'title': '测试播放',
      'file_media_id': 0,
      'description': '',
      'product_id': 0,
      if (official) 'video_source': 'DOUYIN',
    },
    'file': {'state': 'ready'},
  };
  tearDown(() async {
    await NativeVideoSessions.stopAll();
    messenger.setMockMethodCallHandler(channel, null);
  });
  testWidgets(
    'local card opens one playback route and starts audible playback once',
    (tester) async {
      final platform = PopupVideoPlatform();
      final previous = VideoPlayerPlatform.instance;
      VideoPlayerPlatform.instance = platform;
      addTearDown(() => VideoPlayerPlatform.instance = previous);
      final m = await f.fixture();
      await tester.pumpWidget(
        f.harness(
          m,
          home: Scaffold(body: NativePublicResourceCard(item: item())),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(NativeInlineResource), findsNothing);
      expect(platform.creates, 0);
      await tester.tap(find.byType(InkWell).first);
      await tester.pumpAndSettle();
      expect(find.byType(NativeResourceViewer), findsOneWidget);
      expect(platform.creates, 1);
      expect(platform.plays, 1);
      expect(platform.volumes.last, 1);
      await tester.tap(find.text('全屏'));
      await tester.pumpAndSettle();
      expect(find.text('退出全屏'), findsOneWidget);
      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      expect(find.byType(NativeResourceViewer), findsOneWidget);
      expect(find.text('退出全屏'), findsNothing);
      expect(platform.creates, 1);
      expect(platform.plays, 1);
      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      await tester.runAsync(() async {
        await Future<void>.delayed(const Duration(milliseconds: 30));
      });
      await tester.pump();
      expect(platform.disposals, 1);
      expect(find.byType(NativePublicResourceCard), findsOneWidget);
      await tester.tap(find.byType(InkWell).first);
      await tester.pumpAndSettle();
      expect(platform.creates, 2);
      expect(platform.plays, 2);
      await tester.pumpWidget(const SizedBox());
      await tester.runAsync(() async {
        await Future<void>.delayed(const Duration(milliseconds: 30));
      });
    },
  );
  testWidgets(
    'official card resolves once and opens native window without an intermediate route',
    (tester) async {
      var resolutions = 0, opens = 0;
      final closed = Completer<void>();
      messenger.setMockMethodCallHandler(channel, (call) async {
        opens++;
        await closed.future;
        return null;
      });
      final m = await f.fixture(
        intercept: (r) async {
          if (!r.url.path.endsWith('/resources/8/playback')) return null;
          resolutions++;
          return http.Response(
            jsonEncode(ready),
            200,
            headers: {'content-type': 'application/json'},
          );
        },
      );
      await tester.pumpWidget(
        f.harness(
          m,
          home: Scaffold(
            body: NativePublicResourceCard(item: item(official: true)),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byType(InkWell).first);
      await tester.pump();
      await tester.tap(find.byType(InkWell).first);
      await tester.pumpAndSettle();
      expect(resolutions, 1);
      expect(opens, 1);
      expect(find.byType(NativeResourceViewer), findsNothing);
      expect(find.byType(NativeInlineResource), findsNothing);
      closed.complete();
      await tester.pumpAndSettle();
      await tester.pumpWidget(const SizedBox());
      await tester.runAsync(() async {
        await Future<void>.delayed(const Duration(milliseconds: 30));
      });
    },
  );
  testWidgets(
    'pre-save official preview opens directly and retains editor input',
    (tester) async {
      var opens = 0, checks = 0, resolutions = 0;
      messenger.setMockMethodCallHandler(channel, (_) async {
        opens++;
        return null;
      });
      final m = await f.fixture(
        intercept: (r) async {
          if (r.url.path.endsWith('/playback')) resolutions++;
          if (!r.url.path.endsWith('/merchant/video-links/check')) return null;
          checks++;
          return http.Response(
            jsonEncode({'check': ready}),
            200,
            headers: {'content-type': 'application/json'},
          );
        },
      );
      await tester.pumpWidget(f.harness(m, home: const NativeDouyinEditor()));
      await tester.pumpAndSettle();
      Finder field(String label) => find.byWidgetPredicate(
        (w) => w is TextField && w.decoration?.labelText == label,
      );
      const share = 'https://www.douyin.com/video/7690195495465012480';
      await tester.enterText(field('视频分享链接'), share);
      await tester.enterText(field('视频名称'), '保留标题');
      await tester.tap(find.text('检查链接'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('预览播放'));
      await tester.pumpAndSettle();
      expect(opens, 1);
      expect(checks, 1);
      expect(resolutions, 0);
      expect(find.byType(NativeResourceViewer), findsNothing);
      expect(find.byType(NativeInlineResource), findsNothing);
      expect(tester.widget<TextField>(field('视频分享链接')).controller!.text, share);
      expect(tester.widget<TextField>(field('视频名称')).controller!.text, '保留标题');
      expect(
        tester
            .widget<OutlinedButton>(find.widgetWithText(OutlinedButton, '预览播放'))
            .onPressed,
        isNotNull,
      );
      await tester.pumpWidget(const SizedBox());
      await tester.runAsync(() async {
        await Future<void>.delayed(const Duration(milliseconds: 30));
      });
    },
  );
  for (final change in ['leave', 'background', 'session']) {
    testWidgets('official resolve cannot late-open after $change', (
      tester,
    ) async {
      var opens = 0;
      final pending = Completer<http.Response>();
      messenger.setMockMethodCallHandler(channel, (_) async {
        opens++;
        return null;
      });
      final m = await f.fixture(
        intercept: (r) async {
          if (r.url.path.endsWith('/resources/8/playback')) {
            return pending.future;
          }
          return null;
        },
      );
      await tester.pumpWidget(
        f.harness(m, home: const Scaffold(body: Text('原页面'))),
      );
      await tester.pumpAndSettle();
      final navigator = tester.state<NavigatorState>(find.byType(Navigator));
      unawaited(
        navigator.push(
          MaterialPageRoute<void>(
            builder: (_) => Scaffold(
              body: NativePublicResourceCard(item: item(official: true)),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byType(InkWell).first);
      await tester.pump();
      if (change == 'leave') {
        navigator.pop();
      }
      if (change == 'background') {
        tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
      }
      if (change == 'session') {
        m.token = 'different-token';
      }
      await tester.pumpAndSettle();
      pending.complete(
        http.Response(
          jsonEncode(ready),
          200,
          headers: {'content-type': 'application/json'},
        ),
      );
      await tester.pumpAndSettle();
      expect(opens, 0);
      if (change == 'background') {
        tester.binding.handleAppLifecycleStateChanged(
          AppLifecycleState.resumed,
        );
      }
      await tester.pumpAndSettle();
      expect(opens, 0);
      await tester.pumpWidget(const SizedBox());
      await tester.runAsync(() async {
        await Future<void>.delayed(const Duration(milliseconds: 30));
      });
    });
  }
  for (final change in ['covered', 'background', 'session']) {
    testWidgets('local initialization cannot late-play after $change', (
      tester,
    ) async {
      final platform = PopupVideoPlatform()..initialization = Completer<void>();
      final previous = VideoPlayerPlatform.instance;
      VideoPlayerPlatform.instance = platform;
      addTearDown(() => VideoPlayerPlatform.instance = previous);
      final m = await f.fixture();
      await tester.pumpWidget(
        f.harness(
          m,
          home: Scaffold(body: NativePublicResourceCard(item: item())),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byType(InkWell).first);
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      expect(platform.creates, 1);
      if (change == 'covered') {
        final nav = tester.state<NavigatorState>(find.byType(Navigator));
        unawaited(
          nav.push(
            MaterialPageRoute<void>(
              builder: (_) => const Scaffold(body: Text('其他页面')),
            ),
          ),
        );
      }
      if (change == 'background') {
        tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
      }
      if (change == 'session') m.token = 'changed-token';
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      platform.initialization!.complete();
      await tester.pumpAndSettle();
      expect(platform.plays, 0);
      if (change == 'background') {
        tester.binding.handleAppLifecycleStateChanged(
          AppLifecycleState.resumed,
        );
      }
      await tester.pumpAndSettle();
      expect(platform.plays, 0);
      await tester.pumpWidget(const SizedBox());
      await tester.runAsync(() async {
        await Future<void>.delayed(const Duration(milliseconds: 30));
      });
    });
  }
  testWidgets(
    'reopening waits for the previous playback window to release audio',
    (tester) async {
      final platform = PopupVideoPlatform()..disposal = Completer<void>();
      final previous = VideoPlayerPlatform.instance;
      VideoPlayerPlatform.instance = platform;
      addTearDown(() => VideoPlayerPlatform.instance = previous);
      final m = await f.fixture();
      await tester.pumpWidget(
        f.harness(
          m,
          home: Scaffold(body: NativePublicResourceCard(item: item())),
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byType(InkWell).first);
      await tester.pumpAndSettle();
      expect(platform.plays, 1);
      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();
      await tester.tap(find.byType(InkWell).first);
      await tester.pump();
      expect(platform.creates, 1);
      platform.disposal!.complete();
      await tester.runAsync(() async {
        await Future<void>.delayed(const Duration(milliseconds: 30));
      });
      await tester.pumpAndSettle();
      expect(platform.creates, 2);
      expect(platform.plays, 2);
      await tester.pumpWidget(const SizedBox());
      await tester.runAsync(() async {
        await Future<void>.delayed(const Duration(milliseconds: 30));
      });
    },
  );
}
