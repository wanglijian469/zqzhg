import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'dart:ui' as ui;
import 'package:qr/qr.dart';
import 'package:video_player/video_player.dart';
import 'package:pdfrx/pdfrx.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter/foundation.dart' show kReleaseMode;
import 'package:flutter/services.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:http/http.dart' as http;
import 'package:image_picker/image_picker.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:url_launcher/url_launcher.dart';

part 'analytics.dart';
part 'service_center.dart';
part 'form_rules.dart';
part 'onboarding.dart';
part 'taxonomy.dart';
part 'industrial_ui.dart';
part 'mobile_fields.dart';
part 'merchant_experience.dart';
part 'merchant_workspace.dart';
part 'merchant_products.dart';
part 'resources_experience.dart';
part 'app_sync.dart';
part 'app_navigation.dart';
part 'favorites.dart';
part 'exhibition_home.dart';
part 'exhibition_portal.dart';
part 'browse_screen.dart';
part 'exhibit_card.dart';
part 'exhibition_halls.dart';
part 'app_upgrade.dart';
part 'native_auth_entry.dart';
part 'unified_auth.dart';
part 'password_policy.dart';
part 'mobile_upgrade.dart';
part 'procurement_workspace.dart';
part 'product_quality.dart';

typedef Json = Map<String, dynamic>;
const green = industrialBlue;
const pale = industrialSurface;
const productionApiBaseUrl = 'https://zqzhg.com';
Json asMap(dynamic value) => Map<String, dynamic>.from(value as Map);
List<Json> rows(dynamic value) => (value as List? ?? []).map(asMap).toList();
List<int> imageIDs(Json data) => (data['media_ids'] as List? ?? [])
    .map((id) => (id as num).toInt())
    .toList();

class ApiFailure implements Exception {
  final String code;
  final List<Json> fields;
  final int status;
  ApiFailure(this.code, [this.fields = const [], this.status = 0]);
}

class AppModel extends ChangeNotifier {
  String merchantVendorName = '';
  bool newlyRegistered = false;
  final Map<String, Future<Json>> clientCapabilityCache = {};
  late final MerchantDataStore merchant = MerchantDataStore(this);
  late final FavoritesStore favorites = FavoritesStore(this);
  late final AppAnalytics analytics = AppAnalytics(this);
  bool get usageAnalyticsAllowed =>
      analyticsEnabled && prefs.getBool('service.analytics_consent') == true;
  VoidCallback? openMerchant;
  VoidCallback? openHome;
  void favoritesChanged() => notifyListeners();
  final SharedPreferences prefs;
  final http.Client client;
  final bool analyticsEnabled;
  final Map<String, Json> resources;
  late String language;
  late final String baseUrl;
  late String token;
  late Json account;
  late final HallRegistry hallRegistry = HallRegistry(this);
  ExhibitionHall get activeHall => hallRegistry.active;
  List<ExhibitionHall> get availableHalls => hallRegistry.items;
  bool get showHallSelector =>
      availableHalls.where((h) => h.isOpen).length >= 2;
  bool get isHallPreview => !activeHall.isOpen;
  bool get hallsReady => hallRegistry.ready;
  bool get modernPublicApi => hallsReady && !hallRegistry.legacy;
  String? get hallError => hallRegistry.error;
  void publicChanged() => notifyListeners();
  void selectHall(ExhibitionHall hall) => hallRegistry.select(hall);
  Future<void> loadHalls({bool refresh = false}) =>
      hallRegistry.load(refresh: refresh);
  final browseMemory = <String, Json>{};
  final _taxonomies = <String, Future<dynamic>>{};
  final _categoryLabels = <String, Json>{};
  String productCategory(Json data) {
    final supplied = content(data, 'category_name');
    if (supplied.isNotEmpty) return supplied;
    final modern = (data['product_category_id'] as num? ?? 0).toInt();
    final id = modern > 0 ? modern : data['category_id'];
    final category = _categoryLabels['$baseUrl:$id'];
    return category == null ? '' : content(category, 'name');
  }

  String publicPath(String path, {ExhibitionHall? hall}) {
    final uri = Uri.parse(path);
    return uri
        .replace(
          queryParameters: {
            ...uri.queryParameters,
            'hall': (hall ?? activeHall).slug,
          },
        )
        .toString();
  }

  Future<dynamic> publicApi(String path, {ExhibitionHall? hall}) async {
    final source = baseUrl, selected = hall ?? activeHall;
    await loadHalls();
    if (source != baseUrl) throw ApiFailure('network_error');
    if (!hallsReady) throw ApiFailure(hallError ?? 'network_error');
    final actual = availableHalls
        .where((h) => h.slug == selected.slug)
        .firstOrNull;
    if (actual == null || !actual.isOpen) throw ApiFailure('not_found');
    return api(publicPath(path, hall: actual));
  }

  Future<dynamic> publicTaxonomy({
    bool refresh = false,
    String kind = 'legacy',
  }) {
    final key = '$baseUrl:${activeHall.slug}:$kind';
    if (refresh) _taxonomies.remove(key);
    final cached = _taxonomies[key];
    if (cached != null) return cached;
    final source = baseUrl;
    final request = publicApi('/public/taxonomy?category_kind=$kind').then((
      value,
    ) {
      for (final category in rows(value['categories'])) {
        _categoryLabels['$source:${category['id']}'] = category;
      }
      if (source == baseUrl) publicChanged();
      return value;
    });
    _taxonomies[key] = request;
    unawaited(
      request.then<void>(
        (_) {},
        onError: (Object e, StackTrace s) {
          if (identical(_taxonomies[key], request)) _taxonomies.remove(key);
        },
      ),
    );
    return request;
  }

  AppModel(
    this.prefs,
    this.resources, {
    http.Client? client,
    bool? analyticsEnabled,
    String? testBaseUrl,
  }) : analyticsEnabled = analyticsEnabled ?? Platform.isAndroid,
       client = client ?? http.Client() {
    unawaited(clearRetiredBoundaryCache());
    language = prefs.getString('language') ?? 'zh';
    const compiledBase = String.fromEnvironment(
      'API_BASE_URL',
      defaultValue: 'https://zqzhg.com',
    );
    assert(!kReleaseMode || testBaseUrl == null);
    baseUrl = kReleaseMode ? productionApiBaseUrl : testBaseUrl ?? compiledBase;
    final savedBase = prefs.getString('base_url');
    final migrate =
        savedBase != null && savedBase.replaceAll(RegExp(r'/$'), '') != baseUrl;
    if (!usageAnalyticsAllowed) {
      unawaited(analytics.withdraw());
    }
    token = migrate ? '' : prefs.getString('token') ?? '';
    account = migrate
        ? <String, dynamic>{}
        : asMap(jsonDecode(prefs.getString('account') ?? '{}'));
    // Keep source-scoped drafts and favorites. Foreign sessions never cross hosts.
    unawaited(prefs.setString('base_url', baseUrl));
    if (migrate) {
      unawaited(prefs.remove('token'));
      unawaited(prefs.remove('account'));
    }
  }
  Future<void> clearRetiredBoundaryCache() async {
    if (prefs.getBool('retired_boundary_cache_cleared') == true) return;
    try {
      final root = await getApplicationSupportDirectory();
      final cache = Directory('${root.path}/verified-boundaries');
      if (await cache.exists()) {
        await cache.delete(recursive: true);
      }
      await prefs.setBool('retired_boundary_cache_cleared', true);
    } catch (_) {
      /* Retry this optional cache cleanup on the next launch. */
    }
  }

  String t(String key) =>
      resources[language]?[key]?.toString() ??
      resources['zh']?[key]?.toString() ??
      key;
  String content(Json data, String field) =>
      language == 'en' && (data['${field}_en'] ?? '').toString().isNotEmpty
      ? data['${field}_en'].toString()
      : (data[field] ?? '').toString();
  String get scope =>
      '${Uri.encodeComponent(baseUrl)}.${account['id'] ?? 'guest'}';
  Future<void> toggleLanguage() async {
    language = language == 'zh' ? 'en' : 'zh';
    await prefs.setString('language', language);
    notifyListeners();
  }

  Future<dynamic> api(String path, {String method = 'GET', Json? data}) async {
    final requestToken = token, requestBase = baseUrl;
    try {
      final req = http.Request(method, Uri.parse('$baseUrl/api/v1$path'));
      req.headers['Content-Type'] = 'application/json';
      if (token.isNotEmpty) req.headers['Authorization'] = 'Bearer $token';
      if (data != null) req.body = jsonEncode(data);
      final response = await http.Response.fromStream(
        await client.send(req).timeout(const Duration(seconds: 30)),
      ).timeout(const Duration(seconds: 30));
      if (response.statusCode == 404 && path.startsWith('/auth/visitor')) {
        throw ApiFailure('favoritesUpgradeRequired');
      }
      if (response.statusCode == 404 && path == '/public/halls') {
        throw ApiFailure('not_found', const [], 404);
      }
      if (response.statusCode == 404) {
        throw ApiFailure('not_found', const [], 404);
      }
      final dynamic parsed = jsonDecode(response.body);
      if (response.statusCode >= 400) {
        final credentials = [
          '/auth/merchant',
          '/auth/visitor',
          '/auth/merchant/register',
          '/auth/visitor/register',
          '/auth/phone',
          '/auth/phone/password',
          '/auth/password',
          '/auth/phone/verify',
          '/auth/phone/password/reset',
          '/auth/phone/login',
          '/auth/phone/register',
          '/auth/phone/code',
          '/auth/phone/challenge',
          '/auth/phone/password/setup',
          '/auth/sms',
          '/auth/admin',
        ].contains(path);
        if (response.statusCode == 401 &&
            !credentials &&
            token == requestToken &&
            baseUrl == requestBase) {
          await expireSession();
        }
        throw ApiFailure(
          parsed['error']?['code'] ?? 'internal_error',
          rows(parsed['error']?['fields']),
          response.statusCode,
        );
      }
      return parsed;
    } on ApiFailure {
      rethrow;
    } catch (_) {
      throw ApiFailure('network_error');
    }
  }

