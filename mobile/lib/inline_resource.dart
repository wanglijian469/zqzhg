part of 'main.dart';

/// The same video surface is used by public cards, management and draft previews.
class NativeInlineResource extends StatefulWidget {
  final Json item;
  final Json? check;
  final bool private, autoStart;
  const NativeInlineResource({
    super.key,
    required this.item,
    this.check,
    this.private = false,
    this.autoStart = false,
  });
  @override
  State<NativeInlineResource> createState() => _NativeInlineResourceState();
}

class _NativeInlineResourceState extends State<NativeInlineResource>
    with WidgetsBindingObserver {
  VideoPlayerController? video;
  Json? checked;
  String error = '', scope = '';
  bool busy = false, fullscreen = false;
  int generation = 0;
  DialogRoute<void>? fullRoute;
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    if (widget.autoStart) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) start();
      });
    }
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final next = ModelScope.of(context).scope;
    final route = ModalRoute.of(context);
    if (scope.isNotEmpty &&
        (scope != next || route != null && !route.isCurrent && !fullscreen)) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) stop();
      });
    }
    scope = next;
  }

  Future<void> stop() async {
    generation++;
    busy = false;
    final old = video;
    video = null;
    checked = null;
    final route = fullRoute;
    fullRoute = null;
    if (mounted && route?.isActive == true) {
      Navigator.of(context).removeRoute(route!);
    }
    fullscreen = false;
    if (old != null) {
      await old.pause();
      await old.dispose();
    }
    NativeVideoSessions.release(this);
    if (mounted) setState(() {});
  }

  Future<void> start() async {
    if (busy) return;
    final g = ++generation;
    setState(() {
      busy = true;
      error = '';
    });
    try {
      final m = ModelScope.of(context);
      if (isDouyinVideo(widget.item)) {
        final value =
            widget.check ??
            asMap(
              await m.api(
                videoV2(
                  '/${widget.private ? 'merchant' : 'public'}/resources/${widget.item['id']}/playback',
                ),
              ),
            );
        if (!mounted || g != generation) return;
        if (value['status'] != 'READY' ||
            trustedDouyinPlayer(
                  (value['player_url'] ?? '').toString(),
                  (value['video_id'] ?? '').toString(),
                ) ==
                null) {
          throw ApiFailure((value['reason'] ?? 'video_not_public').toString());
        }
        setState(() => checked = value);
      } else {
        final data = asMap(widget.item['data']);
        if ((data['file_media_id'] ?? 0) == 0 ||
            widget.item['file']?['state'] != null &&
                widget.item['file']['state'] != 'ready') {
          throw ApiFailure('media_not_ready');
        }
        if (!await NativeVideoSessions.activate(this, stop) ||
            !mounted ||
            g != generation) {
          return;
        }
        final next = VideoPlayerController.networkUrl(
          Uri.parse('${m.baseUrl}/api/v1/media/${data['file_media_id']}'),
          httpHeaders: widget.private
              ? {'Authorization': 'Bearer ${m.token}'}
              : {},
        );
        video = next;
        await next.initialize();
        if (!mounted || g != generation) {
          if (identical(video, next)) video = null;
          await next.dispose();
          return;
        }
        await next.play();
        setState(() {});
      }
    } catch (e) {
      if (mounted && g == generation) {
        setState(
          () => error = isDouyinVideo(widget.item)
              ? videoErrorText(e)
              : ModelScope.of(context).t('network_error'),
        );
      }
    } finally {
      if (mounted && g == generation) setState(() => busy = false);
    }
  }

  Future<void> enterFullscreen() async {
    if (video == null || fullscreen) return;
    setState(() => fullscreen = true);
    final route = DialogRoute<void>(
      context: context,
      barrierDismissible: false,
      builder: (c) => Dialog.fullscreen(
        backgroundColor: Colors.black,
        child: SafeArea(
          child: Column(
            children: [
              TextButton(
                onPressed: () => Navigator.pop(c),
                child: const Text('退出全屏'),
              ),
              Expanded(
                child: Center(
                  child: AspectRatio(
                    aspectRatio: video!.value.aspectRatio,
                    child: VideoPlayer(video!),
                  ),
                ),
              ),
              controls(),
            ],
          ),
        ),
      ),
    );
    fullRoute = route;
    try {
      await Navigator.of(context).push(route);
    } finally {
      fullRoute = null;
      if (mounted) setState(() => fullscreen = false);
    }
  }

  Widget controls() => Wrap(
    alignment: WrapAlignment.center,
    spacing: 12,
    children: [
      TextButton.icon(
        onPressed: () {
          if (video == null) return;
          setState(() {
            video!.value.isPlaying ? video!.pause() : video!.play();
          });
        },
        icon: Icon(
          video?.value.isPlaying == true ? Icons.pause : Icons.play_arrow,
        ),
        label: Text(video?.value.isPlaying == true ? '暂停' : '播放'),
      ),
      if (!fullscreen)
        TextButton(onPressed: enterFullscreen, child: const Text('全屏')),
    ],
  );
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed) unawaited(stop());
  }

  @override
  void dispose() {
    generation++;
    WidgetsBinding.instance.removeObserver(this);
    NativeVideoSessions.release(this);
    final old = video;
    video = null;
    if (old != null) unawaited(old.dispose());
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final d = asMap(widget.item['data']), m = ModelScope.of(context);
    final original = Uri.tryParse(
      (d['canonical_video_url'] ?? widget.check?['canonical_url'] ?? '')
          .toString(),
    );
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              resourceTitle(m, widget.item),
              style: Theme.of(context).textTheme.titleMedium,
            ),
            Text(
              isDouyinVideo(widget.item) ? '来源：抖音' : '来源：本地上传',
              style: Theme.of(context).textTheme.bodySmall,
            ),
            if (checked != null)
              NativeOfficialInline(
                key: ValueKey(generation),
                check: checked!,
                onStopped: () => unawaited(stop()),
              )
            else if (video?.value.isInitialized == true) ...[
              if (!fullscreen)
                AspectRatio(
                  aspectRatio: video!.value.aspectRatio,
                  child: VideoPlayer(video!),
                ),
              VideoProgressIndicator(
                video!,
                allowScrubbing: true,
                padding: const EdgeInsets.symmetric(vertical: 8),
              ),
              controls(),
            ] else if ((d['cover_media_id'] ?? 0) > 0)
              Photo(id: d['cover_media_id'], aspectRatio: 16 / 9),
            if (error.isNotEmpty)
              Text(error, style: const TextStyle(color: Colors.red)),
            if (busy)
              const Padding(
                padding: EdgeInsets.all(12),
                child: LinearProgressIndicator(),
              ),
            Wrap(
              spacing: 12,
              children: [
                if (checked == null && video?.value.isInitialized != true)
                  FilledButton.icon(
                    key: const ValueKey('inline-video-start'),
                    onPressed: busy ? null : start,
                    icon: const Icon(Icons.play_arrow),
                    label: Text(error.isEmpty ? '播放视频' : '重试播放'),
                  ),
                if (checked != null || video != null)
                  TextButton(onPressed: stop, child: const Text('收起视频')),
                if (original != null &&
                    original.scheme == 'https' &&
                    original.userInfo.isEmpty &&
                    !original.hasPort &&
                    ['www.douyin.com', 'v.douyin.com'].contains(original.host))
                  TextButton(
                    onPressed: () => launchUrl(
                      original,
                      mode: LaunchMode.externalApplication,
                    ),
                    child: const Text('打开原视频'),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
