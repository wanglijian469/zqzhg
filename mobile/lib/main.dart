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
import 'package:flutter/services.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:http/http.dart' as http;
import 'package:image_picker/image_picker.dart';
import 'package:path_provider/path_provider.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:url_launcher/url_launcher.dart';

part 'analytics.dart';
part 'form_rules.dart';
part 'onboarding.dart';
part 'industrial_ui.dart';
part 'merchant_experience.dart';
part 'merchant_workspace.dart';
part 'merchant_products.dart';
part 'resources_experience.dart';
part 'app_sync.dart';
part 'app_navigation.dart';
part 'favorites.dart';
part 'exhibition_home.dart';
part 'browse_screen.dart';
part 'exhibit_card.dart';
part 'exhibition_halls.dart';

typedef Json = Map<String, dynamic>;
const green = industrialBlue;
const pale = industrialSurface;
Json asMap(dynamic value) => Map<String, dynamic>.from(value as Map);
List<Json> rows(dynamic value) => (value as List? ?? []).map(asMap).toList();
List<int> imageIDs(Json data) => (data['media_ids'] as List? ?? [])
    .map((id) => (id as num).toInt())
    .toList();

class ApiFailure implements Exception {
  final String code;
  final List<Json> fields;
  ApiFailure(this.code, [this.fields = const []]);
}

class AppModel extends ChangeNotifier {
  late final MerchantDataStore merchant = MerchantDataStore(this);
  late final FavoritesStore favorites = FavoritesStore(this);
  late final AppAnalytics analytics = AppAnalytics(this);
  VoidCallback? openMerchant;
  VoidCallback? openHome;
  void favoritesChanged() => notifyListeners();
  final SharedPreferences prefs;
  final http.Client client;
  final bool analyticsEnabled;
  final Map<String, Json> resources;
  late String language;
  late String baseUrl;
  late String token;
  late Json account;
  ExhibitionHall activeHall = ExhibitionHall.agriculture;
  bool get isHallPreview => activeHall == ExhibitionHall.cablesPreview;
  void selectHall(ExhibitionHall hall) {
    if (hall == activeHall) return;
    activeHall = hall;
    notifyListeners();
  }