  Future<int> upload(String path, {String purpose = 'unknown'}) async {
    try {
      final req = http.MultipartRequest(
        'POST',
        Uri.parse('$baseUrl/api/v1/merchant/media'),
      );
      req.headers['Authorization'] = 'Bearer $token';
      req.fields['purpose'] = purpose;
      req.files.add(await http.MultipartFile.fromPath('file', path));
      final response = await http.Response.fromStream(
        await req.send().timeout(const Duration(seconds: 60)),
      ).timeout(const Duration(seconds: 60));
      final result = jsonDecode(response.body);
      if (response.statusCode >= 400) {
        throw ApiFailure(result['error']?['code'] ?? 'invalid_image');
      }
      return (result['id'] as num).toInt();
    } on ApiFailure {
      rethrow;
    } catch (_) {
      throw ApiFailure('network_error');
    }
  }

  Future<void> expireSession() async {
    token = '';
    await prefs.remove('token');
    notifyListeners();
  }

  Future<void> signIn(Json result) async {
    merchant.clear();
    merchantVendorName = '';
    token = result['token'];
    account = asMap(result['account']);
    await prefs.setString('token', token);
    await prefs.setString('account', jsonEncode(account));
    await favorites.mergeGuest();
    notifyListeners();
    if (favorites.active) unawaited(favorites.refresh());
  }

  Future<void> signOut() async {
    final exitingToken = token, exitingScope = scope;
    if (token.isNotEmpty) {
      try {
        await api('/auth/logout', method: 'POST', data: {});
      } catch (_) {}
    }
    if (scope != exitingScope || (token.isNotEmpty && token != exitingToken)) {
      return;
    }
    token = '';
    account = {};
    merchantVendorName = '';
    merchant.clear();
    await prefs.remove('token');
    await prefs.remove('account');
    notifyListeners();
  }
}

class ModelScope extends InheritedNotifier<AppModel> {
  const ModelScope({super.key, required AppModel model, required super.child})
    : super(notifier: model);
  static AppModel of(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<ModelScope>()!.notifier!;
}

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final resources = <String, Json>{};
  for (final lang in ['zh', 'en']) {
    resources[lang] = asMap(
      jsonDecode(await rootBundle.loadString('assets/i18n/$lang.json')),
    );
  }
  final model = AppModel(await SharedPreferences.getInstance(), resources);
  runApp(ExhibitionApp(model: model));
}

class ExhibitionApp extends StatelessWidget {
  final AppModel model;
  const ExhibitionApp({super.key, required this.model});
  @override
  Widget build(BuildContext context) => ModelScope(
    model: model,
    child: AnimatedBuilder(
      animation: model,
      builder: (context, _) => MaterialApp(
        title: model.t('brand'),
        navigatorObservers: [analyticsRoutes],
        debugShowCheckedModeBanner: false,
        locale: Locale(model.language),
        supportedLocales: const [Locale('zh'), Locale('en')],
        localizationsDelegates: GlobalMaterialLocalizations.delegates,
        theme: industrialTheme(),
        home: const HomeScreen(),
      ),
    ),
  );
}

AppBar appBar(
  BuildContext context,
  String title, {
  List<Widget> extraActions = const [],
  PreferredSizeWidget? bottom,
}) {
  final m = ModelScope.of(context);
  return AppBar(
    title: Text(title),
    leading: Navigator.of(context).canPop()
        ? const BackButton()
        : m.openHome != null &&
              ModalRoute.of(context)?.isFirst == true &&
              title != m.t('brand')
        ? IconButton(
            tooltip: m.t('backHome'),
            icon: const Icon(Icons.home_outlined),
            onPressed: m.openHome,
          )
        : null,
    bottom: bottom,
    actions: [
      ...extraActions,
      TextButton(
        onPressed: m.toggleLanguage,
        child: Text(m.language == 'zh' ? 'English' : '中文'),
      ),
    ],
  );
}

void showError(BuildContext context, dynamic error) {
  final m = ModelScope.of(context);
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      content: Text(m.t(error is ApiFailure ? error.code : 'internal_error')),
    ),
  );
}

void push(BuildContext context, Widget screen) =>
    Navigator.push(context, MaterialPageRoute(builder: (_) => screen));
Widget space([double h = 16]) => SizedBox(height: h);
Widget message(BuildContext context, String text) => Container(
  width: double.infinity,
  padding: const EdgeInsets.all(16),
  margin: const EdgeInsets.symmetric(vertical: 12),
  decoration: BoxDecoration(
    color: pale,
    borderRadius: BorderRadius.circular(10),
  ),
  child: Text(text, style: const TextStyle(fontSize: 14, height: 1.6)),
);

class AsyncPane extends StatefulWidget {
  final Future<dynamic> Function() load;
  final Widget Function(dynamic) builder;
  final String identity;
  const AsyncPane({
    super.key,
    required this.load,
    required this.builder,
    required this.identity,
  });
  @override
  State<AsyncPane> createState() => _AsyncPaneState();
}

class _AsyncPaneState extends State<AsyncPane> {
  late Future<dynamic> future;
  int retryVersion = 0;
  @override
  void initState() {
    super.initState();
    future = widget.load();
  }

  @override
  void didUpdateWidget(AsyncPane old) {
    super.didUpdateWidget(old);
    if (old.identity != widget.identity) future = widget.load();
  }

  @override
  Widget build(BuildContext context) => FutureBuilder<dynamic>(
    key: ValueKey(widget.identity + retryVersion.toString()),
    future: future,
    builder: (context, snap) {
      final m = ModelScope.of(context);
      if (snap.hasError) {
        return Padding(
          padding: const EdgeInsets.all(24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                m.t(
                  snap.error is ApiFailure
                      ? (snap.error as ApiFailure).code
                      : 'network_error',
                ),
              ),
              space(),
              OutlinedButton(
                onPressed: () => setState(() {
                  retryVersion++;
                  future = widget.load();
                }),
                child: Text(m.t('retry')),
              ),
            ],
          ),
        );
      }
      if (!snap.hasData) {
        return const Padding(
          padding: EdgeInsets.all(40),
          child: Center(child: CircularProgressIndicator()),
        );
      }
      return widget.builder(snap.data);
    },
  );
}

class Photo extends StatelessWidget {
  final int id;
  final List<int> fallbackIDs;
  final bool thumb;
  final double? height;
  final double aspectRatio;
  final BoxFit fit;
  final double radius;
  final Alignment alignment;
  final Widget? loadingPlaceholder, errorPlaceholder;
  const Photo({
    super.key,
    required this.id,
    this.fallbackIDs = const [],
    this.thumb = true,
    this.height,
    this.aspectRatio = 4 / 3,
    this.fit = BoxFit.contain,
    this.radius = 10,
    this.alignment = Alignment.center,
    this.loadingPlaceholder,
    this.errorPlaceholder,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final image = Image.network(
      '${m.baseUrl}/api/v1/media/$id${thumb ? '?thumb=1' : ''}',
      headers: m.token.isEmpty ? null : {'Authorization': 'Bearer ${m.token}'},
      fit: fit,
      alignment: alignment,
      width: double.infinity,
      height: double.infinity,
      loadingBuilder: (c, child, p) => p == null
          ? child
          : loadingPlaceholder ??
                const ColoredBox(
                  color: industrialLine,
                  child: Center(
                    child: SizedBox(
                      width: 24,
                      height: 24,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                  ),
                ),
      errorBuilder: (c, e, s) => fallbackIDs.isNotEmpty
          ? Photo(
              id: fallbackIDs.first,
              fallbackIDs: fallbackIDs.skip(1).toList(),
              thumb: thumb,
              fit: fit,
              radius: 0,
              alignment: alignment,
              loadingPlaceholder: loadingPlaceholder,
              errorPlaceholder: errorPlaceholder,
            )
          : errorPlaceholder ??
                const ColoredBox(
                  color: industrialSurface,
                  child: Center(
                    child: Icon(
                      Icons.image_not_supported_outlined,
                      color: Color(0xff8ba0ba),
                    ),
                  ),
                ),
    );
    return ClipRRect(
      borderRadius: BorderRadius.circular(radius),
      child: height != null
          ? SizedBox(height: height, width: double.infinity, child: image)
          : AspectRatio(aspectRatio: aspectRatio, child: image),
    );
  }
}

class HomeScreen extends StatelessWidget {
  const HomeScreen({super.key});
  @override
  Widget build(BuildContext context) =>
      AppShell(key: ValueKey(ModelScope.of(context).baseUrl));
}

class Contact extends StatelessWidget {
  final Json data;
  const Contact({super.key, required this.data});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final phone = (data['phone'] ?? '').toString(),
        wechat = (data['wechat'] ?? '').toString();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (phone.isNotEmpty)
          FilledButton.icon(
            icon: const Icon(Icons.phone),
            onPressed: () => dialContact(context, phone),
            label: Text(phone),
          ),
        if (phone.isNotEmpty)
          TextButton.icon(
            onPressed: () => copyNativeText(context, phone),
            icon: const Icon(Icons.copy, size: 18),
            label: Text(m.t('copyPhone')),
          ),
        if (wechat.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(top: 10),
            child: OutlinedButton.icon(
              icon: const Icon(Icons.copy),
              onPressed: () async {
                analyticsContact(context, 'contact_wechat');
                await Clipboard.setData(ClipboardData(text: wechat));
                if (context.mounted) {
                  ScaffoldMessenger.of(
                    context,
                  ).showSnackBar(SnackBar(content: Text(m.t('copied'))));
                }
              },
              label: Text('${m.t('copy')} · $wechat'),
            ),
          ),
      ],
    );
  }
}

