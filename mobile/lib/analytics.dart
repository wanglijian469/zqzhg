part of 'main.dart';

final analyticsRoutes = RouteObserver<PageRoute<dynamic>>();

/// First-party, per-server anonymous identifiers; no account/contact/URL data.
class AppAnalytics {
  final AppModel model;
  final List<Json> _queue = [];
  String _base = '', _visitor = '', _session = '';
  DateTime _active = DateTime.fromMillisecondsSinceEpoch(0);
  bool _sending = false, _disposed = false;
  Timer? _timer;
  AppAnalytics(this.model);
  String get _key => 'zqzt.analytics.${base64Url.encode(utf8.encode(_base))}';
  String _uuid() {
    final random = Random.secure();
    final bytes = List.generate(16, (_) => random.nextInt(256));
    bytes[6] = (bytes[6] & 15) | 64;
    bytes[8] = (bytes[8] & 63) | 128;
    final s = bytes.map((b) => b.toRadixString(16).padLeft(2, '0')).join();
    return '${s.substring(0, 8)}-${s.substring(8, 12)}-${s.substring(12, 16)}-${s.substring(16, 20)}-${s.substring(20)}';
  }

  void _load() {
    if (_base == model.baseUrl) return;
    _timer?.cancel();
    _base = model.baseUrl;
    _queue.clear();
    _visitor = model.prefs.getString('$_key.visitor') ?? _uuid();
    _session = model.prefs.getString('$_key.session') ?? _uuid();
    _active =
        DateTime.tryParse(model.prefs.getString('$_key.active') ?? '') ??
        DateTime.fromMillisecondsSinceEpoch(0);
    try {
      for (final entry
          in jsonDecode(model.prefs.getString('$_key.queue') ?? '[]') as List) {
        final pending = asMap(entry);
        final event = asMap(pending['event']);
        if ((pending['tries'] as int) < 3 &&
            DateTime.parse(
              event['occurred_at'],
            ).isAfter(DateTime.now().subtract(const Duration(hours: 24)))) {
          _queue.add(pending);
        }
      }
    } catch (_) {
      _queue.clear();
    }
  }

  Future<void> _persist() async {
    try {
      final key = _key;
      await model.prefs.setString('$key.visitor', _visitor);
      await model.prefs.setString('$key.session', _session);
      await model.prefs.setString('$key.active', _active.toIso8601String());
      await model.prefs.setString('$key.queue', jsonEncode(_queue));
    } catch (_) {
      /* Browsing remains available if preferences are unavailable. */
    }
  }

  void event(
    String name,
    String page, {
    String? kind,
    int? id,
    String? keyword,
  }) {
    if (!model.analyticsEnabled || model.isHallPreview) {
      return;
    }
    if (!RegExp(
      r'^(/|/browse(?:/products)?|/products/[0-9]+|/resources/[0-9]+|/v/[a-z0-9-]+)$',
    ).hasMatch(page)) {
      return;
    }
    _disposed = false;
    _load();
    final now = DateTime.now().toUtc();
    if (now.difference(_active) > const Duration(minutes: 30)) {
      _session = _uuid();
    }
    _active = now;
    var term = keyword?.trim().replaceAll(RegExp(r'\s+'), ' ');
    if (term != null &&
        RegExp(
          r'(1[3-9][0-9]{9}|[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}|(?:password|token|secret|access.?key)\s*[:=])',
          caseSensitive: false,
        ).hasMatch(term)) {
      term = '[已脱敏]';
    }
    _queue.add({
      'tries': 0,
      'event': {
        'event_id': _uuid(),
        'visitor_id': _visitor,
        'session_id': _session,
        'source': 'android',
        'event_name': name,
        'page': page,
        'occurred_at': now.toIso8601String(),
        'device': 'mobile',
        'target_kind': kind ?? '',
        'target_id': id ?? 0,
        if (term != null) 'keyword': String.fromCharCodes(term.runes.take(80)),
      },
    });
    if (_queue.length > 100) _queue.removeRange(0, _queue.length - 100);
    unawaited(_persist());
    _schedule(const Duration(seconds: 2));
  }

  void page(String page, {String? kind, int? id}) {
    event('page_view', page);
    if (kind != null && id != null) {
      event('${kind}_view', page, kind: kind, id: id);
    }
  }

  void _schedule(Duration delay) {
    if (_disposed || (_timer?.isActive ?? false)) return;
    _timer = Timer(delay, () => unawaited(flush()));
  }

  Future<void> flush() async {
    if (_sending || _queue.isEmpty) return;
    _sending = true;
    final base = _base;
    final batch = _queue.take(20).toList();
    for (final pending in batch) {
      pending['tries'] = (pending['tries'] as int) + 1;
    }
    try {
      final response = await model.client
          .post(
            Uri.parse('$base/api/v1/public/analytics/events'),
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode({'events': batch.map((p) => p['event']).toList()}),
          )
          .timeout(const Duration(seconds: 8));
      if (_base == base &&
          response.statusCode >= 200 &&
          response.statusCode < 300) {
        _queue.removeWhere(batch.contains);
      }
    } catch (_) {
      /* Finite retry; reporting never blocks business requests. */
    } finally {
      _queue.removeWhere((p) => (p['tries'] as int) >= 3);
      _sending = false;
      await _persist();
      if (_queue.isNotEmpty) _schedule(const Duration(seconds: 10));
    }
  }

  void dispose() {
    _disposed = true;
    _timer?.cancel();
  }
}

class AnalyticsScreen extends StatefulWidget {
  final String page, kind;
  final int id;
  final Widget child;
  const AnalyticsScreen({
    super.key,
    required this.page,
    required this.kind,
    required this.id,
    required this.child,
  });
  @override
  State<AnalyticsScreen> createState() => _AnalyticsScreenState();
}

class _AnalyticsScreenState extends State<AnalyticsScreen> with RouteAware {
  PageRoute<dynamic>? route;
  String identity = '';
  void view() {
    if (route?.isCurrent != true) return;
    ModelScope.of(
      context,
    ).analytics.page(widget.page, kind: widget.kind, id: widget.id);
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final next = ModalRoute.of(context);
    if (next is PageRoute<dynamic> && route != next) {
      analyticsRoutes.unsubscribe(this);
      route = next;
      analyticsRoutes.subscribe(this, next);
    }
    final key = '${ModelScope.of(context).baseUrl}:${widget.page}';
    if (key != identity) {
      identity = key;
      view();
    }
  }

  @override
  void didPopNext() => view();
  @override
  void dispose() {
    analyticsRoutes.unsubscribe(this);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AnalyticsContactScope(
    page: widget.page,
    kind: widget.kind,
    id: widget.id,
    child: widget.child,
  );
}

class AnalyticsContactScope extends InheritedWidget {
  final String page, kind;
  final int id;
  const AnalyticsContactScope({
    super.key,
    required this.page,
    required this.kind,
    required this.id,
    required super.child,
  });
  @override
  bool updateShouldNotify(AnalyticsContactScope oldWidget) =>
      page != oldWidget.page || id != oldWidget.id || kind != oldWidget.kind;
}

void analyticsContact(BuildContext context, String name) {
  final scope = context
      .dependOnInheritedWidgetOfExactType<AnalyticsContactScope>();
  if (scope != null) {
    ModelScope.of(
      context,
    ).analytics.event(name, scope.page, kind: scope.kind, id: scope.id);
  }
}