  Future<dynamic>? _taxonomy;
  String? _taxonomyBase;
  Future<dynamic> publicTaxonomy({bool refresh = false}) {
    if (refresh || _taxonomyBase != baseUrl) _taxonomy = null;
    _taxonomyBase = baseUrl;
    if (_taxonomy != null) return _taxonomy!;
    final request = api('/public/taxonomy');
    _taxonomy = request;
    unawaited(
      request.then<void>(
        (_) {},
        onError: (Object e, StackTrace s) {
          if (identical(_taxonomy, request)) _taxonomy = null;
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
  }) : analyticsEnabled = analyticsEnabled ?? Platform.isAndroid,
       client = client ?? http.Client() {
    language = prefs.getString('language') ?? 'zh';
    const compiledBase = String.fromEnvironment(
      'API_BASE_URL',
      defaultValue: 'http://39.97.250.244:8089',
    );
    final savedBase = prefs.getString('base_url');
    const migrationKey = 'endpoint_migration.aliyun_39_97_250_244_8089';
    final firstCloudLaunch =
        compiledBase == 'http://39.97.250.244:8089' &&
        prefs.getBool(migrationKey) != true;
    final migrate =
        firstCloudLaunch &&
        (savedBase == null ||
            [
              'http://172.27.35.2:8080',
              'http://10.0.2.2:8080',
            ].contains(savedBase.replaceAll(RegExp(r'/$'), '')));
    baseUrl = migrate ? compiledBase : savedBase ?? compiledBase;
    token = migrate ? '' : prefs.getString('token') ?? '';
    account = migrate
        ? <String, dynamic>{}
        : asMap(jsonDecode(prefs.getString('account') ?? '{}'));
    if (firstCloudLaunch) {
      unawaited(prefs.setBool(migrationKey, true));
      if (migrate) {
        unawaited(prefs.setString('base_url', compiledBase));
        unawaited(prefs.remove('token'));
        unawaited(prefs.remove('account'));
      }
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
      final dynamic parsed = jsonDecode(response.body);
      if (response.statusCode >= 400) {
        final credentials = [
          '/auth/merchant',
          '/auth/visitor',
          '/auth/merchant/register',
          '/auth/visitor/register',
          '/auth/phone',
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
        );
      }
      return parsed;
    } on ApiFailure {
      rethrow;
    } catch (_) {
      throw ApiFailure('network_error');
    }
  }

  Future<int> upload(String path) async {
    try {
      final req = http.MultipartRequest(
        'POST',
        Uri.parse('$baseUrl/api/v1/merchant/media'),
      );
      req.headers['Authorization'] = 'Bearer $token';
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
    merchant.clear();
    await prefs.remove('token');
    await prefs.remove('account');
    notifyListeners();
  }

  Future<void> changeEndpoint(String url) async {
    final uri = Uri.tryParse(url);
    if (uri == null ||
        !['http', 'https'].contains(uri.scheme) ||
        uri.host.isEmpty ||
        uri.query.isNotEmpty ||
        uri.userInfo.isNotEmpty ||
        (uri.path.isNotEmpty && uri.path != '/')) {
      throw ApiFailure('invalid_request');
    }
    await signOut();
    baseUrl = url.replaceAll(RegExp(r'/$'), '');
    activeHall = ExhibitionHall.agriculture;
    await prefs.setString('base_url', baseUrl);
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
        title: '大陆农机',
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
            onPressed: () async {
              try {
                analyticsContact(context, 'contact_phone');
                if (!await launchUrl(Uri(scheme: 'tel', path: phone))) {
                  throw ApiFailure('network_error');
                }
              } catch (e) {
                if (context.mounted) showError(context, e);
              }
            },
            label: Text(phone),
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
                    child: FavoriteButton(item: {...item, 'kind': widget.kind}),
                  ),
                  dataDetails(context, data, widget.kind),
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
                        : asMap(result['vendor']['data']),
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

class LoginScreen extends StatefulWidget {
  final bool visitor;
  final bool register;
  final String initialName;
  const LoginScreen({
    super.key,
    this.register = false,
    this.initialName = '',
    this.visitor = false,
  });
  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final phone = TextEditingController(),
      code = TextEditingController(),
      username = TextEditingController(),
      password = TextEditingController();
  bool accountMode = true, registerMode = false;
  bool manufacturerLogin = false;
  bool get visitorAccount => widget.visitor && !manufacturerLogin;
  final confirmation = TextEditingController();
  Map<String, String> authErrors = {};
  @override
  void initState() {
    super.initState();
    registerMode = widget.register;
    username.text = widget.initialName;
  }

  bool busy = false;
  String dev = '';
  int cooldown = 0;
  Timer? timer;
  @override
  void dispose() {
    confirmation.dispose();
    username.dispose();
    password.dispose();
    phone.dispose();
    code.dispose();
    timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Scaffold(
      appBar: appBar(
        context,
        m.t(widget.visitor ? 'favoriteAccountLogin' : 'verify'),
      ),
      body: ListView(
        padding: const EdgeInsets.all(24),
        children: [
          if (widget.visitor)
            SegmentedButton<bool>(
              segments: [
                ButtonSegment(value: false, label: Text(m.t('visitorAccount'))),
                ButtonSegment(
                  value: true,
                  label: Text(m.t('manufacturerAccount')),
                ),
              ],
              selected: {manufacturerLogin},
              onSelectionChanged: busy
                  ? null
                  : (value) => setState(() {
                      manufacturerLogin = value.first;
                      registerMode = false;
                      password.clear();
                      confirmation.clear();
                      authErrors = {};
                    }),
            )
          else
            SegmentedButton<bool>(
              segments: [
                ButtonSegment(value: true, label: Text(m.t('accountLogin'))),
                ButtonSegment(value: false, label: Text(m.t('smsLogin'))),
              ],
              selected: {accountMode},
              onSelectionChanged: busy
                  ? null
                  : (v) => setState(() => accountMode = v.first),
            ),
          space(),
          if (accountMode) ...[
            Wrap(
              children: [
                TextButton(
                  onPressed: busy
                      ? null
                      : () => setState(() {
                          registerMode = false;
                          authErrors = {};
                          password.clear();
                          confirmation.clear();
                        }),
                  child: Text(m.t('accountLogin')),
                ),
                TextButton(
                  onPressed: busy
                      ? null
                      : () => setState(() {
                          registerMode = true;
                          authErrors = {};
                          password.clear();
                          confirmation.clear();
                        }),
                  child: Text(
                    visitorAccount
                        ? m.t('newVisitorRegister')
                        : syncText(context, '新厂商注册', 'Register manufacturer'),
                  ),
                ),
              ],
            ),
            TextField(
              controller: username,
              autofillHints: const [AutofillHints.username],
              decoration: InputDecoration(
                labelText: m.t('username'),
                errorText: authErrors['login_name'],
              ),
            ),
            space(),
            TextField(
              controller: password,
              obscureText: true,
              autofillHints: const [AutofillHints.password],
              decoration: InputDecoration(
                labelText: m.t('password'),
                errorText: authErrors['password'],
                helperText: registerMode
                    ? syncText(context, '密码12–72个UTF-8字节', '12–72 UTF-8 bytes')
                    : null,
              ),
            ),
            space(),
            if (registerMode)
              TextField(
                controller: confirmation,
                obscureText: true,
                decoration: InputDecoration(
                  labelText: syncText(context, '确认密码', 'Confirm password'),
                  errorText: authErrors['confirmation'],
                ),
              ),
            FilledButton(
              onPressed: busy
                  ? null
                  : () async {
                      if (registerMode) {
                        final n = utf8.encode(password.text).length;
                        setState(
                          () => authErrors = {
                            if (username.text.trim().isEmpty ||
                                username.text.trim().runes.length > 100)
                              'login_name': syncText(
                                context,
                                '账号不能为空，最多100字',
                                'Required, up to 100 characters',
                              ),
                            if (n < 12 ||
                                n > 72 ||
                                password.text.trim().isEmpty)
                              'password': syncText(
                                context,
                                '密码需12–72个UTF-8字节',
                                'Use 12–72 UTF-8 bytes',
                              ),
                            if (confirmation.text != password.text)
                              'confirmation': syncText(
                                context,
                                '两次密码不一致',
                                'Passwords do not match',
                              ),
                          },
                        );
                        if (authErrors.isNotEmpty) return;
                      }
                      setState(() => busy = true);
                      try {
                        final r = asMap(
                          await m.api(
                            visitorAccount
                                ? (registerMode
                                      ? '/auth/visitor/register'
                                      : '/auth/visitor')
                                : (registerMode
                                      ? '/auth/merchant/register'
                                      : '/auth/merchant'),
                            method: 'POST',
                            data: {
                              (registerMode ? 'login_name' : 'username'):
                                  username.text.trim(),
                              'password': password.text,
                            },
                          ),
                        );
                        await m.signIn(r);
                        password.clear();
                        if (context.mounted) Navigator.pop(context, true);
                      } catch (e) {
                        if (context.mounted) {
                          if (registerMode &&
                              e is ApiFailure &&
                              e.code == 'network_error') {
                            setState(() {
                              registerMode = false;
                              authErrors = {
                                'login_name': syncText(
                                  context,
                                  '注册结果不确定，请用刚设置的账号密码登录后继续。',
                                  'Registration unconfirmed. Sign in with these credentials to continue.',
                                ),
                              };
                            });
                          } else if (e is ApiFailure && e.fields.isNotEmpty) {
                            setState(
                              () => authErrors = {
                                for (final f in e.fields)
                                  f['field']: fieldIssueText(m, f),
                              },
                            );
                          } else {
                            showError(context, e);
                          }
                        }
                      } finally {
                        if (mounted) setState(() => busy = false);
                      }
                    },
              child: Text(
                busy
                    ? m.t('loading')
                    : registerMode
                    ? syncText(context, '注册并继续', 'Register and continue')
                    : m.t('login'),
              ),
            ),
          ] else ...[
            TextField(
              controller: phone,
              keyboardType: TextInputType.phone,
              autofillHints: const [AutofillHints.telephoneNumberNational],
              decoration: InputDecoration(labelText: m.t('phone')),
            ),
            space(),
            TextField(
              controller: code,
              keyboardType: TextInputType.number,
              autofillHints: const [AutofillHints.oneTimeCode],
              decoration: InputDecoration(labelText: m.t('code')),
            ),
            space(),
            OutlinedButton(
              onPressed: busy || cooldown > 0
                  ? null
                  : () async {
                      setState(() => busy = true);
                      try {
                        final result = await m.api(
                          '/auth/sms',
                          method: 'POST',
                          data: {'phone': phone.text.trim()},
                        );
                        if (mounted) {
                          setState(() {
                            dev = result['development_code'] ?? '';
                            cooldown = 60;
                          });
                          timer?.cancel();
                          timer = Timer.periodic(const Duration(seconds: 1), (
                            t,
                          ) {
                            if (!mounted) {
                              t.cancel();
                              return;
                            }
                            setState(() => cooldown--);
                            if (cooldown <= 0) t.cancel();
                          });
                        }
                      } catch (e) {
                        if (context.mounted) showError(context, e);
                      } finally {
                        if (mounted) setState(() => busy = false);
                      }
                    },
              child: Text(cooldown > 0 ? '$cooldown s' : m.t('sendCode')),
            ),
            if (dev.isNotEmpty) message(context, '${m.t('devCode')}: $dev'),
            space(),
            FilledButton(
              onPressed: busy
                  ? null
                  : () async {
                      setState(() => busy = true);
                      try {
                        final result = await m.api(
                          '/auth/phone',
                          method: 'POST',
                          data: {
                            'phone': phone.text.trim(),
                            'code': code.text.trim(),
                          },
                        );
                        await m.signIn(asMap(result));
                        if (context.mounted) Navigator.pop(context, true);
                      } catch (e) {
                        if (context.mounted) showError(context, e);
                      } finally {
                        if (mounted) setState(() => busy = false);
                      }
                    },
              child: Text(m.t('verify')),
            ),
          ],
        ],
      ),
    );
  }
}

class MerchantScreen extends StatelessWidget {
  const MerchantScreen({super.key});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    if (m.token.isNotEmpty && m.account['role'] != 'visitor') {
      return MerchantWorkspace(key: ValueKey(m.scope));
    }
    return Scaffold(
      appBar: appBar(context, m.t('merchantCenter')),
      body: ListView(
        padding: const EdgeInsets.all(24),
        children: [
          Text(
            m.t('joinTitle'),
            style: const TextStyle(fontSize: 28, fontWeight: FontWeight.bold),
          ),
          message(context, m.t('localDraft')),
          if (m.account['role'] == 'visitor')
            message(context, m.t('visitorMerchantHelp')),
          FilledButton(
            onPressed: () => m.account['role'] == 'visitor'
                ? push(context, const LoginScreen())
                : push(context, const EditorScreen(kind: 'vendor')),
            child: Text(
              m.t(
                m.account['role'] == 'visitor' ? 'manufacturerAccount' : 'join',
              ),
            ),
          ),
          space(),
          OutlinedButton(
            onPressed: () => push(context, const LoginScreen()),
            child: Text(m.t('login')),
          ),
        ],
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
    final raw = m.prefs.getString(draftKey!);
    if (raw != null) {
      try {
        final local = asMap(jsonDecode(raw));
        if (local['revision_id'] == (current?['revision_id'] ?? 0)) {
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
      fields[key] = TextEditingController(text: (data[key] ?? '').toString());
      fields[key]!.addListener(() {
        data[key] = fields[key]!.text;
        if (key == 'region_name') data['region_id'] = 0;
        scheduleSave();
        if (mounted) setState(() => issues.remove(key));
      });
    }
    if (widget.initialSection != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        final target = sectionKeys[widget.initialSection]?.currentContext;
        if (target != null) Scrollable.ensureVisible(target, alignment: .05);
      });
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
    timer?.cancel();
    timer = Timer(
      const Duration(milliseconds: 250),
      () => unawaited(persist()),
    );
  }

  Future<void> persist() async {
    if (!mounted || draftKey == null || retired) return;
    final m = ModelScope.of(context);
    if (m.token.isNotEmpty && ownerID == 0 && draftKey!.contains('.guest.')) {
      ownerID = m.account['id'];
      final previous = draftKey!;
      draftKey = 'draft.${m.scope}.${current?['id'] ?? widget.kind}';
      await m.prefs.setString(
        draftKey!,
        jsonEncode({
          'data': data,
          'photos': photos,
          'revision_id': current?['revision_id'] ?? 0,
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
      }),
    );
  }

  @override
  void dispose() {
    timer?.cancel();
    for (final field in fields.values) {
      field.dispose();
    }
    super.dispose();
  }

  Future<void> retainPhoto(XFile file, {String? slot}) async {
    if (slot == null &&
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
      final response = await http.Response.fromStream(
        await m.client.send(request),
      ).timeout(const Duration(minutes: 3));
      final payload = asMap(jsonDecode(response.body));
      if (response.statusCode >= 400) {
        if (response.statusCode == 401) await m.expireSession();
        throw ApiFailure(payload['error']['code']);
      }
      final mid = payload['id'];
      photo['id'] = mid;
      if (photo['slot'] == null) {
        data['media_ids'] = [...imageIDs(data), mid];
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
    if (m.token.isEmpty) {
      final ok = await Navigator.push<bool>(
        context,
        MaterialPageRoute(
          builder: (_) => LoginScreen(
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
    final m = ModelScope.of(context);
    timer?.cancel();
    await persist();
    if (!mounted) return;
    if (retired) return;
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
      WidgetsBinding.instance.addPostFrameCallback((_) {
        final ctx = fieldKeys[errors.keys.first]?.currentContext;
        if (ctx != null) Scrollable.ensureVisible(ctx, alignment: .3);
      });
      return;
    }
    if (m.token.isEmpty) {
      final ok = await Navigator.push<bool>(
        context,
        MaterialPageRoute(
          builder: (_) => LoginScreen(
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
      final oldKey = draftKey;
      final returningOwner = ownerID != 0;
      ownerID = m.account['id'];
      draftKey = 'draft.${m.scope}.${current?['id'] ?? widget.kind}';
      await persist();
      if (oldKey != draftKey) await m.prefs.remove(oldKey!);
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
    setState(() => busy = true);
    final creatingBooth = widget.kind == 'vendor' && current == null;
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
              ? {'kind': widget.kind, 'data': data}
              : {'revision_id': current!['revision_id'], 'data': data},
        ),
      );
      current = saved;
      data = credentialEditorData(asMap(saved['data']));
      final oldKey = draftKey;
      draftKey = 'draft.${m.scope}.${saved['id']}';
      await persist();
      if (oldKey != draftKey) await m.prefs.remove(oldKey!);
      if (submit) {
        await m.api(
          '/merchant/contents/${saved['id']}/submit',
          method: 'POST',
          data: {'revision_id': saved['revision_id']},
        );
        await m.prefs.remove(draftKey!);
      }
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(m.t(submit ? 'pending' : 'saved'))),
        );
        if (creatingBooth) {
          returnToMerchant(context);
        } else if (submit) {
          Navigator.pop(context, true);
        }
      }
    } catch (e) {
      if (e is ApiFailure && e.code == 'unauthorized') {
        await m.expireSession();
      }
      if (mounted && e is ApiFailure && e.fields.isNotEmpty) {
        setState(() => issues = {for (final f in e.fields) f['field']: f});
      } else if (mounted) {
        showError(context, e);
      }
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Widget field(
    BuildContext context,
    String key,
    String label, {
    bool multiline = false,
  }) {
    final m = ModelScope.of(context);
    return Padding(
      key: fieldKeys.putIfAbsent(key, () => GlobalKey()),
      padding: const EdgeInsets.only(bottom: 18),
      child: TextField(
        controller: fields[key],
        maxLines: multiline ? 4 : 1,
        decoration: guidedDecoration(
          m,
          key,
          (data[key] ?? '').toString(),
          m.t(label),
          issue: issues[key],
        ),
        keyboardType: key == 'phone'
            ? TextInputType.phone
            : multiline
            ? TextInputType.multiline
            : TextInputType.text,
        enabled: !busy && current?['status'] != 'pending',
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final pending = current?['status'] == 'pending';
    return PopScope(
      canPop: true,
      onPopInvokedWithResult: (didPop, _) {
        if (didPop) {
          timer?.cancel();
          if (draftKey != null) {
            unawaited(
              m.prefs.setString(
                draftKey!,
                jsonEncode({
                  'data': data,
                  'photos': photos,
                  'revision_id': current?['revision_id'] ?? 0,
                }),
              ),
            );
          }
        }
      },
      child: Scaffold(
        appBar: appBar(
          context,
          m.t(widget.kind == 'vendor' ? 'myBooth' : 'addProduct'),
        ),
        body: SingleChildScrollView(
          padding: const EdgeInsets.all(20),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const NativeJoinGuide(),
              if (issues.isNotEmpty)
                Container(
                  padding: const EdgeInsets.all(16),
                  margin: const EdgeInsets.only(bottom: 20),
                  color: const Color(0xffffeeee),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        m.language == 'zh'
                            ? '请修改以下项目，填写内容已保留'
                            : 'Correct these fields; input retained.',
                      ),
                      for (final entry in issues.entries)
                        TextButton(
                          onPressed: () {
                            final ctx = fieldKeys[entry.key]?.currentContext;
                            if (ctx != null) {
                              Scrollable.ensureVisible(ctx, alignment: .3);
                            }
                          },
                          child: Text(
                            '${formRules[entry.key.replaceAll(RegExp(r"\.\d+\."), ".*.")]?[m.language] ?? entry.key}：${fieldIssueText(m, entry.value)}',
                          ),
                        ),
                    ],
                  ),
                ),
              if (m.token.isEmpty) message(context, m.t('localDraft')),
              if (current != null)
                message(
                  context,
                  '${m.t(current!['status'])}${(current!['review_note'] ?? '').toString().isEmpty ? '' : '\n${current!['review_note'] == 'offline' ? m.t('offline') : current!['review_note']}'}',
                ),
              KeyedSubtree(
                key: sectionKeys['basic'],
                child: field(
                  context,
                  'name',
                  widget.kind == 'vendor' ? 'company' : 'productName',
                ),
              ),
              Text(
                m.t('basicInfo'),
                style: const TextStyle(
                  fontSize: 19,
                  fontWeight: FontWeight.bold,
                ),
              ),
              if (widget.kind == 'vendor') ...[
                field(context, 'region_name', 'regionName'),
                field(context, 'main_products', 'mainProducts'),
                Text(
                  key: sectionKeys['contact'],
                  m.t('contactInfo'),
                  style: const TextStyle(
                    fontSize: 19,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                field(context, 'phone', 'phone'),
                field(context, 'wechat', 'wechat'),
                message(context, m.t('contactRequired')),
                field(context, 'address', 'address'),
              ],
              NativeCategoryPicker(
                kind: widget.kind,
                key: fieldKeys.putIfAbsent('category_id', () => GlobalKey()),
                value: data['category_id'] ?? 0,
                enabled: !busy && !pending,
                issue: issues['category_id'],
                onChanged: (id) {
                  setState(() => data['category_id'] = id);
                  scheduleSave();
                },
              ),
              if (widget.kind == 'product')
                message(context, m.t('optionalName')),
              field(
                context,
                'description',
                widget.kind == 'vendor' ? 'about' : 'description',
                multiline: true,
              ),
              field(
                context,
                'description_en',
                'englishDescription',
                multiline: true,
              ),
              if (widget.kind == 'product')
                field(context, 'parameters', 'parameters', multiline: true),
              Text(
                key: sectionKeys['gallery'],
                m.t('photos'),
                style: const TextStyle(
                  fontSize: 19,
                  fontWeight: FontWeight.bold,
                ),
              ),
              space(),
              ...imageIDs(data).map(
                (id) => Row(
                  children: [
                    Expanded(
                      child: Padding(
                        padding: const EdgeInsets.only(bottom: 10),
                        child: Photo(id: id, height: 140),
                      ),
                    ),
                    IconButton(
                      onPressed: pending || busy
                          ? null
                          : () {
                              setState(() {
                                data['media_ids'] = imageIDs(
                                  data,
                                ).where((mid) => mid != id).toList();
                                photos.removeWhere((p) => p['id'] == id);
                                final profile = asMap(
                                  data['company_profile'] ?? {},
                                );
                                if (profile['gallery_annotations'] != null) {
                                  data['company_profile'] = {
                                    ...profile,
                                    'gallery_annotations':
                                        rows(profile['gallery_annotations'])
                                            .where((r) => r['media_id'] != id)
                                            .toList(),
                                  };
                                }
                              });
                              scheduleSave();
                            },
                      icon: const Icon(Icons.close),
                    ),
                  ],
                ),
              ),
              ...photos
                  .where((p) => p['id'] == null && p['slot'] == null)
                  .map(
                    (p) => Row(
                      children: [
                        Expanded(
                          child: Padding(
                            padding: const EdgeInsets.only(bottom: 10),
                            child: Image.file(
                              File(p['path']),
                              height: 140,
                              fit: BoxFit.cover,
                              errorBuilder: (_, _, _) =>
                                  Text(m.t('photoRetry')),
                            ),
                          ),
                        ),
                        IconButton(
                          onPressed: busy || pending
                              ? null
                              : () {
                                  setState(() => photos.remove(p));
                                  scheduleSave();
                                },
                          icon: const Icon(Icons.close),
                        ),
                      ],
                    ),
                  ),
              Wrap(
                spacing: 12,
                children: [
                  OutlinedButton.icon(
                    onPressed: busy || pending
                        ? null
                        : () => pick(ImageSource.camera),
                    icon: const Icon(Icons.camera_alt_outlined),
                    label: Text(m.t('camera')),
                  ),
                  OutlinedButton.icon(
                    onPressed: busy || pending
                        ? null
                        : () => pick(ImageSource.gallery),
                    icon: const Icon(Icons.photo_library_outlined),
                    label: Text(m.t('album')),
                  ),
                ],
              ),
              if (widget.kind == 'vendor')
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
                  onBrandPick: (slot, file) => pickBrand(slot, file),
                  onBrandRetry: retryBrand,
                  onBrandDiscard: (slot) {
                    setState(
                      () => photos.removeWhere((p) => p['slot'] == slot),
                    );
                    scheduleSave();
                  },
                  enabled: !busy && !pending,
                  onChanged: (v) {
                    setState(() {
                      final old = asMap(data['company_profile'] ?? {}),
                          next = asMap(v['company_profile'] ?? {});
                      for (final k in ['logo_media_id', 'cover_media_id']) {
                        if (old[k] != next[k]) {
                          photos.removeWhere(
                            (p) => p['slot'] == k && p['id'] == null,
                          );
                        }
                      }
                      data = v;
                    });
                    scheduleSave();
                  },
                ),
              if (widget.kind == 'vendor')
                PortalMediaEditor(
                  initiallyExpanded: widget.initialSection == 'credentials',
                  key: sectionKeys['credentials'],
                  data: data,
                  issues: issues,
                  enabled: !busy && !pending,
                  photos: photos,
                  onPick: pickBrand,
                  onRetry: retryBrand,
                  onDiscard: (slot) {
                    setState(
                      () => photos.removeWhere((p) => p['slot'] == slot),
                    );
                    scheduleSave();
                  },
                  onChanged: (v) {
                    setState(() => data = v);
                    scheduleSave();
                  },
                ),
              if (widget.kind == 'product' && current != null)
                ExpansionTile(
                  title: Text(m.t('manageResources')),
                  children: [
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
                  ],
                ),
              message(context, m.t('localSavedHint')),
              message(context, m.t('submitNotice')),
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
                onPressed: busy || pending ? null : () => save(),
                child: Text(m.t('saveCloud')),
              ),
              space(),
              FilledButton(
                onPressed: busy || pending ? null : () => save(submit: true),
                child: Text(m.t(busy ? 'loading' : 'submit')),
              ),
              space(30),
            ],
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