Widget dataDetails(
  BuildContext context,
  Json data,
  String kind, {
  List<Json> localPhotos = const [],
}) {
  final m = ModelScope.of(context);
  final ids = imageIDs(data);
  final description = m.content(data, 'description');
  return Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Text(
        m.content(data, 'name'),
        style: const TextStyle(fontSize: 28, fontWeight: FontWeight.bold),
      ),
      if ((data['category_id'] ?? 0) != 0)
        AsyncPane(
          identity: '${m.baseUrl}:category:${data['category_id']}',
          load: () => m.api(
            m.token.isEmpty ? '/public/taxonomy' : '/merchant/taxonomy',
          ),
          builder: (result) {
            final categories = rows(
              result['categories'],
            ).where((c) => c['id'] == data['category_id']);
            if (categories.isEmpty) return const SizedBox.shrink();
            return Padding(
              padding: const EdgeInsets.only(top: 10),
              child: Text(
                '${m.t('category')}: ${m.content(categories.first, 'name')}',
              ),
            );
          },
        ),
      if ((data['region_name'] ?? '').toString().isNotEmpty)
        Padding(
          padding: const EdgeInsets.only(top: 8),
          child: Text(data['region_name']),
        ),
      if ((data['main_products'] ?? '').toString().isNotEmpty)
        Padding(
          padding: const EdgeInsets.only(top: 12),
          child: Text('${m.t('mainProducts')}: ${data['main_products']}'),
        ),
      NativeTaxonomySummary(data: data, kind: kind),
      if (ids.isNotEmpty)
        SizedBox(
          height: 240,
          child: PageView(
            children: ids
                .map(
                  (id) => Padding(
                    padding: const EdgeInsets.symmetric(
                      vertical: 16,
                      horizontal: 4,
                    ),
                    child: InkWell(
                      onTap: () => push(
                        context,
                        PhotoViewer(ids: ids, initial: ids.indexOf(id)),
                      ),
                      child: Photo(id: id, thumb: false),
                    ),
                  ),
                )
                .toList(),
          ),
        ),
      ...localPhotos
          .where((p) => p['id'] == null)
          .map(
            (p) => Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Image.file(
                File(p['path']),
                height: 200,
                fit: BoxFit.cover,
                errorBuilder: (_, _, _) => Text(m.t('photoRetry')),
              ),
            ),
          ),
      if (description.isNotEmpty) ...[
        space(),
        Text(
          m.t(kind == 'vendor' ? 'about' : 'description'),
          style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
        ),
        space(8),
        Text(description, style: const TextStyle(height: 1.8)),
      ],
      if ((data['parameters'] ?? '').toString().isNotEmpty) ...[
        space(),
        Text(
          m.t('parameters'),
          style: const TextStyle(fontSize: 20, fontWeight: FontWeight.bold),
        ),
        space(8),
        Text(data['parameters'], style: const TextStyle(height: 1.8)),
      ],
      if ((data['address'] ?? '').toString().isNotEmpty)
        Padding(
          padding: const EdgeInsets.only(top: 16),
          child: Text(data['address']),
        ),
    ],
  );
}

class PhotoViewer extends StatelessWidget {
  final List<int> ids;
  final int initial;
  const PhotoViewer({super.key, required this.ids, required this.initial});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: appBar(context, m.t('gallery')),
      body: PageView(
        controller: PageController(initialPage: initial),
        children: ids
            .map(
              (id) => InteractiveViewer(
                minScale: 1,
                maxScale: 5,
                child: Center(child: Photo(id: id, thumb: false)),
              ),
            )
            .toList(),
      ),
    );
  }
}

class DetailScreen extends StatefulWidget {
  final int id;
  final String kind;
  const DetailScreen({super.key, required this.id, required this.kind});
  @override
  State<DetailScreen> createState() => _DetailScreenState();
}

class _DetailScreenState extends State<DetailScreen> {
  int page = 1;
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    if (widget.kind == 'vendor') {
      return CompanyShowroomScreen(slug: 'v-${widget.id}');
    }
    return Scaffold(
      appBar: appBar(
        context,
        m.t(widget.kind == 'vendor' ? 'myBooth' : 'description'),
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(20),
        child: AsyncPane(
          identity: '${m.baseUrl}:${widget.id}:$page',
          load: () => m.api(
            '/public/${widget.kind == 'vendor' ? 'vendors' : 'products'}/${widget.id}?page=$page',
          ),
          builder: (result) {
            final item = asMap(result['item']);
            final data = asMap(item['data']);
            return AnalyticsScreen(
              page: '/products/${widget.id}',
              kind: 'product',
              id: widget.id,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (widget.kind == 'vendor')
                    OutlinedButton.icon(
                      onPressed: () => push(
                        context,
                        ShowroomPromotion(
                          slug: item['showroom_slug'] ?? 'v-${item['id']}',
                        ),
                      ),
                      icon: const Icon(Icons.share_outlined),
                      label: Text(m.t('shareShowroom')),
                    ),
                  Align(
                    alignment: Alignment.centerRight,
                    child: Wrap(
                      crossAxisAlignment: WrapCrossAlignment.center,
                      children: [
                        IconButton(
                          tooltip: m.t('share'),
                          icon: const Icon(Icons.share_outlined),
                          onPressed: () async {
                            final url = '${m.baseUrl}/products/${widget.id}';
                            try {
                              await sharingBridge.invokeMethod('shareText', {
                                'text': url,
                                'title': m.content(data, 'name'),
                              });
                            } catch (_) {
                              if (context.mounted) {
                                await copyNativeText(context, url);
                              }
                            }
                          },
                        ),
                        FavoriteButton(item: {...item, 'kind': widget.kind}),
                      ],
                    ),
                  ),
                  dataDetails(context, data, widget.kind),
                  if (widget.kind == 'product') ...[space(12)],
                  if (widget.kind == 'product')
                    ExpansionTile(
                      title: Text(
                        '${m.t('shortVideos')} / ${m.t('electronicCatalogs')}',
                      ),
                      children: [NativeResourceList(productID: widget.id)],
                    ),
                  space(24),
                  Contact(
                    data: widget.kind == 'vendor'
                        ? data
                        : asMap(asMap(result['vendor'] ?? {})['data'] ?? {}),
                  ),
                  if (result['vendor'] != null) ...[
                    space(),
                    ListTile(
                      leading: const Icon(Icons.storefront),
                      title: Text(
                        m.content(asMap(result['vendor']['data']), 'name'),
                      ),
                      trailing: const Icon(Icons.arrow_forward),
                      onTap: () => push(
                        context,
                        DetailScreen(
                          id: result['vendor']['id'],
                          kind: 'vendor',
                        ),
                      ),
                    ),
                  ],
                  if (result['products'] != null) ...[
                    space(24),
                    Text(
                      m.t('products'),
                      style: const TextStyle(
                        fontSize: 22,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                    ...rows(
                      result['products']['items'],
                    ).map((p) => ItemTile(item: p)),
                    if (rows(result['products']['items']).isEmpty)
                      message(context, m.t('emptyProducts')),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        TextButton(
                          onPressed: page > 1
                              ? () => setState(() => page--)
                              : null,
                          child: Text(m.t('previous')),
                        ),
                        Text('$page'),
                        TextButton(
                          onPressed: page * 20 < result['products']['total']
                              ? () => setState(() => page++)
                              : null,
                          child: Text(m.t('next')),
                        ),
                      ],
                    ),
                  ],
                ],
              ),
            );
          },
        ),
      ),
    );
  }
}

class MerchantScreen extends StatelessWidget {
  const MerchantScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    if (isMerchant(m)) return MerchantWorkspace(key: ValueKey(m.scope));
    return Scaffold(
      appBar: appBar(context, m.t('merchantCenter')),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 480),
          child: ListView(
            padding: const EdgeInsets.all(24),
            children: [
              Text(
                syncText(context, '厂商入驻', 'Manufacturer entry'),
                style: const TextStyle(
                  fontSize: 28,
                  fontWeight: FontWeight.bold,
                ),
              ),
              space(),
              Text(
                syncText(
                  context,
                  '注册账号 → 工作台完善资料 → 提交审核',
                  'Create account → Complete profile → Submit for review',
                ),
              ),
              space(24),
              FilledButton(
                onPressed: () => authenticateManufacturerEntry(context),
                child: Text(m.t('join')),
              ),
              space(),
              OutlinedButton(
                onPressed: () =>
                    authenticateManufacturerEntry(context, register: false),
                child: Text(syncText(context, '已有公司账号登录', 'Company sign-in')),
              ),
              if (m.account['role'] == 'visitor')
                message(context, m.t('visitorMerchantHelp')),
            ],
          ),
        ),
      ),
    );
  }
}

class HistoryScreen extends StatelessWidget {
  final int id;
  const HistoryScreen({super.key, required this.id});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Scaffold(
      appBar: appBar(context, m.t('history')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(20),
        child: AsyncPane(
          identity: '${m.scope}:$id',
          load: () => m.api('/merchant/contents/$id/history'),
          builder: (result) => Column(
            children: rows(result)
                .map(
                  (log) => ListTile(
                    title: Text(
                      m.t(
                        log['action'] == 'submit'
                            ? 'submitAction'
                            : log['action'],
                      ),
                    ),
                    subtitle: Text(
                      '${log['created_at']}\n${log['note'] == 'offline' ? m.t('offline') : log['note']}',
                    ),
                  ),
                )
                .toList(),
          ),
        ),
      ),
    );
  }
}

class ReviewStatusScreen extends StatefulWidget {
  const ReviewStatusScreen({super.key});
  @override
  State<ReviewStatusScreen> createState() => _ReviewStatusScreenState();
}

