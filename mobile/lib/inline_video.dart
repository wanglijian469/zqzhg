part of 'main.dart';

/// One audio owner across uploaded videos and official iframe players.
/// Call [activate] before creating/playing, and [release] after disposal.
class NativeVideoSessions {
  static Object? _owner, _candidate;
  static Future<void> Function()? _stop;
  static Future<void>? _draining;
  static int _generation = 0;

  static Future<void> _drain() {
    final stop = _stop;
    _owner = null;
    _stop = null;
    if (stop == null) return _draining ?? Future<void>.value();
    final previous = _draining;
    final next = Future<void>.sync(() async {
      if (previous != null) await previous;
      await stop();
    });
    _draining = next;
    next.then((_) {
      if (identical(_draining, next)) _draining = null;
    }, onError: (Object _, StackTrace _) {});
    return next;
  }

  static Future<bool> activate(
    Object owner,
    Future<void> Function() stop,
  ) async {
    if (identical(_owner, owner)) return true;
    final generation = ++_generation;
    _candidate = owner;
    try {
      await _drain();
    } catch (_) {
      // A failed release must not start another audio source.
      return false;
    }
    if (generation != _generation) return false;
    _candidate = null;
    _owner = owner;
    _stop = stop;
    return true;
  }

  static void release(Object owner) {
    if (identical(_candidate, owner)) {
      _candidate = null;
      _generation++;
    }
    if (identical(_owner, owner)) {
      _owner = null;
      _stop = null;
    }
  }

  static Future<void> stopAll() {
    _generation++;
    _candidate = null;
    return _drain();
  }
}

/// Token-free Android platform view, mounted in the current Flutter page.
class NativeOfficialInline extends StatefulWidget {
  final Json check;
  final VoidCallback? onStopped;
  const NativeOfficialInline({super.key, required this.check, this.onStopped});

  @override
  State<NativeOfficialInline> createState() => _NativeOfficialInlineState();
}

class _NativeOfficialInlineState extends State<NativeOfficialInline>
    with WidgetsBindingObserver {
  MethodChannel? _channel;
  bool _active = false, _fullscreen = false;
  String _error = '';
  int _generation = 0, _viewGeneration = 0;

  Uri? get _url => widget.check['status'] == 'READY'
      ? trustedDouyinPlayer(
          (widget.check['player_url'] ?? '').toString(),
          (widget.check['video_id'] ?? '').toString(),
        )
      : null;
  int _dimension(String key) =>
      (widget.check[key] is num ? (widget.check[key] as num).toInt() : 0).clamp(
        0,
        16384,
      );

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    unawaited(_start());
  }

  @override
  void didUpdateWidget(NativeOfficialInline oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.check['player_url'] != widget.check['player_url'] ||
        oldWidget.check['video_id'] != widget.check['video_id'] ||
        oldWidget.check['status'] != widget.check['status'] ||
        oldWidget.check['width'] != widget.check['width'] ||
        oldWidget.check['height'] != widget.check['height']) {
      unawaited(_replace());
    }
  }

  Future<void> _replace() async {
    final stopping = _stop(notify: false);
    final generation = _generation;
    await stopping;
    if (mounted && generation == _generation) await _start();
  }

  Future<void> _start() async {
    if (_url == null) return;
    final generation = ++_generation;
    final acquired = await NativeVideoSessions.activate(this, () => _stop());
    if (!mounted || generation != _generation || !acquired) return;
    setState(() {
      _active = true;
      _error = '';
      _viewGeneration++;
    });
  }

  Future<void> _invoke(String method) async {
    try {
      await _channel?.invokeMethod<void>(method);
    } on PlatformException {
      // Native teardown may already have removed this view's channel.
    } on MissingPluginException {
      // A disposed platform view has no remaining native method handler.
    }
  }

  Future<void> _stop({bool notify = true}) async {
    _generation++;
    await _invoke('stop');
    _channel?.setMethodCallHandler(null);
    _channel = null;
    NativeVideoSessions.release(this);
    if (!mounted) return;
    setState(() {
      _active = false;
      _fullscreen = false;
    });
    if (notify) widget.onStopped?.call();
  }

  void _created(int id) {
    final channel = MethodChannel('zqzg/official-video/$id');
    if (!mounted || !_active) {
      unawaited(channel.invokeMethod<void>('stop').catchError((Object _) {}));
      return;
    }
    _channel = channel;
    channel.setMethodCallHandler((call) async {
      if (!mounted || !identical(channel, _channel)) return;
      switch (call.method) {
        case 'fullscreen':
          setState(() => _fullscreen = call.arguments == true);
        case 'error':
          setState(() {
            _fullscreen = false;
            _error = '官方播放器暂时无法加载，请检查网络后重试。';
          });
          unawaited(_stop(notify: false));
        case 'stopped':
          await _stop();
      }
    });
    unawaited(_invoke('start'));
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed && _active) unawaited(_stop());
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _generation++;
    NativeVideoSessions.release(this);
    _channel?.setMethodCallHandler(null);
    unawaited(_invoke('stop'));
    _channel = null;
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_url == null) {
      return const Text('该视频暂时无法播放，请重新检查链接。');
    }
    if (Theme.of(context).platform != TargetPlatform.android) {
      return const Text('请使用 Android App 播放官方视频。');
    }
    final wide = _dimension('width') >= _dimension('height');
    final aspect = wide ? 800 / 485 : 324 / 672;
    return PopScope(
      canPop: !_fullscreen,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop && _fullscreen) unawaited(_invoke('exitFullscreen'));
      },
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 8),
            child: Text(
              '抖音官方播放器',
              style: TextStyle(fontWeight: FontWeight.w600),
            ),
          ),
          if (_active && _error.isEmpty)
            LayoutBuilder(
              builder: (context, constraints) {
                final width = min(constraints.maxWidth, 560 * aspect);
                return Center(
                  child: SizedBox(
                    width: width,
                    height: width / aspect,
                    child: AndroidView(
                      key: ValueKey(_viewGeneration),
                      viewType: 'zqzg/official-video',
                      creationParams: {
                        'url': _url.toString(),
                        'width': _dimension('width'),
                        'height': _dimension('height'),
                      },
                      creationParamsCodec: const StandardMessageCodec(),
                      onPlatformViewCreated: _created,
                    ),
                  ),
                );
              },
            )
          else if (_error.isNotEmpty || !_active)
            Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                children: [
                  Text(_error.isEmpty ? '视频已停止。' : _error),
                  TextButton(
                    onPressed: () async {
                      await _stop(notify: false);
                      if (mounted) await _start();
                    },
                    child: const Text('重新加载播放器'),
                  ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
