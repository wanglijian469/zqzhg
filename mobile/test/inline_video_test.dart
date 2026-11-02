import 'dart:async';
import 'package:dalunongji/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  const id = '7690195495465012480';
  const ready = <String, dynamic>{
    'status': 'READY',
    'video_id': id,
    'player_url': 'https://open.douyin.com/player/video?vid=$id&autoplay=0',
    'width': 1920,
    'height': 1080,
    'token': 'never-send-this',
  };
  final created = <int>[];
  final nativeCalls = <int, List<String>>{};
  final messenger =
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger;
  setUp(() {
    created.clear();
    nativeCalls.clear();
    messenger.setMockMethodCallHandler(SystemChannels.platform_views, (
      call,
    ) async {
      final args = call.arguments as Map;
      if (call.method == 'create') {
        final viewId = args['id'] as int;
        created.add(viewId);
        nativeCalls[viewId] = [];
        messenger.setMockMethodCallHandler(
          MethodChannel('zqzg/official-video/$viewId'),
          (call) async {
            nativeCalls[viewId]!.add(call.method);
            return call.method == 'exitFullscreen' ? true : null;
          },
        );
        return viewId;
      }
      if (call.method == 'resize') {
        return {'width': args['width'], 'height': args['height']};
      }
      return null;
    });
  });
  tearDown(() async {
    await NativeVideoSessions.stopAll();
    messenger.setMockMethodCallHandler(SystemChannels.platform_views, null);
    for (final id in created) {
      messenger.setMockMethodCallHandler(
        MethodChannel('zqzg/official-video/$id'),
        null,
      );
    }
  });

  Future<void> event(int id, String name, [Object? value]) async {
    const codec = StandardMethodCodec();
    await messenger.handlePlatformMessage(
      'zqzg/official-video/$id',
      codec.encodeMethodCall(MethodCall(name, value)),
      (_) {},
    );
  }

  Widget player() => MaterialApp(
    theme: ThemeData(platform: TargetPlatform.android),
    home: Scaffold(body: NativeOfficialInline(check: ready)),
  );

  test(
    'a new video session awaits release of the previous audio owner',
    () async {
      final first = Object(), second = Object();
      final stopped = Completer<void>();
      expect(
        await NativeVideoSessions.activate(first, () => stopped.future),
        isTrue,
      );
      var acquired = false;
      final next = NativeVideoSessions.activate(second, () async {}).then((ok) {
        acquired = ok;
      });
      await Future<void>.delayed(Duration.zero);
      expect(acquired, isFalse);
      stopped.complete();
      await next;
      expect(acquired, isTrue);
    },
  );

  test('stopAll invalidates a pending session acquisition', () async {
    final stopped = Completer<void>();
    await NativeVideoSessions.activate(Object(), () => stopped.future);
    final pending = NativeVideoSessions.activate(Object(), () async {});
    final stopping = NativeVideoSessions.stopAll();
    stopped.complete();
    await stopping;
    expect(await pending, isFalse);
  });

  test('rapid session switches all await the original teardown', () async {
    final stopped = Completer<void>();
    await NativeVideoSessions.activate(Object(), () => stopped.future);
    final superseded = NativeVideoSessions.activate(Object(), () async {});
    final newest = NativeVideoSessions.activate(Object(), () async {});
    var ready = false;
    final acquired = newest.then((ok) => ready = ok);
    await Future<void>.delayed(Duration.zero);
    expect(ready, isFalse);
    stopped.complete();
    expect(await superseded, isFalse);
    await acquired;
    expect(ready, isTrue);
  });

  test('disposing a waiting player prevents it from acquiring audio', () async {
    final stopped = Completer<void>();
    await NativeVideoSessions.activate(Object(), () => stopped.future);
    final owner = Object();
    final pending = NativeVideoSessions.activate(owner, () async {});
    NativeVideoSessions.release(owner);
    stopped.complete();
    expect(await pending, isFalse);
  });

  testWidgets('untrusted official player never creates a platform view', (
    tester,
  ) async {
    await tester.pumpWidget(
      MaterialApp(
        home: NativeOfficialInline(
          check: {
            ...ready,
            'player_url': 'https://evil.test/player/video?vid=$id&autoplay=0',
          },
        ),
      ),
    );
    expect(find.byType(AndroidView), findsNothing);
    expect(find.text('该视频暂时无法播放，请重新检查链接。'), findsOneWidget);
  });

  testWidgets(
    'inline creation sends only validated public URL and dimensions',
    (tester) async {
      await tester.pumpWidget(player());
      await tester.pumpAndSettle();
      final view = tester.widget<AndroidView>(find.byType(AndroidView));
      expect(view.viewType, 'zqzg/official-video');
      expect(view.creationParams, {
        'url': ready['player_url'],
        'width': 1920,
        'height': 1080,
      });
      expect(view.creationParamsCodec, isA<StandardMessageCodec>());
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );

  testWidgets(
    'player errors stop native playback and retry only after a user press',
    (tester) async {
      await tester.pumpWidget(player());
      await tester.pumpAndSettle();
      expect(created, hasLength(1));
      await event(created.single, 'error');
      await tester.pumpAndSettle();
      expect(find.byType(AndroidView), findsNothing);
      expect(find.text('官方播放器暂时无法加载，请检查网络后重试。'), findsOneWidget);
      await tester.pump(const Duration(seconds: 35));
      expect(created, hasLength(1));
      await tester.tap(find.text('重新加载播放器'));
      await tester.pumpAndSettle();
      expect(created, hasLength(2));
      expect(nativeCalls[created.first], contains('stop'));
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );

  testWidgets(
    'background unloads the view and foreground never auto reloads it',
    (tester) async {
      await tester.pumpWidget(player());
      await tester.pumpAndSettle();
      final id = created.single;
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.paused);
      await tester.pumpAndSettle();
      expect(nativeCalls[id], contains('stop'));
      // Flutter suspends frame rendering while paused; verify removal on resume.
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pumpAndSettle();
      expect(find.byType(AndroidView), findsNothing);
      expect(created, hasLength(1));
      await tester.pumpWidget(const SizedBox.shrink());
    },
  );

  testWidgets(
    'back exits in-place fullscreen before leaving the current route',
    (tester) async {
      final navigator = GlobalKey<NavigatorState>();
      await tester.pumpWidget(
        MaterialApp(
          navigatorKey: navigator,
          theme: ThemeData(platform: TargetPlatform.android),
          home: const Scaffold(body: Text('resource list')),
        ),
      );
      unawaited(
        navigator.currentState!.push(
          MaterialPageRoute<void>(
            builder: (_) => Scaffold(body: NativeOfficialInline(check: ready)),
          ),
        ),
      );
      await tester.pumpAndSettle();
      final id = created.single;
      await event(id, 'fullscreen', true);
      await tester.pumpAndSettle();
      await navigator.currentState!.maybePop();
      await tester.pumpAndSettle();
      expect(nativeCalls[id], contains('exitFullscreen'));
      expect(find.byType(AndroidView), findsOneWidget);
      await event(id, 'fullscreen', false);
      await tester.pumpAndSettle();
      await navigator.currentState!.maybePop();
      await tester.pumpAndSettle();
      expect(find.text('resource list'), findsOneWidget);
      expect(find.byType(AndroidView), findsNothing);
    },
  );
}