class _ReviewStatusScreenState extends State<ReviewStatusScreen> {
  String status = 'pending';
  int page = 1;
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Scaffold(
      appBar: appBar(context, m.t('reviewStatus')),
      body: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          DropdownButtonFormField<String>(
            initialValue: status,
            items: ['pending', 'rejected', 'approved', 'draft']
                .map((s) => DropdownMenuItem(value: s, child: Text(m.t(s))))
                .toList(),
            onChanged: (s) => setState(() {
              status = s!;
              page = 1;
            }),
          ),
          space(),
          AsyncPane(
            identity: '${m.scope}:$status:$page',
            load: () => m.api('/merchant/contents?status=$status&page=$page'),
            builder: (result) => Column(
              children: [
                if (rows(result['items']).isEmpty)
                  message(context, m.t('noHistory')),
                ...rows(result['items']).map(
                  (item) => ListTile(
                    title: Text(m.content(asMap(item['data']), 'name')),
                    subtitle: Text(
                      '${m.t(item['status'])}\n${item['review_note'] == 'offline' ? m.t('offline') : item['review_note']}',
                    ),
                    trailing: const Icon(Icons.chevron_right),
                    onTap: () => push(
                      context,
                      EditorScreen(kind: item['kind'], item: item),
                    ),
                  ),
                ),
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    TextButton(
                      onPressed: page > 1 ? () => setState(() => page--) : null,
                      child: Text(m.t('previous')),
                    ),
                    Text('$page'),
                    TextButton(
                      onPressed: page * 20 < result['total']
                          ? () => setState(() => page++)
                          : null,
                      child: Text(m.t('next')),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class EditorScreen extends StatefulWidget {
  final String kind;
  final Json? item;
  final String? initialSection;
  const EditorScreen({
    super.key,
    required this.kind,
    this.item,
    this.initialSection,
  });
  @override
  State<EditorScreen> createState() => _EditorScreenState();
}

class _EditorScreenState extends State<EditorScreen> {
  final fields = <String, TextEditingController>{};
  Json data = {};
  Json? current;
  List<Json> photos = [];
  bool initialized = false, busy = false;
  Timer? timer;
  String? draftKey;
  int editorHallID = 1;
  int productStep = 0;
  bool guestAdoptionApproved = false;
  bool operationLock = false,
      dirty = false,
      leaving = false,
      creationUncertain = false;
  String? guestDraftKey;
  List<String> galleryOrder = [];
  final fieldFocus = <String, FocusNode>{};
  final expandedSections = <String>{};
  final groupControllers = <String, ExpansibleController>{};
  int taxonomyVersion = 0;
  Map<String, Json> issues = {};
  final fieldKeys = <String, GlobalKey>{};
  int brandProgress = 0;
  int ownerID = 0;
  String sourceEndpoint = '';
  bool retired = false;
  final sectionKeys = <String, GlobalKey>{
    for (final section in [
      'basic',
      'brand',
      'profile',
      'contact',
      'credentials',
      'gallery',
    ])
      section: GlobalKey(),
  };
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final m = ModelScope.of(context);
    if (initialized) {
      if (!retired &&
          (m.baseUrl != sourceEndpoint ||
              (ownerID != 0 &&
                  m.token.isNotEmpty &&
                  m.account['id'] != ownerID))) {
        retired = true;
        timer?.cancel();
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) {
            returnToMerchant(context);
          }
        });
      }
      return;
    }
    initialized = true;
    ownerID = m.account['id'] ?? 0;
    sourceEndpoint = m.baseUrl;
    current = widget.item;
    editorHallID = (current?['hall_id'] as num?)?.toInt() ?? m.activeHall.id;
    draftKey = 'draft.${m.scope}.${widget.item?['id'] ?? widget.kind}';
    data = widget.item == null
        ? {
            'name': '',
            'name_en': '',
            'main_products': '',
            'region_name': '',
            'region_id': 0,
            'category_id': 0,
            'keywords': '',
            'description': '',
            'description_en': '',
            'phone': '',
            'wechat': '',
            'address': '',
            'parameters': '',
            'media_ids': <int>[],
          }
        : asMap(widget.item!['data']);
    var raw = m.prefs.getString(draftKey!);
    if (widget.kind == 'vendor' && widget.item == null && m.newlyRegistered) {
      final guest = 'draft.${Uri.encodeComponent(m.baseUrl)}.guest.vendor';
      if (raw == null && m.prefs.containsKey(guest)) {
        raw = m.prefs.getString(guest);
        guestDraftKey = guest;
      }
      m.newlyRegistered = false;
    }
    if (widget.kind == 'product') {
      final identity = m.scope;
      unawaited(
        m.merchant.vendor().then<void>((value) {
          if (!mounted || m.scope != identity) return;
          final items = rows(value['items']);
          if (items.isNotEmpty) {
            editorHallID =
                (items.first['hall_id'] as num?)?.toInt() ?? editorHallID;
            m.merchantVendorName = m.content(
              asMap(items.first['data']),
              'name',
            );
            setState(() {});
          }
        }, onError: (Object e, StackTrace stack) {}),
      );
    }
    if (raw != null) {
      try {
        final local = asMap(jsonDecode(raw));
        if (local['revision_id'] == (current?['revision_id'] ?? 0) ||
            current == null && local['current'] is Map) {
          if (current == null && local['current'] is Map) {
            current = asMap(local['current']);
          }
          productStep = (local['step'] as int? ?? 0).clamp(0, 2);
          creationUncertain = local['creation_uncertain'] == true;
          galleryOrder = List<String>.from(local['gallery_order'] ?? []);
          data = asMap(local['data']);
          photos = rows(local['photos']);
        }
      } catch (_) {}
    }
    if (widget.kind == 'vendor') {
      final p = asMap(data['company_profile'] ?? {});
      if (p['credentials'] != null) {
        data['company_profile'] = {
          ...p,
          'credentials': rows(p['credentials'])
              .asMap()
              .entries
              .map(
                (e) => <String, dynamic>{
                  ...e.value,
                  '_key':
                      e.value['_key'] ??
                      'existing-${e.key}-${e.value['media_id']}',
                },
              )
              .toList(),
        };
      }
    }
    for (final key in [
      'name',
      'name_en',
      'main_products',
      'region_name',
      'keywords',
      'description',
      'description_en',
      'phone',
      'wechat',
      'address',
      'parameters',
    ]) {
      fieldFocus[key] = FocusNode()
        ..addListener(() {
          if (mounted) setState(() {});
        });
      fields[key] = TextEditingController(text: (data[key] ?? '').toString());
      fields[key]!.addListener(() {
        data[key] = fields[key]!.text;
        if (key == 'region_name') data['region_id'] = 0;
        scheduleSave();
        if (mounted) setState(() => issues.remove(key));
      });
    }
    if (widget.initialSection != null) {
      productStep =
          [
            'brand',
            'profile',
            'credentials',
            'gallery',
          ].contains(widget.initialSection)
          ? 1
          : widget.initialSection == 'review'
          ? 2
          : 0;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        final target = sectionKeys[widget.initialSection]?.currentContext;
        if (target != null) Scrollable.ensureVisible(target, alignment: .05);
      });
    }
    if (widget.initialSection != null) {
      expandedSections.addAll(['profile', 'credentials', 'more']);
    }
    unawaited(recoverCamera());
  }

  Future<void> recoverCamera() async {
    if (!Platform.isAndroid) return;
    try {
      final response = await ImagePicker().retrieveLostData();
      if (response.files != null) {
        for (final f in response.files!) {
          await retainPhoto(f);
        }
      }
    } catch (e) {
      if (mounted) showError(context, e);
    }
  }

  void scheduleSave() {
    dirty = true;
    timer?.cancel();
    timer = Timer(
      const Duration(milliseconds: 250),
      () => unawaited(persist()),
    );
  }

  Future<void> persist() async {
    if (!mounted || draftKey == null || retired) return;
    final m = ModelScope.of(context);
    if (m.token.isNotEmpty &&
        ownerID == 0 &&
        guestAdoptionApproved &&
        draftKey!.contains('.guest.')) {
      ownerID = m.account['id'];
      final previous = draftKey!;
      draftKey = 'draft.${m.scope}.${current?['id'] ?? widget.kind}';
      await m.prefs.setString(
        draftKey!,
        jsonEncode({
          'data': data,
          'photos': photos,
          'revision_id': current?['revision_id'] ?? 0,
          'step': productStep,
          'current': current,
          'gallery_order': galleryOrder,
          'creation_uncertain': creationUncertain,
        }),
      );
      await m.prefs.remove(previous);
    }
    await m.prefs.setString(
      draftKey!,
      jsonEncode({
        'data': data,
        'photos': photos,
        'revision_id': current?['revision_id'] ?? 0,
        'step': productStep,
        'current': current,
        'gallery_order': galleryOrder,
        'creation_uncertain': creationUncertain,
      }),
    );
    if (widget.item == null && current != null) {
      await m.prefs.setString(
        'draft.${m.scope}.${widget.kind}',
        jsonEncode({
          'data': data,
          'photos': photos,
          'revision_id': current?['revision_id'] ?? 0,
          'step': productStep,
          'current': current,
          'gallery_order': galleryOrder,
          'creation_uncertain': creationUncertain,
        }),
      );
    }
  }

  @override
  void dispose() {
    timer?.cancel();
    for (final controller in groupControllers.values) {
      controller.dispose();
    }
    for (final node in fieldFocus.values) {
      node.dispose();
    }
    for (final field in fields.values) {
      field.dispose();
    }
    super.dispose();
  }

  Future<void> retainPhoto(
    XFile file, {
    String? slot,
    String? replacement,
  }) async {
    if (slot == null &&
        replacement == null &&
        imageIDs(data).length +
                photos
                    .where((p) => p['id'] == null && p['slot'] == null)
                    .length >=
            12) {
      throw ApiFailure('invalid_image');
    }
    if (await file.length() > 10 * 1024 * 1024) {
      throw ApiFailure('invalid_image');
    }
    final dir = await getApplicationDocumentsDirectory();
    final target =
        '${dir.path}/draft-${DateTime.now().microsecondsSinceEpoch}-${file.name.replaceAll(RegExp(r'[^a-zA-Z0-9._-]'), '_')}';
    await File(file.path).copy(target);
    if (!mounted) return;
    setState(() {
      if (slot != null) photos.removeWhere((p) => p['slot'] == slot);
      photos.add({'path': target, 'slot': ?slot});
      if (slot == null) galleryOrder.add('p:$target');
      dirty = true;
    });
    await persist();
  }

  Future<void> pick(ImageSource source) async {
    try {
      final picker = ImagePicker();
      if (source == ImageSource.camera) {
        final photo = await picker.pickImage(source: source, imageQuality: 90);
        if (photo != null) await retainPhoto(photo);
      } else {
        for (final photo in await picker.pickMultiImage(imageQuality: 90)) {
          await retainPhoto(photo);
        }
      }
    } catch (e) {
      if (mounted) showError(context, e);
    }
  }

  Future<void> uploadPendingPhotos({bool brandOnly = false}) async {
    final m = ModelScope.of(context);
    for (final photo in photos.where(
      (p) => p['id'] == null && (!brandOnly || p['slot'] != null),
    )) {
      final request = ProgressMultipart(
        'POST',
        Uri.parse('${m.baseUrl}/api/v1/merchant/media'),
        (sent, total) {
          if (mounted) {
            setState(() => brandProgress = (sent / total * 100).floor());
          }
        },
      );
      request.headers['Authorization'] = 'Bearer ${m.token}';
      request.files.add(
        await http.MultipartFile.fromPath('file', photo['path']),
      );
      request.fields['purpose'] = photo['slot'] == null
          ? (widget.kind == 'product' ? 'product_image' : 'company_gallery')
          : photo['slot'] == 'logo_media_id'
          ? 'company_logo'
          : photo['slot'] == 'cover_media_id'
          ? 'company_cover'
          : 'credential';
      photo.remove('error');
      http.Response response;
      try {
        response = await http.Response.fromStream(
          await m.client.send(request),
        ).timeout(const Duration(minutes: 3));
      } catch (e) {
        photo['error'] = 'network_error';
        await persist();
        rethrow;
      }
      final payload = asMap(jsonDecode(response.body));
      if (response.statusCode >= 400) {
        photo['error'] = payload['error']['code'];
        await persist();
        if (response.statusCode == 401) await m.expireSession();
        throw ApiFailure(payload['error']['code']);
      }
      final mid = payload['id'];
      photo['id'] = mid;
      if (photo['slot'] == null) {
        final key = 'p:${photo['path']}';
        final position = galleryOrder.indexOf(key);
        if (position >= 0) galleryOrder[position] = 'm:$mid';
        data['media_ids'] = [...imageIDs(data), mid];
        galleryChanged();
      } else if (photo['slot'].toString().startsWith('credential:')) {
        final p = asMap(data['company_profile'] ?? {});
        final key = photo['slot'].toString().substring('credential:'.length);
        data['company_profile'] = {
          ...p,
          'credentials': rows(
            p['credentials'],
          ).map((r) => r['_key'] == key ? {...r, 'media_id': mid} : r).toList(),
        };
      } else {
        data['company_profile'] = {
          ...asMap(data['company_profile'] ?? {}),
          photo['slot']: mid,
        };
      }
      await persist();
      if (mounted) setState(() {});
    }
  }

  Future<void> pickBrand(String slot, XFile file) async {
    try {
      await retainPhoto(file, slot: slot);
      await retryBrand();
    } catch (e) {
      if (mounted) showError(context, e);
    }
  }

  Future<void> retryBrand() async {
    final m = ModelScope.of(context);
    if (!mounted) return;
    if (m.token.isEmpty) {
      final ok = await Navigator.push<bool>(
        context,
        MaterialPageRoute(
          builder: (_) => LoginScreen(
            onRegistered: () => guestAdoptionApproved = true,
            register: ownerID == 0,
            initialName: ownerID == 0 ? (data['name'] ?? '').toString() : '',
          ),
        ),
      );
      if (ok != true ||
          !mounted ||
          retired ||
          (ownerID != 0 && m.account['id'] != ownerID)) {
        return;
      }
    }
    if (!await approveGuestAdoption() || !mounted) return;
    await adoptGuestOwnership();
    if (!mounted) return;
    setState(() => busy = true);
    try {
      await uploadPendingPhotos(brandOnly: true);
    } catch (e) {
      if (mounted) showError(context, e);
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Future<void> save({bool submit = false}) async {
    if (operationLock || busy || creationUncertain) return;
    operationLock = true;
    try {
      await saveInternal(submit: submit);
    } finally {
      operationLock = false;
    }
  }

  Future<void> saveInternal({bool submit = false}) async {
    final m = ModelScope.of(context);
    timer?.cancel();
    await persist();
    if (!mounted) return;
    if (retired) return;
    if (widget.kind == 'product' &&
        (data['category_id'] ?? 0) == 0 &&
        (data['keywords'] ?? '').toString().trim().isEmpty) {
      try {
        final capabilities = await m.clientCapabilities();
        if (capabilities['product_name_only'] != true) {
          if (mounted) showError(context, ApiFailure('mobileUpgradeRequired'));
          return;
        }
      } catch (error) {
        if (mounted) showError(context, error);
        return;
      }
    }
    final errors = validateFormData(
      data,
      widget.kind,
      pendingCredentials: photos
          .where((p) => p['id'] == null && p['slot'] != null)
          .map((p) => p['slot'].toString())
          .toSet(),
    );
    setState(() => issues = errors);
    if (errors.isNotEmpty) {
      revealIssues();
      return;
    }
    if (!mounted) return;
    if (m.token.isEmpty) {
      bool registeredDuringAuth = false;
      final ok = await Navigator.push<bool>(
        context,
        MaterialPageRoute(
          builder: (_) => LoginScreen(
            onRegistered: () => registeredDuringAuth = true,
            register: ownerID == 0,
            initialName: ownerID == 0 ? (data['name'] ?? '').toString() : '',
          ),
        ),
      );
      if (ok != true ||
          !mounted ||
          retired ||
          (ownerID != 0 && m.account['id'] != ownerID)) {
        return;
      }
      if (registeredDuringAuth) guestAdoptionApproved = true;
      if (!await approveGuestAdoption() || !mounted) return;
      final oldKey = draftKey;
      final returningOwner = ownerID != 0;
      ownerID = m.account['id'];
      draftKey = 'draft.${m.scope}.${current?['id'] ?? widget.kind}';
      await persist();
      if (oldKey != draftKey && oldKey != 'draft.${m.scope}.${widget.kind}') {
        await m.prefs.remove(oldKey!);
      }
      setState(() => taxonomyVersion++);
      if (returningOwner) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(
              content: Text(
                syncText(
                  context,
                  '登录成功，资料已保留，请再次点击保存或提交。',
                  'Signed in. Your edits are retained; retry save or submit.',
                ),
              ),
            ),
          );
        }
        return;
      }
    }
    if (!mounted) return;
    if (!await approveGuestAdoption() || !mounted) return;
    await adoptGuestOwnership();
    if (!mounted) return;
    setState(() => busy = true);
    final firstCreation = current == null;
    try {
      if (widget.kind == 'vendor' && current == null) {
        final existing = rows(
          (await m.api('/merchant/contents?kind=vendor&page_size=1'))['items'],
        );
        if (existing.isNotEmpty) {
          if (!mounted) return;
          await showDialog<void>(
            context: context,
            builder: (context) => AlertDialog(
              title: Text(m.language == 'zh' ? '账号已有展厅' : 'Existing booth'),
              content: Text(
                m.language == 'zh'
                    ? '当前输入保留在本机，未覆盖云端资料。请编辑原展厅。'
                    : 'Your input is retained locally. Cloud data was not overwritten.',
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(context),
                  child: Text(m.t('cancel')),
                ),
                FilledButton(
                  onPressed: () {
                    Navigator.pop(context);
                    push(
                      context,
                      EditorScreen(kind: 'vendor', item: existing.first),
                    );
                  },
                  child: Text(m.t('edit')),
                ),
              ],
            ),
          );
          return;
        }
      }
      await uploadPendingPhotos();
      if (!mounted || retired) return;
      final saved = asMap(
        await m.api(
          current == null
              ? '/merchant/contents'
              : '/merchant/contents/${current!['id']}/draft',
          method: current == null ? 'POST' : 'PUT',
          data: current == null
              ? {'kind': widget.kind, 'hall_id': editorHallID, 'data': data}
              : {'revision_id': current!['revision_id'], 'data': data},
        ),
      );
      final normalized = credentialEditorData(asMap(saved['data']));
      final previousIDs = (data['media_ids'] as List? ?? []).toList();
      final nextIDs = normalized['media_ids'] as List? ?? [];
      for (var i = 0; i < previousIDs.length && i < nextIDs.length; i++) {
        final position = galleryOrder.indexOf('m:${previousIDs[i]}');
        if (position >= 0) galleryOrder[position] = 'm:${nextIDs[i]}';
      }
      current = saved;
      data = normalized;
      final oldKey = draftKey;
      draftKey = 'draft.${m.scope}.${saved['id']}';
      await persist();
      if (oldKey != draftKey && oldKey != 'draft.${m.scope}.${widget.kind}') {
        await m.prefs.remove(oldKey!);
      }
      dirty = false;
      if (guestDraftKey != null) {
        await m.prefs.remove(guestDraftKey!);
        guestDraftKey = null;
      }
      if (submit) {
        await m.api(
          '/merchant/contents/${saved['id']}/submit',
          method: 'POST',
          data: {'revision_id': saved['revision_id']},
        );
        await m.prefs.remove(draftKey!);
        if (widget.item == null) {
          await m.prefs.remove('draft.${m.scope}.${widget.kind}');
        }
      }
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(m.t(submit ? 'pending' : 'saved'))),
        );
        m.merchant.refresh();
        if (submit && widget.kind == 'vendor') {
          returnToMerchant(context);
        } else if (submit) {
          Navigator.pop(context, true);
        }
      }
    } catch (e) {
      if (firstCreation &&
          current == null &&
          e is ApiFailure &&
          e.code == 'network_error') {
        creationUncertain = true;
        await persist();
      }
      if (e is ApiFailure && e.code == 'unauthorized') {
        await m.expireSession();
      }
      if (mounted && e is ApiFailure && e.fields.isNotEmpty) {
        setState(() => issues = {for (final f in e.fields) f['field']: f});
        revealIssues();
      } else if (mounted) {
        showError(context, e);
      }
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  List<Json> galleryEntries() {
    final all = <Json>[
      for (final id in imageIDs(data)) {'id': id, 'key': 'm:$id'},
      for (final p in photos.where((p) => p['id'] == null && p['slot'] == null))
        {...p, 'key': 'p:${p['path']}'},
    ];
    for (final item in all) {
      if (!galleryOrder.contains(item['key'])) galleryOrder.add(item['key']);
    }
    galleryOrder.removeWhere((key) => !all.any((item) => item['key'] == key));
    all.sort(
      (a, b) => galleryOrder
          .indexOf(a['key'])
          .compareTo(galleryOrder.indexOf(b['key'])),
    );
    return all;
  }

  void galleryChanged() {
    data['media_ids'] = galleryEntries()
        .where((p) => p['id'] != null)
        .map((p) => p['id'])
        .toList();
    scheduleSave();
  }

  void moveImage(Json item, int offset) {
    setState(() {
      final index = galleryOrder.indexOf(item['key']), target = index + offset;
      if (target < 0 || target >= galleryOrder.length) return;
      galleryOrder.removeAt(index);
      galleryOrder.insert(target, item['key']);
      galleryChanged();
    });
  }

  void removeImage(Json item) {
    setState(() {
      galleryOrder.remove(item['key']);
      if (item['id'] != null) {
        data['media_ids'] = imageIDs(
          data,
        ).where((id) => id != item['id']).toList();
      }
      photos.removeWhere(
        (p) => item['id'] != null
            ? p['id'] == item['id']
            : p['path'] == item['path'],
      );
      final profile = asMap(data['company_profile'] ?? {});
      if (profile['gallery_annotations'] != null) {
        data['company_profile'] = {
          ...profile,
          'gallery_annotations': rows(
            profile['gallery_annotations'],
          ).where((p) => p['media_id'] != item['id']).toList(),
        };
      }
      galleryChanged();
    });
  }

  Future<void> replaceImage(Json item) async {
    try {
      final file = await ImagePicker().pickImage(
        source: ImageSource.gallery,
        imageQuality: 90,
      );
      if (file == null) return;
      final previous = galleryOrder.indexOf(item['key']);
      await retainPhoto(file, replacement: item['key']);
      if (!mounted) return;
      setState(() {
        removeImage(item);
        galleryOrder.remove('p:${photos.last['path']}');
        galleryOrder.insert(
          previous.clamp(0, galleryOrder.length),
          'p:${photos.last['path']}',
        );
        galleryChanged();
      });
    } catch (e) {
      if (mounted) showError(context, e);
    }
  }

  Future<void> retryGallery() async {
    if (busy || operationLock) return;
    if (!await nativeSessionReady(context, ownerID, sourceEndpoint) ||
        !mounted) {
      return;
    }
    if (!await approveGuestAdoption() || !mounted) return;
    await adoptGuestOwnership();
    if (!mounted) return;
    setState(() => busy = true);
    try {
      await uploadPendingPhotos();
    } catch (e) {
      if (mounted) showError(context, e);
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  void revealIssues() {
    if (issues.isEmpty) return;
    final key = issues.keys.first;
    setState(() {
      if (key.startsWith('company_profile.credentials') ||
          key.startsWith('company_profile.gallery_annotations')) {
        expandedSections.add('credentials');
      } else if (key.startsWith('company_profile.')) {
        expandedSections.add('profile');
      } else if (key.endsWith('_en')) {
        expandedSections.add('english');
      } else if ([
        'category_id',
        'product_category_id',
        'brand',
        'model',
        'applicable_brands',
        'applicable_models',
        'keywords',
        'parameters',
        'address',
      ].contains(key)) {
        expandedSections.add('more');
      }
      productStep = key == 'media_ids' || key.startsWith('company_profile')
          ? 1
          : 0;
    });
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      for (final id in expandedSections) {
        groupControllers[id]?.expand();
      }
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        final target =
            fieldKeys[key]?.currentContext ??
            sectionKeys[key.startsWith('company_profile') ? 'profile' : 'basic']
                ?.currentContext;
        if (target != null) Scrollable.ensureVisible(target, alignment: .15);
        fieldFocus[key]?.requestFocus();
      });
      WidgetsBinding.instance.scheduleFrame();
    });
  }

  Widget field(
    BuildContext context,
    String key,
    String label, {
    bool multiline = false,
  }) {
    final m = ModelScope.of(context),
        rule = asMap(formRules[key] ?? {}),
        value = fields[key]?.text ?? '',
        node = fieldFocus[key]!;
    final count = rule['unit'] == 'bytes'
        ? utf8.encode(value).length
        : value.runes.length;
    final limit = rule['max'] as num?;
    return Padding(
      key: fieldKeys.putIfAbsent(key, () => GlobalKey()),
      padding: const EdgeInsets.only(bottom: 16),
      child: NativeFieldLabel(
        label: m.t(label),
        child: TextField(
          controller: fields[key],
          focusNode: node,
          maxLines: multiline ? 4 : 1,
          enabled: !busy && current?['status'] != 'pending',
          decoration: InputDecoration(
            hintText: rule[m.language == 'zh' ? 'sample_zh' : 'sample_en'],
            hintStyle: const TextStyle(
              color: Color(0xff9aa8b8),
              fontStyle: FontStyle.italic,
            ),
            helperText: node.hasFocus || limit != null && count > limit * .85
                ? '$count / ${limit ?? '—'} ${rule['unit'] == 'bytes' ? (m.language == 'zh' ? '字节' : 'bytes') : (m.language == 'zh' ? '字' : 'characters')}'
                : null,
            errorText: issues[key] == null
                ? null
                : fieldIssueText(m, issues[key]!),
            errorMaxLines: 3,
          ),
          keyboardType: key == 'phone'
              ? TextInputType.phone
              : multiline
              ? TextInputType.multiline
              : TextInputType.text,
        ),
      ),
    );
  }

  Widget group(String id, String zh, String en, List<Widget> children) {
    return ExpansionTile(
      key: ValueKey(id),
      controller: groupControllers.putIfAbsent(
        id,
        () => ExpansibleController(),
      ),
      initiallyExpanded: expandedSections.contains(id),
      maintainState: true,
      tilePadding: EdgeInsets.zero,
      title: Text(syncText(context, zh, en)),
      onExpansionChanged: (open) {
        if (open) {
          expandedSections.add(id);
        } else {
          expandedSections.remove(id);
        }
      },
      children: children,
    );
  }

  Future<void> adoptGuestOwnership() async {
    final m = ModelScope.of(context);
    if (ownerID != 0 || !guestAdoptionApproved || m.token.isEmpty) return;
    final previous = draftKey;
    ownerID = m.account['id'];
    draftKey = 'draft.${m.scope}.${current?['id'] ?? widget.kind}';
    await persist();
    if (previous != draftKey && previous != null) {
      await m.prefs.remove(previous);
    }
  }

  Future<bool> approveGuestAdoption() async {
    if (ownerID != 0 ||
        guestAdoptionApproved ||
        draftKey?.contains('.guest.') != true) {
      return true;
    }
    final m = ModelScope.of(context);
    final accepted = await showDialog<bool>(
      context: context,
      builder: (c) => AlertDialog(
        title: Text(syncText(c, '使用本机访客草稿？', 'Use the guest draft?')),
        content: Text(
          syncText(
            c,
            '当前登录的是已有账号。是否将这份访客资料保存到该账号？已有云端企业不会被覆盖。',
            'You signed in to an existing account. Use this guest draft for that account? An existing cloud company will not be overwritten.',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(c, false),
            child: Text(m.t('cancel')),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(c, true),
            child: Text(syncText(c, '使用这份草稿', 'Use this draft')),
          ),
        ],
      ),
    );
    if (!mounted) return false;
    if (accepted == true) guestAdoptionApproved = true;
    return accepted == true;
  }

  Future<void> useGuestDraft() async {
    final m = ModelScope.of(context),
        key = 'draft.${Uri.encodeComponent(m.baseUrl)}.guest.vendor';
    final raw = m.prefs.getString(key);
    if (raw == null) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (c) => AlertDialog(
        title: Text(syncText(c, '使用本机访客草稿？', 'Use the guest draft?')),
        content: Text(
          syncText(
            c,
            '仅替换当前未保存表单；不会覆盖云端企业。',
            'Only replace this unsaved form; cloud information is unchanged.',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(c, false),
            child: Text(m.t('cancel')),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(c, true),
            child: Text(m.t('confirm')),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    try {
      final local = asMap(jsonDecode(raw));
      setState(() {
        data = asMap(local['data']);
        photos = rows(local['photos']);
        guestDraftKey = key;
        for (final entry in fields.entries) {
          entry.value.text = (data[entry.key] ?? '').toString();
        }
      });
      scheduleSave();
    } catch (_) {
      if (mounted) showError(context, ApiFailure('invalid_request'));
    }
  }

  Widget gallery() {
    final m = ModelScope.of(context),
        enabled = !busy && current?['status'] != 'pending',
        entries = galleryEntries();
    return Column(
      key: sectionKeys['gallery'],
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          syncText(context, '产品／企业图片', 'Product / company images'),
          style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
        ),
        Text(
          syncText(
            context,
            '最多12张，首张为封面；保存时上传，失败保留并可重试。',
            'Up to 12 images. First is the cover. Save uploads pending images; failures remain available to retry.',
          ),
        ),
        space(),
        for (var i = 0; i < entries.length; i++)
          Card(
            child: Padding(
              padding: const EdgeInsets.all(10),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (entries[i]['id'] != null)
                    Photo(
                      id: entries[i]['id'],
                      height: 150,
                      fit: BoxFit.contain,
                    )
                  else
                    Image.file(
                      File(entries[i]['path']),
                      height: 150,
                      fit: BoxFit.contain,
                      errorBuilder: (_, _, _) => Text(m.t('photoRetry')),
                    ),
                  Text(i == 0 ? syncText(context, '封面', 'Cover') : '${i + 1}'),
                  if (entries[i]['id'] == null)
                    Text(
                      entries[i]['error'] != null
                          ? syncText(
                              context,
                              '上传失败，文件已保留',
                              'Upload failed; file retained',
                            )
                          : syncText(context, '待上传', 'Pending upload'),
                    ),
                  Wrap(
                    spacing: 4,
                    children: [
                      IconButton(
                        tooltip: syncText(context, '前移', 'Move earlier'),
                        onPressed: enabled && i > 0
                            ? () => moveImage(entries[i], -1)
                            : null,
                        icon: const Icon(Icons.arrow_upward),
                      ),
                      IconButton(
                        tooltip: syncText(context, '后移', 'Move later'),
                        onPressed: enabled && i < entries.length - 1
                            ? () => moveImage(entries[i], 1)
                            : null,
                        icon: const Icon(Icons.arrow_downward),
                      ),
                      TextButton(
                        onPressed: enabled && i > 0
                            ? () {
                                setState(() {
                                  galleryOrder.remove(entries[i]['key']);
                                  galleryOrder.insert(0, entries[i]['key']);
                                  galleryChanged();
                                });
                              }
                            : null,
                        child: Text(syncText(context, '设为封面', 'Set cover')),
                      ),
                      TextButton(
                        onPressed: enabled
                            ? () => replaceImage(entries[i])
                            : null,
                        child: Text(syncText(context, '替换', 'Replace')),
                      ),
                      IconButton(
                        tooltip: m.t('remove'),
                        onPressed: enabled
                            ? () => removeImage(entries[i])
                            : null,
                        icon: const Icon(Icons.close),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        if (busy)
          LinearProgressIndicator(
            value: brandProgress > 0 ? brandProgress / 100 : null,
          ),
        Wrap(
          spacing: 8,
          children: [
            OutlinedButton.icon(
              onPressed: enabled ? () => pick(ImageSource.camera) : null,
              icon: const Icon(Icons.camera_alt_outlined),
              label: Text(m.t('camera')),
            ),
            OutlinedButton.icon(
              onPressed: enabled ? () => pick(ImageSource.gallery) : null,
              icon: const Icon(Icons.photo_library_outlined),
              label: Text(m.t('album')),
            ),
            if (photos.any((p) => p['id'] == null && p['slot'] == null))
              TextButton(
                onPressed: enabled ? retryGallery : null,
                child: Text(m.t('retry')),
              ),
          ],
        ),
      ],
    );
  }

  Widget companyExtras() {
    final pending = current?['status'] == 'pending';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        group('more', '经营分类与详细地址', 'Category and detailed address', [
          field(context, 'address', 'address'),
        ]),
        group('english', '英文资料（选填）', 'English details (optional)', [
          field(context, 'name_en', 'nameEN'),
          field(
            context,
            'description_en',
            'englishDescription',
            multiline: true,
          ),
        ]),
        group(
          'profile',
          '品牌、企业补充资料与区域联系人',
          'Brand, company details and regional contacts',
          [
            PromotionalProfileEditor(
              initialSection: widget.initialSection,
              sectionKeys: sectionKeys,
              data: data,
              issues: issues,
              fieldKeys: fieldKeys,
              brandProgress: brandProgress,
              brandFiles: photos
                  .where((p) => p['slot'] != null && p['id'] == null)
                  .toList(),
              onBrandPick: pickBrand,
              onBrandRetry: retryBrand,
              onBrandDiscard: (slot) {
                setState(() => photos.removeWhere((p) => p['slot'] == slot));
                scheduleSave();
              },
              enabled: !busy && !pending,
              onChanged: (v) {
                setState(() {
                  final old = asMap(data['company_profile'] ?? {}),
                      next = asMap(v['company_profile'] ?? {});
                  for (final slot in ['logo_media_id', 'cover_media_id']) {
                    if (old[slot] != next[slot]) {
                      photos.removeWhere(
                        (photo) => photo['slot'] == slot && photo['id'] == null,
                      );
                    }
                  }
                  data = v;
                });
                scheduleSave();
              },
            ),
          ],
        ),
        group(
          'credentials',
          '资质、实拍说明及媒体',
          'Credentials, photo annotations and media',
          [
            PortalMediaEditor(
              key: sectionKeys['credentials'],
              initiallyExpanded: true,
              data: data,
              issues: issues,
              enabled: !busy && !pending,
              photos: photos,
              onPick: pickBrand,
              onRetry: retryBrand,
              onDiscard: (slot) {
                setState(() => photos.removeWhere((p) => p['slot'] == slot));
                scheduleSave();
              },
              onChanged: (v) {
                setState(() => data = v);
                scheduleSave();
              },
            ),
          ],
        ),
      ],
    );
  }

  Widget productBasics() {
    final m = ModelScope.of(context), pending = current?['status'] == 'pending';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        field(context, 'name', 'productName'),
        NativeProductQuality(data: data),
        Text(
          syncText(
            context,
            '草稿可先填名称；新产品提交审核时须选择产品分类。',
            'Drafts may start with a name. New products require a category when submitted.',
          ),
        ),
        space(),
        NativeTaxonomyFields(
          kind: 'product',
          hallID: editorHallID,
          data: data,
          enabled: !busy && !pending,
          issues: issues,
          onChanged: (patch) {
            setState(() {
              data = {...data, ...patch};
              for (final key in patch.keys) {
                issues.remove(key);
              }
            });
            scheduleSave();
          },
        ),
        group(
          'more',
          '关键词、说明和参数（选填）',
          'Keywords, description and parameters (optional)',
          [
            field(context, 'keywords', 'keywords'),
            Wrap(
              spacing: 8,
              children: [
                for (final value in nativeKeywordSuggestions(
                  data,
                  m.productCategory(data),
                ))
                  ActionChip(
                    label: Text(value),
                    onPressed: busy || pending
                        ? null
                        : () {
                            fields['keywords']!.text = [
                              fields['keywords']!.text.trim(),
                              value,
                            ].where((v) => v.isNotEmpty).join('，');
                          },
                  ),
              ],
            ),
            field(context, 'description', 'description', multiline: true),
            OutlinedButton(
              onPressed: busy || pending
                  ? null
                  : () async {
                      final name = fields['name']!.text.trim();
                      if (name.isEmpty) return;
                      final suggestion = syncText(
                        context,
                        '产品：$name。',
                        'Product: $name.',
                      );
                      final apply = await showDialog<bool>(
                        context: context,
                        builder: (c) => AlertDialog(
                          title: Text(
                            syncText(c, '产品说明建议', 'Description suggestion'),
                          ),
                          content: Text(suggestion),
                          actions: [
                            TextButton(
                              onPressed: () => Navigator.pop(c, false),
                              child: Text(m.t('cancel')),
                            ),
                            FilledButton(
                              onPressed: () => Navigator.pop(c, true),
                              child: Text(m.t('save')),
                            ),
                          ],
                        ),
                      );
                      if (apply == true && mounted) {
                        fields['description']!.text = [
                          fields['description']!.text,
                          suggestion,
                        ].where((v) => v.trim().isNotEmpty).join('\n');
                      }
                    },
              child: Text(
                syncText(
                  context,
                  '查看中性说明建议',
                  'Preview a neutral description suggestion',
                ),
              ),
            ),
            field(context, 'parameters', 'parameters', multiline: true),
            TextButton(
              onPressed: busy || pending
                  ? null
                  : () {
                      fields['parameters']!.text = [
                        fields['parameters']!.text,
                        nativeParameterTemplate(
                          m.availableHalls
                                  .where((hall) => hall.id == editorHallID)
                                  .firstOrNull
                                  ?.slug ??
                              '',
                          m.language == 'zh',
                        ),
                      ].where((v) => v.trim().isNotEmpty).join('\n');
                    },
              child: Text(
                syncText(
                  context,
                  '追加空白参数模板',
                  'Append an empty parameter template',
                ),
              ),
            ),
          ],
        ),
        group('english', '英文资料（选填）', 'English details (optional)', [
          field(context, 'name_en', 'nameEN'),
          field(
            context,
            'description_en',
            'englishDescription',
            multiline: true,
          ),
        ]),
      ],
    );
  }

  Widget companyBasics() {
    final m = ModelScope.of(context);
    return Column(
      key: sectionKeys['basic'],
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        field(context, 'name', 'company'),
        NativeRegionField(
          key: ValueKey('region:$sourceEndpoint'),
          data: data,
          enabled: !busy && current?['status'] != 'pending',
          onChanged: (v) {
            setState(() {
              fields['region_name']!.text = v['region_name'];
              data = {...data, ...v};
              issues.remove('region_name');
            });
            scheduleSave();
          },
        ),
        if (issues['region_name'] != null)
          Text(
            fieldIssueText(m, issues['region_name']!),
            style: const TextStyle(color: Colors.red),
          ),
        space(),
        field(context, 'main_products', 'mainProducts'),
        NativeTaxonomyFields(
          kind: 'vendor',
          hallID: editorHallID,
          data: data,
          enabled: !busy && current?['status'] != 'pending',
          issues: issues,
          onChanged: (patch) {
            setState(() {
              data = {...data, ...patch};
              for (final key in patch.keys) {
                issues.remove(key);
              }
            });
            scheduleSave();
          },
        ),
        field(context, 'phone', 'phone'),
        field(context, 'wechat', 'wechat'),
        Text(m.t('contactRequired')),
        space(),
        field(context, 'description', 'about', multiline: true),
        group('reference', '介绍参考与建议', 'Introduction references and suggestions', [
          Text(
            syncText(
              context,
              '参考结构：企业名称、所在地区、主营产品、联系方式。仅填写能够核实的情况，不必补造成立年限、资质或产能。',
              'Suggested structure: company name, region, main products and contact information. Include only verifiable facts.',
            ),
          ),
          TextButton(
            onPressed: busy
                ? null
                : () async {
                    final name = fields['name']!.text.trim(),
                        products = fields['main_products']!.text.trim(),
                        region = fields['region_name']!.text.trim();
                    final text = m.language == 'zh'
                        ? [
                            if (name.isNotEmpty) name,
                            if (region.isNotEmpty) '所在地区：$region',
                            if (products.isNotEmpty) '主营产品：$products',
                          ].join('。')
                        : [
                            if (name.isNotEmpty) name,
                            if (region.isNotEmpty) 'Region: $region',
                            if (products.isNotEmpty) 'Main products: $products',
                          ].join('. ');
                    if (text.isEmpty) return;
                    final accepted = await showDialog<bool>(
                      context: context,
                      builder: (c) => AlertDialog(
                        title: Text(
                          syncText(
                            c,
                            '基于已填资料的草稿',
                            'Draft based on your entries',
                          ),
                        ),
                        content: Text(text),
                        actions: [
                          TextButton(
                            onPressed: () => Navigator.pop(c, false),
                            child: Text(m.t('cancel')),
                          ),
                          FilledButton(
                            onPressed: () => Navigator.pop(c, true),
                            child: Text(
                              syncText(c, '追加到介绍', 'Append to introduction'),
                            ),
                          ),
                        ],
                      ),
                    );
                    if (accepted == true && mounted) {
                      fields['description']!.text = [
                        fields['description']!.text,
                        text,
                      ].where((v) => v.trim().isNotEmpty).join('\n');
                    }
                  },
            child: Text(
              syncText(
                context,
                '生成事实型介绍草稿',
                'Preview a factual introduction draft',
              ),
            ),
          ),
        ]),
      ],
    );
  }

  Widget previewBody() {
    final m = ModelScope.of(context);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          fields['name']!.text,
          style: const TextStyle(fontSize: 23, fontWeight: FontWeight.bold),
        ),
        if (m.merchantVendorName.isNotEmpty)
          Text('${m.t('vendor')}: ${m.merchantVendorName}'),
        for (final item in galleryEntries().take(1))
          if (item['id'] != null)
            Photo(id: item['id'], height: 180, fit: BoxFit.contain)
          else
            Image.file(
              File(item['path']),
              height: 180,
              fit: BoxFit.contain,
              errorBuilder: (_, _, _) => const SizedBox(),
            ),
        Text(fields['description']!.text),
        Text(fields['parameters']!.text),
        Text(m.t('submitNotice')),
        if (current != null)
          group('resources', '视频与画册管理', 'Videos and catalogues', [
            NativeResourceList(
              kind: 'video',
              productID: current!['id'],
              management: true,
            ),
            NativeResourceList(
              kind: 'catalog',
              productID: current!['id'],
              management: true,
            ),
          ]),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context),
        product = widget.kind == 'product',
        pending = current?['status'] == 'pending';
    return PopScope(
      canPop: !dirty || leaving,
      onPopInvokedWithResult: (didPop, _) async {
        if (didPop) return;
        final leave = await showDialog<bool>(
          context: context,
          builder: (c) => AlertDialog(
            title: Text(syncText(c, '离开编辑？', 'Leave the editor?')),
            content: Text(
              syncText(
                c,
                '未同步的文字和图片会保留为本机草稿。',
                'Unsynced text and images are retained in the local draft.',
              ),
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(c, false),
                child: Text(m.t('cancel')),
              ),
              FilledButton(
                onPressed: () => Navigator.pop(c, true),
                child: Text(syncText(c, '保留草稿并离开', 'Keep draft and leave')),
              ),
            ],
          ),
        );
        if (leave == true && mounted) {
          await persist();
          if (!mounted) return;
          setState(() => leaving = true);
          WidgetsBinding.instance.addPostFrameCallback((_) {
            if (mounted) Navigator.pop(context);
          });
        }
      },
      child: Theme(
        data: merchantTheme(context),
        child: Scaffold(
          appBar: appBar(
            context,
            product
                ? syncText(
                    context,
                    current == null ? '发布产品' : '编辑产品',
                    current == null ? 'Publish product' : 'Edit product',
                  )
                : m.t('myBooth'),
          ),
          body: SafeArea(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(20),
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 840),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      if (!product) const NativeJoinGuide(),
                      Wrap(
                        spacing: 8,
                        children: [
                          for (var i = 0; i < 3; i++)
                            ChoiceChip(
                              label: Text(
                                '${i + 1}. ${m.language == 'zh' ? (product ? ['基础信息', '图片与展示资料', '预览并提交'] : ['基本资料', '展示资料', '协议与提交'])[i] : (product ? ['Basics', 'Images', 'Preview & submit'] : ['Basics', 'Presentation', 'Agreements & submit'])[i]}',
                              ),
                              selected: productStep == i,
                              onSelected: busy
                                  ? null
                                  : (_) {
                                      setState(() => productStep = i);
                                      scheduleSave();
                                    },
                            ),
                        ],
                      ),
                      space(),
                      if (current != null)
                        Text(
                          '${m.t(current!['status'])}${(current!['review_note'] ?? '').toString().isEmpty ? '' : ' · ${current!['review_note']}'}',
                        ),
                      if (issues.isNotEmpty)
                        UpgradeNotice(
                          syncText(
                            context,
                            '请修改以下项目，填写内容已保留',
                            'Correct these fields; input retained.',
                          ),
                        ),
                      if (issues.isNotEmpty)
                        Wrap(
                          children: [
                            for (final issue in issues.entries)
                              TextButton(
                                onPressed: () {
                                  setState(
                                    () => issues = {
                                      issue.key: issue.value,
                                      ...issues,
                                    },
                                  );
                                  revealIssues();
                                },
                                child: Text(
                                  '${formRules[issue.key]?[m.language] ?? issue.key}: ${fieldIssueText(m, issue.value)}',
                                ),
                              ),
                          ],
                        ),
                      if (!product &&
                          current == null &&
                          !m.newlyRegistered &&
                          m.prefs.containsKey(
                            'draft.${Uri.encodeComponent(m.baseUrl)}.guest.vendor',
                          ) &&
                          guestDraftKey == null)
                        TextButton(
                          onPressed: busy ? null : useGuestDraft,
                          child: Text(
                            syncText(
                              context,
                              '使用本机访客草稿',
                              'Use a guest draft from this device',
                            ),
                          ),
                        ),
                      if (creationUncertain)
                        UpgradeNotice(
                          syncText(
                            context,
                            '上次首次保存结果不明确。请先到工作台核对，未自动重复创建。',
                            'The initial save could not be confirmed. Check the workspace before creating again; no automatic retry was sent.',
                          ),
                        ),
                      if (!product)
                        (productStep == 0
                            ? companyBasics()
                            : productStep == 1
                            ? Column(children: [companyExtras(), gallery()])
                            : Column(
                                crossAxisAlignment: CrossAxisAlignment.stretch,
                                children: [
                                  previewBody(),
                                  const NativePublicationAgreements(),
                                  Text(
                                    syncText(
                                      context,
                                      '确认协议并提交资料，审核通过后公开；保存草稿不会公开。',
                                      'Accept the agreements and submit for review. Drafts stay private.',
                                    ),
                                  ),
                                ],
                              ))
                      else if (productStep == 0)
                        productBasics()
                      else if (productStep == 1)
                        gallery()
                      else
                        previewBody(),
                      space(18),
                      Text(
                        m.t('localSavedHint'),
                        style: const TextStyle(fontSize: 12),
                      ),
                      Text(
                        m.t('submitNotice'),
                        style: const TextStyle(fontSize: 12),
                      ),
                      space(),
                      Wrap(
                        spacing: 12,
                        children: [
                          if (productStep > 0)
                            OutlinedButton(
                              onPressed: busy
                                  ? null
                                  : () {
                                      setState(() => productStep--);
                                      scheduleSave();
                                    },
                              child: Text(m.t('previous')),
                            ),
                          if (productStep < 2)
                            FilledButton(
                              onPressed: busy
                                  ? null
                                  : () {
                                      setState(() => productStep++);
                                      scheduleSave();
                                    },
                              child: Text(m.t('next')),
                            ),
                        ],
                      ),
                      if (!product)
                        OutlinedButton(
                          onPressed: () {
                            unawaited(persist());
                            push(
                              context,
                              PreviewScreen(
                                data: Map<String, dynamic>.from(data),
                                kind: widget.kind,
                                photos: List<Json>.from(photos),
                              ),
                            );
                          },
                          child: Text(m.t('preview')),
                        ),
                      space(),
                      OutlinedButton(
                        onPressed: busy || pending || creationUncertain
                            ? null
                            : () => save(),
                        child: Text(m.t(busy ? 'loading' : 'saveCloud')),
                      ),
                      space(),
                      if (productStep == 2)
                        FilledButton(
                          onPressed: busy || pending || creationUncertain
                              ? null
                              : () => save(submit: true),
                          child: Text(m.t(busy ? 'loading' : 'submit')),
                        ),
                      if (creationUncertain)
                        TextButton(
                          onPressed: () => returnToMerchant(context),
                          child: Text(
                            syncText(context, '到工作台核对', 'Check the workspace'),
                          ),
                        ),
                      space(24),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class PreviewScreen extends StatelessWidget {
  final Json data;
  final String kind;
  final List<Json> photos;
  const PreviewScreen({
    super.key,
    required this.data,
    required this.kind,
    required this.photos,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Scaffold(
      appBar: appBar(context, m.t('preview')),
      body: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          message(context, m.t('previewNotice')),
          if (kind == 'vendor') NativeBrandPreview(data: data, files: photos),
          if (kind == 'vendor')
            CompanyPortalBody(data: data, localPhotos: photos),
          if (kind != 'vendor')
            dataDetails(
              context,
              data,
              kind,
              localPhotos: photos.where((p) => p['slot'] == null).toList(),
            ),
          space(24),
          Contact(data: data),
        ],
      ),
    );
  }
}
